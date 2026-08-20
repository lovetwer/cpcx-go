package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type registerReq struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Email     string `json:"email"`
	DeviceID  string `json:"device_id"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type deviceLoginReq struct {
	DeviceID    string `json:"device_id"`
	Email       string `json:"email"`
	DeviceModel string `json:"device_model"`
}

type authResp struct {
	OK    bool   `json:"ok"`
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// handleRegister 注册
func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}
	var cnt int
	if err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", req.Username).Scan(&cnt); err != nil {
		writeError(w, http.StatusInternalServerError, "数据库错误")
		return
	}
	if cnt > 0 {
		writeError(w, http.StatusConflict, "用户名已存在")
		return
	}
	pw, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "密码处理失败")
		return
	}
	res, err := DB.Exec("INSERT INTO users(username,password,device_id,email) VALUES(?,?,?,?)",
		req.Username, pw, req.DeviceID, req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "注册失败")
		return
	}
	id, _ := res.LastInsertId()
	u, _ := getUserByID(id)
	writeJSON(w, http.StatusOK, authResp{OK: true, Token: generateToken(id), User: u})
}

// handleLogin 账号密码登录
func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	u, err := getUserByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if !checkPassword(req.Password, u.Password) {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	writeJSON(w, http.StatusOK, authResp{OK: true, Token: generateToken(u.ID), User: u})
}

// handleDeviceLogin 按设备号一键登录（无则自动建号）
// 用户名格式：机型_时间戳（如 "XYD-W10_1739485210"），确保全局唯一不重复
func handleDeviceLogin(w http.ResponseWriter, r *http.Request) {
	var req deviceLoginReq
	if err := readJSON(r, &req); err != nil || req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "设备号不能为空")
		return
	}
	u, err := getUserByDevice(req.DeviceID)
	if err != nil {
		// 自动创建该设备对应的用户，用户名 = 机型_时间戳，确保不重复
		username := req.DeviceModel + "_" + fmt.Sprintf("%d", time.Now().Unix())
		pw, _ := hashPassword(randHex(12))
		res, e := DB.Exec("INSERT INTO users(username,password,device_id,email) VALUES(?,?,?,?)",
			username, pw, req.DeviceID, req.Email)
		// 若用户名冲突（极端情况），追加随机后缀重试
		for e != nil {
			username = req.DeviceModel + "_" + fmt.Sprintf("%d", time.Now().UnixNano())
			res, e = DB.Exec("INSERT INTO users(username,password,device_id,email) VALUES(?,?,?,?)",
				username, pw, req.DeviceID, req.Email)
		}
		id, _ := res.LastInsertId()
		u, _ = getUserByID(id)
	}
	writeJSON(w, http.StatusOK, authResp{OK: true, Token: generateToken(u.ID), User: u})
}

// handleMe 返回当前用户
func handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := getUserByID(currentUserID(r))
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "user": u})
}

// ---------- 查询辅助 ----------

// handleUpdateMe 更新当前用户资料（邮箱 / 昵称 / 密码）
func handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		OldPassword string `json:"old_password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	uid := currentUserID(r)
	var sets []string
	var args []interface{}
	if body.Email != "" {
		sets = append(sets, "email=?")
		args = append(args, body.Email)
	}
	if body.Username != "" && body.Username != authUser(r).Username {
		var cnt int
		if err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username=? AND id<>?", body.Username, uid).Scan(&cnt); err == nil && cnt > 0 {
			writeError(w, http.StatusConflict, "用户名已存在")
			return
		}
		sets = append(sets, "username=?")
		args = append(args, body.Username)
	}
	if body.Password != "" {
		if len(body.Password) < 6 {
			writeError(w, http.StatusBadRequest, "密码至少6位")
			return
		}
		// 获取当前用户信息（含密码哈希）
		curUser, _ := getUserByID(uid)
		if curUser == nil {
			writeError(w, http.StatusUnauthorized, "用户不存在")
			return
		}
		// 查询包含密码哈希的用户信息
		fullUser, err := getUserByUsername(curUser.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "用户信息查询失败")
			return
		}
		// 设备号一键登录的账户（password 为空）可跳过原密码校验；
		// 普通账户修改密码时必须校验原密码
		if fullUser.Password != "" {
			if body.OldPassword == "" {
				writeError(w, http.StatusBadRequest, "请输入原密码")
				return
			}
			if !checkPassword(body.OldPassword, fullUser.Password) {
				writeError(w, http.StatusBadRequest, "原密码错误")
				return
			}
		}
		pw, err := hashPassword(body.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "密码处理失败")
			return
		}
		sets = append(sets, "password=?")
		args = append(args, pw)
	}
	if len(sets) == 0 {
		writeError(w, http.StatusBadRequest, "没有可更新的字段")
		return
	}
	args = append(args, uid)
	if _, err := DB.Exec("UPDATE users SET "+strings.Join(sets, ",")+" WHERE id=?", args...); err != nil {
		writeError(w, http.StatusInternalServerError, "更新失败")
		return
	}
	u, _ := getUserByID(uid)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "user": u})
}

// handleDeleteMe 注销账号：删除用户及其所有关联数据（彩票、分享）
func handleDeleteMe(w http.ResponseWriter, r *http.Request) {
	uid := currentUserID(r)
	// 删除用户的彩票记录
	if _, err := DB.Exec("DELETE FROM lotteries WHERE user_id=?", uid); err != nil {
		writeError(w, http.StatusInternalServerError, "注销失败：无法删除彩票数据")
		return
	}
	// 删除用户的分享记录
	if _, err := DB.Exec("DELETE FROM shares WHERE user_id=?", uid); err != nil {
		writeError(w, http.StatusInternalServerError, "注销失败：无法删除分享数据")
		return
	}
	// 删除用户本体
	if _, err := DB.Exec("DELETE FROM users WHERE id=?", uid); err != nil {
		writeError(w, http.StatusInternalServerError, "注销失败：无法删除用户")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "msg": "账号已注销"})
}

// authUser 取当前登录用户（用于对比昵称）
func authUser(r *http.Request) *User {
	u, _ := getUserByID(currentUserID(r))
	return u
}

func getUserByID(id int64) (*User, bool) {
	u := &User{}
	err := DB.QueryRow("SELECT id,username,device_id,email,created_at FROM users WHERE id=?",
		id).Scan(&u.ID, &u.Username, &u.DeviceID, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, false
	}
	return u, true
}

func getUserByUsername(name string) (*User, error) {
	u := &User{}
	err := DB.QueryRow("SELECT id,username,password,device_id,email,created_at FROM users WHERE username=?",
		name).Scan(&u.ID, &u.Username, &u.Password, &u.DeviceID, &u.Email, &u.CreatedAt)
	return u, err
}

func getUserByDevice(device string) (*User, error) {
	u := &User{}
	err := DB.QueryRow("SELECT id,username,device_id,email,created_at FROM users WHERE device_id=?",
		device).Scan(&u.ID, &u.Username, &u.DeviceID, &u.Email, &u.CreatedAt)
	return u, err
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
