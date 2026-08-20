package main

import (
	"net/http"
	"strings"
)

// handleListDraw 查询官方开奖结果
func handleListDraw(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var conds []string
	var args []interface{}
	if t := q.Get("type"); t != "" {
		conds = append(conds, "type=?")
		args = append(args, t)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := DB.Query("SELECT id,type,issue,red_balls,blue_balls,draw_date,pool_amount,fyj_count,fyj_money,created_at FROM draw_results "+where+" ORDER BY id DESC LIMIT 100", args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()
	var list []DrawResult
	for rows.Next() {
		var d DrawResult
		if err := rows.Scan(&d.ID, &d.Type, &d.Issue, &d.RedBalls, &d.BlueBalls, &d.DrawDate, &d.PoolAmount, &d.FyjCount, &d.FyjMoney, &d.CreatedAt); err != nil {
			continue
		}
		list = append(list, d)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "list": list})
}

// handlePull 手动触发拉奖（管理员）
func handlePull(w http.ResponseWriter, r *http.Request) {
	if _, err := PullLottery(); err != nil {
		writeError(w, http.StatusInternalServerError, "拉奖失败："+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "msg": "拉奖完成"})
}

// handleCheck 手动触发核对（管理员）
func handleCheck(w http.ResponseWriter, r *http.Request) {
	if err := CheckAll(true); err != nil {
		writeError(w, http.StatusInternalServerError, "核对失败："+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "msg": "核对完成"})
}
