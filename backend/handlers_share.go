package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// handleCreateShare 生成分享链接（需登录）：传入选中的彩票ID列表，返回分享码
func handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(r, &req); err != nil || len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "请选择要分享的彩票")
		return
	}
	uid := currentUserID(r)
	// 校验这些彩票确实属于当前用户
	placeholders := make([]string, len(req.IDs))
	args := []interface{}{uid}
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	rows, err := DB.Query("SELECT id FROM lotteries WHERE user_id=? AND id IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()
	var validIDs []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		validIDs = append(validIDs, id)
	}
	if len(validIDs) == 0 {
		writeError(w, http.StatusBadRequest, "未找到可分享的彩票")
		return
	}
	// 生成 8 位分享码
	buf := make([]byte, 4)
	rand.Read(buf)
	code := hex.EncodeToString(buf)
	// 把 ID 列表存为逗号分隔
	idParts := make([]string, len(validIDs))
	for i, id := range validIDs {
		idParts[i] = strconv.FormatInt(id, 10)
	}
	idStr := strings.Join(idParts, ",")
	_, err = DB.Exec("INSERT INTO shares(code, user_id, lottery_ids) VALUES(?,?,?)", code, uid, idStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建分享失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "code": code})
}

// handleGetShare 公开接口（无需登录）：根据分享码查看彩票详情
func handleGetShare(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "分享码不能为空")
		return
	}
	var lotteryIDs string
	err := DB.QueryRow("SELECT lottery_ids FROM shares WHERE code=?", code).Scan(&lotteryIDs)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "分享不存在或已过期")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	ids := strings.Split(lotteryIDs, ",")
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, s := range ids {
		placeholders[i] = "?"
		args[i] = strings.TrimSpace(s)
	}
	rows, err := DB.Query(`SELECT l.id,l.type,l.issue,l.red_balls,l.blue_balls,l.play_type,l.multiple,l.banker_red,l.banker_blue,l.bets,l.status,l.prize_tier
		FROM lotteries l WHERE l.id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var l Lottery
		if err := rows.Scan(&l.ID, &l.Type, &l.Issue, &l.RedBalls, &l.BlueBalls, &l.PlayType, &l.Multiple, &l.BankerRed, &l.BankerBlue, &l.Bets, &l.Status, &l.PrizeTier); err != nil {
			continue
		}
		// 查对应开奖结果（含奖池金额）
		var dr, db string
		var poolAmount int64
		DB.QueryRow("SELECT red_balls, blue_balls, pool_amount FROM draw_results WHERE type=? AND (DATE(draw_date)=? OR issue=?)", l.Type, l.Issue, l.Issue).Scan(&dr, &db, &poolAmount)
		entry := map[string]interface{}{
			"type":        l.Type,
			"issue":       l.Issue,
			"red_balls":   l.RedBalls,
			"blue_balls":  l.BlueBalls,
			"play_type":   l.PlayType,
			"multiple":    l.Multiple,
			"banker_red":  l.BankerRed,
			"banker_blue": l.BankerBlue,
			"bets":        l.Bets,
			"status":      l.Status,
			"prize_tier":  l.PrizeTier,
			"draw_red":    dr,
			"draw_blue":   db,
			"pool_amount": poolAmount,
		}
		list = append(list, entry)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "list": list})
}
