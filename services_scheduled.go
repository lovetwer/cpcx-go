package main

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

// scheduledTimes 每天固定触发的时刻（本地时间）
// 需求：早上 09:00，晚上 21:40、22:00、22:30、23:00
var scheduledTimes = []struct {
	h, m int
}{
	{9, 0}, {21, 40}, {22, 0}, {22, 30}, {23, 0},
}

// nextScheduledTime 计算从 now 起最近的下一个触发时刻（含跨天）
func nextScheduledTime(now time.Time) time.Time {
	best := time.Time{}
	for _, t := range scheduledTimes {
		c := time.Date(now.Year(), now.Month(), now.Day(), t.h, t.m, 0, 0, now.Location())
		if c.Before(now) {
			c = c.AddDate(0, 0, 1) // 今天已过，顺延到明天同一时刻
		}
		if best.IsZero() || c.Before(best) {
			best = c
		}
	}
	return best
}

// isAtScheduledTime 判断 t 的分针/时是否与某个固定时刻精确对齐（用于唤醒后校验，避免误触发）
func isAtScheduledTime(t time.Time) bool {
	for _, s := range scheduledTimes {
		if t.Hour() == s.h && t.Minute() == s.m {
			return true
		}
	}
	return false
}

// startFixedSchedule 按固定时刻循环触发核对+通知。
// 用 time.Sleep 到点唤醒，唤醒后再校验时刻，规避系统休眠/时钟漂移导致的误触发。
func startFixedSchedule() {
	go func() {
		for {
			now := time.Now()
			next := nextScheduledTime(now)
			d := next.Sub(now)
			logInfo("下次定时核对时间: %s（约 %s 后）", next.Format("2006-01-02 15:04"), d.Round(time.Second))
			time.Sleep(d)
			if !isAtScheduledTime(time.Now()) {
				continue
			}
			ScheduledCheckAndNotify()
		}
	}()
}

// ScheduledCheckAndNotify 定时核对并对中奖结果发通知。
// 通知规则（由用户确认）：
//   - 邮件：按中奖用户聚合，每人只发【一封】汇总邮件（列清他全部中奖票），不论中多少注
//   - 推送：每轮核对只给机主（你）发【一条】汇总推送，写清总张数与涉及用户数
//   邮件/推送均在 CheckAll 内部统一聚合后发出，不会被 SMTP 限流。
// NOTIFY_EMAILS 仅作为“额外给自己发一封汇总摘要”的可选项，留空则不发摘要。
func ScheduledCheckAndNotify() {
	logInfo("定时核对与通知：开始")

	// 1. 拉取最新开奖（确保 draw_results 最新）
	if _, err := PullLottery(); err != nil {
		logErr("定时核对：拉奖失败 %v", err)
	}

	// 2. 快照当前所有“未开奖”订单（核对前），供可选汇总使用
	pendings, err := fetchPendingLotteries()
	if err != nil {
		logErr("定时核对：查询未开奖订单失败 %v", err)
		return
	}

	// 3. 执行核对：新中奖的票在 CheckAll 内汇总后发出（每用户一封邮件 + 机主一条推送）
	if err := CheckAll(true); err != nil {
		logErr("定时核对：CheckAll 失败 %v", err)
	}

	// 4. 可选：若配置了 NOTIFY_EMAILS，再发一封汇总摘要给你（不配置则跳过，中奖用户已各自收到邮件）
	if Cfg.NotifyEmails == "" {
		logInfo("定时核对完成：新中奖用户已邮件通知本人，您已收到推送；未配置 NOTIFY_EMAILS，跳过汇总摘要")
		return
	}
	rows, err := buildSummary(pendings)
	if err != nil {
		logErr("定时核对：汇总失败 %v", err)
		return
	}
	if len(rows) == 0 {
		logInfo("定时核对完成：无未开奖订单，跳过汇总摘要")
		return
	}
	body := buildSummaryEmail(rows)
	subject := fmt.Sprintf("彩票核对汇总 %s", time.Now().Format("2006-01-02 15:04"))
	for _, to := range strings.Split(Cfg.NotifyEmails, ",") {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		if err := sendMail([]string{to}, subject, body); err != nil {
			logErr("定时核对：发送汇总邮件失败 %s: %v", to, err)
		} else {
			logInfo("定时核对：已向 %s 发送汇总邮件", to)
		}
	}
}

// pendingLot 是“未开奖”订单的快照字段
type pendingLot struct {
	ID        int64
	UserID    int64
	Type      string
	Issue     string
	RedBalls  string
	BlueBalls string
}

// fetchPendingLotteries 返回当前所有“未开奖”订单（核对前快照）
func fetchPendingLotteries() ([]pendingLot, error) {
	rows, err := DB.Query(`SELECT l.id, l.user_id, l.type, l.issue, l.red_balls, l.blue_balls
		FROM lotteries l WHERE l.status=? ORDER BY l.created_at DESC`, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pendingLot
	for rows.Next() {
		var p pendingLot
		if err := rows.Scan(&p.ID, &p.UserID, &p.Type, &p.Issue, &p.RedBalls, &p.BlueBalls); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// summaryRow 汇总邮件里的一行
type summaryRow struct {
	Username    string
	Type        string
	Issue       string
	RedBalls    string
	BlueBalls   string
	Status      string // 核对后的最新状态
	PrizeTier   string
	DrawRed     string
	DrawBlue    string
	Drawn       bool // 该期是否已开奖
	MatchedRed  int
	MatchedBlue int
}

// buildSummary 为每张快照订单补齐最新状态、对应开奖及命中详情
func buildSummary(pendings []pendingLot) ([]summaryRow, error) {
	rows := make([]summaryRow, 0, len(pendings))
	for _, p := range pendings {
		r := summaryRow{Type: p.Type, Issue: p.Issue, RedBalls: p.RedBalls, BlueBalls: p.BlueBalls}
		// 最新状态 / 奖级
		var status, prizeTier string
		if err := DB.QueryRow("SELECT status, prize_tier FROM lotteries WHERE id=?", p.ID).Scan(&status, &prizeTier); err != nil {
			continue
		}
		r.Status = status
		r.PrizeTier = prizeTier
		// 对应开奖（兼容 issue 为日期或期号两种存储）
		var dr, db string
		err := DB.QueryRow(`SELECT red_balls, blue_balls FROM draw_results
			WHERE type=? AND (DATE(draw_date)=? OR issue=?)`, p.Type, p.Issue, p.Issue).Scan(&dr, &db)
		if err == nil {
			r.Drawn = true
			r.DrawRed = dr
			r.DrawBlue = db
		}
		// 中奖命中详情
		if status == StatusWin && r.Drawn {
			_, mr, mb := MatchResult(p.Type, p.RedBalls, p.BlueBalls, dr, db)
			r.MatchedRed = mr
			r.MatchedBlue = mb
		}
		// 下单用户
		if u, ok := getUserByID(p.UserID); ok {
			r.Username = u.Username
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// buildSummaryEmail 组装 HTML 汇总邮件，分“已中奖 / 已开奖未中奖 / 尚未开奖”三栏
func buildSummaryEmail(rows []summaryRow) string {
	var won, lost, pending []summaryRow
	for _, r := range rows {
		switch r.Status {
		case StatusWin:
			won = append(won, r)
		case StatusNoWin:
			lost = append(lost, r)
		default:
			pending = append(pending, r)
		}
	}

	var b strings.Builder
	b.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;max-width:760px;margin:0 auto;padding:20px;color:#222;font-size:14px">`)
	b.WriteString(`<h2 style="color:#d4380d;margin:0 0 4px">彩票核对汇总</h2>`)
	b.WriteString(fmt.Sprintf(`<p style="color:#888;margin:0 0 16px">生成时间：%s</p>`, time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf(
		`<p style="margin:0 0 16px">本次扫描 <b>%d</b> 张未开奖订单：已开奖 <b>%d</b> 张（中奖 <b>%d</b> / 未中奖 <b>%d</b>），尚未开奖 <b>%d</b> 张。</p>`,
		len(rows), len(won)+len(lost), len(won), len(lost), len(pending)))

	if len(won) > 0 {
		b.WriteString(fmt.Sprintf(`<h3 style="color:#d4380d;margin:18px 0 8px">已中奖（%d 张）</h3>`, len(won)))
		b.WriteString(summaryTable(won, "win"))
	}
	if len(lost) > 0 {
		b.WriteString(fmt.Sprintf(`<h3 style="color:#555;margin:18px 0 8px">已开奖未中奖（%d 张）</h3>`, len(lost)))
		b.WriteString(summaryTable(lost, "lost"))
	}
	if len(pending) > 0 {
		b.WriteString(fmt.Sprintf(`<h3 style="color:#888;margin:18px 0 8px">尚未开奖（%d 张）</h3>`, len(pending)))
		b.WriteString(summaryTable(pending, "pending"))
	}

	b.WriteString(`<p style="margin-top:20px;color:#aaa;font-size:12px">本邮件由大奖来了定时核对任务自动发送。</p>`)
	b.WriteString(`</div>`)
	return b.String()
}

// summaryTable 渲染汇总表格；kind 决定列与配色
func summaryTable(list []summaryRow, kind string) string {
	var b strings.Builder
	head := ""
	switch kind {
	case "win":
		head = `<tr><th>用户</th><th>彩种</th><th>期号</th><th>投注(红/蓝)</th><th>开奖(红/蓝)</th><th>命中</th><th>奖级</th></tr>`
	case "lost":
		head = `<tr><th>用户</th><th>彩种</th><th>期号</th><th>投注(红/蓝)</th><th>开奖(红/蓝)</th></tr>`
	default:
		head = `<tr><th>用户</th><th>彩种</th><th>期号</th><th>投注(红/蓝)</th><th>备注</th></tr>`
	}
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:13px">`)
	b.WriteString(`<thead style="background:#fafafa;color:#666">` + head + `</thead><tbody>`)
	for _, r := range list {
		b.WriteString(`<tr style="border-bottom:1px solid #eee">`)
		b.WriteString(td(esc(r.Username)))
		b.WriteString(td(esc(TypeName(r.Type))))
		b.WriteString(td(esc(r.Issue)))
		b.WriteString(td(fmt.Sprintf("%s / %s", esc(r.RedBalls), esc(r.BlueBalls))))
		switch kind {
		case "win":
			b.WriteString(td(fmt.Sprintf("%s / %s", esc(r.DrawRed), esc(r.DrawBlue))))
			b.WriteString(td(fmt.Sprintf("%d 红 %d 蓝", r.MatchedRed, r.MatchedBlue)))
			tier := r.PrizeTier
			if tier == "" {
				tier = "已中奖"
			}
			b.WriteString(td(esc(tier)))
		case "lost":
			b.WriteString(td(fmt.Sprintf("%s / %s", esc(r.DrawRed), esc(r.DrawBlue))))
		default:
			b.WriteString(td("该期尚未开奖"))
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func td(s string) string {
	return fmt.Sprintf(`<td style="padding:6px 8px">%s</td>`, s)
}

func esc(s string) string {
	return html.EscapeString(s)
}

// handleAdminNotify 手动触发“核对并汇总通知”（管理员），便于不等待固定时刻即刻验证
func handleAdminNotify(w http.ResponseWriter, r *http.Request) {
	ScheduledCheckAndNotify()
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "msg": "已执行定时核对与通知任务"})
}
