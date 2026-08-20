package main

import (
	"database/sql"
	"strings"
)

// CheckAll 只核对「待开奖」的彩票，与开奖结果比对并更新状态和奖级。
// 已经判定过结果的（中奖/未中奖）不再碰，彻底避免历史数据触发重复通知。
// notify=true 时对首次出结果的票（中奖或未中奖）发出通知；notify=false 时只更新状态不发通知。
// 邮件：中奖和未中奖都发给用户；推送：只推送中奖给机主。
func CheckAll(notify bool) error {
	rows, err := DB.Query(`SELECT id,user_id,type,issue,red_balls,blue_balls,banker_red,banker_blue,status FROM lotteries WHERE status=? ORDER BY id`, StatusPending)
	if err != nil {
		return err
	}
	defer rows.Close()
	var lots []Lottery
	for rows.Next() {
		var l Lottery
		if err := rows.Scan(&l.ID, &l.UserID, &l.Type, &l.Issue, &l.RedBalls, &l.BlueBalls, &l.BankerRed, &l.BankerBlue, &l.Status); err != nil {
			continue
		}
		lots = append(lots, l)
	}
	var wins []WinRecord
	var loses []LoseRecord
	for i := range lots {
		wr, lr := checkOne(&lots[i])
		if wr != nil {
			wins = append(wins, *wr)
		}
		if lr != nil {
			loses = append(loses, *lr)
		}
	}
	if notify {
		flushWinNotifications(wins, loses)
	}
	logInfo("核对完成（notify=%v），处理 %d 张彩票，其中 %d 张新中奖，%d 张新未中奖", notify, len(lots), len(wins), len(loses))
	return nil
}

// checkOne 核对单张彩票；当状态从「未开奖」首次变为「中奖」或「未中奖」时返回对应记录触发通知。
// 已经是中奖/未中奖的票只更新 prize_tier（如规则变动导致奖级调整），但不会重复发通知。
func checkOne(lot *Lottery) (*WinRecord, *LoseRecord) {
	var d DrawResult
	// 兼容两种 issue 存储：票面日期（如 2026-08-13）或期号（如 2026093）。
	err := DB.QueryRow(`SELECT id,type,issue,red_balls,blue_balls,draw_date,pool_amount,fyj_count,fyj_money FROM draw_results
		WHERE type=? AND (DATE(draw_date)=? OR issue=?)`,
		lot.Type, lot.Issue, lot.Issue).Scan(&d.ID, &d.Type, &d.Issue, &d.RedBalls, &d.BlueBalls, &d.DrawDate, &d.PoolAmount, &d.FyjCount, &d.FyjMoney)
	if err == sql.ErrNoRows {
		// 该期尚未抓取开奖，保持"未开奖"
		return nil, nil
	}
	if err != nil {
		logErr("核对查询开奖失败: %v", err)
		return nil, nil
	}
	// 复式/胆拖需展开为所有单式组合，任一组合命中即中奖
	combos := enumerateTicketCombos(lot.Type, lot.RedBalls, lot.BlueBalls, lot.BankerRed, lot.BankerBlue)
	won := false
	bestMr, bestMb := 0, 0
	for _, c := range combos {
		w, mr, mb := MatchResult(lot.Type, strings.Join(c[0], ","), strings.Join(c[1], ","), d.RedBalls, d.BlueBalls, d.PoolAmount)
		if w {
			won = true
		}
		if mr > bestMr {
			bestMr = mr
		}
		if mb > bestMb {
			bestMb = mb
		}
	}
	newStatus := StatusNoWin
	newTier := ""
	if won {
		newStatus = StatusWin
		newTier = prizeTier(lot.Type, bestMr, bestMb, d.PoolAmount)
	}
	// 查当前奖级
	var oldTier string
	DB.QueryRow("SELECT prize_tier FROM lotteries WHERE id=?", lot.ID).Scan(&oldTier)

	// 状态和奖级都没变，完全跳过
	if lot.Status == newStatus && oldTier == newTier {
		return nil, nil
	}

	// 记录之前的状态，用于判断是否为「首次出结果」
	wasWin := lot.Status == StatusWin
	wasNoWin := lot.Status == StatusNoWin

	if _, err := DB.Exec("UPDATE lotteries SET status=?, prize_tier=? WHERE id=?", newStatus, newTier, lot.ID); err != nil {
		logErr("更新彩票状态失败 id=%d: %v", lot.ID, err)
		return nil, nil
	}
	lot.Status = newStatus

	// 首次中奖（之前不是中奖，现在中奖了）
	if won && !wasWin {
		if u, ok := getUserByID(lot.UserID); ok {
			return &WinRecord{User: *u, Lot: *lot, Draw: d, MatchedRed: bestMr, MatchedBlue: bestMb}, nil
		}
		logErr("中奖票 id=%d 找不到对应用户，无法通知", lot.ID)
	}

	// 首次未中奖（之前是未开奖，现在未中奖了；已经判定过未中奖的不重复通知）
	if !won && !wasNoWin {
		if u, ok := getUserByID(lot.UserID); ok {
			return nil, &LoseRecord{User: *u, Lot: *lot, Draw: d, MatchedRed: bestMr, MatchedBlue: bestMb}
		}
		logErr("未中奖票 id=%d 找不到对应用户，无法通知", lot.ID)
	}

	return nil, nil
}
