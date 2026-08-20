package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// WinRecord 一张中奖票的命中信息，供聚合通知使用
type WinRecord struct {
	User        User
	Lot         Lottery
	Draw        DrawResult
	MatchedRed  int
	MatchedBlue int
}

// LoseRecord 一张未中奖票的信息，供聚合通知使用
type LoseRecord struct {
	User        User
	Lot         Lottery
	Draw        DrawResult
	MatchedRed  int
	MatchedBlue int
}

// flushWinNotifications 把一轮核对中所有中奖票和未中奖票聚合成最少量的通知发出：
//   - 中奖邮件：按用户邮箱分组，每个中奖用户只发【一封】汇总邮件（列出他全部中奖票）
//   - 未中奖邮件：按用户邮箱分组，每个未中奖用户只发【一封】汇总邮件（列出他全部未中奖票）
//   - 推送：机主（你）只推送中奖，按中奖用户分别推送，每条都带上购买人用户名（≤20 字）
//     未中奖不推送。
func flushWinNotifications(wins []WinRecord, loses []LoseRecord) {
	// —— 中奖通知 ——
	if len(wins) > 0 {
		// 1) 邮件：按用户聚合，每人一封
		if Cfg.MailPassword != "" {
			byUser := map[string][]WinRecord{}
			for _, w := range wins {
				if w.User.Email != "" {
					byUser[w.User.Email] = append(byUser[w.User.Email], w)
				}
			}
			for email, ws := range byUser {
				html := buildWinDigestEmail(ws)
				subject := buildWinSubject(ws)
				if err := sendMail([]string{email}, subject, html); err != nil {
					logErr("发送中奖汇总邮件失败 %s: %v", email, err)
				} else {
					logInfo("已向 %s 发送中奖汇总邮件（%d 张）", email, len(ws))
				}
			}
			for _, w := range wins {
				if w.User.Email == "" {
					logInfo("用户 %d 未配置邮箱，跳过中奖邮件通知", w.User.ID)
				}
			}
		} else {
			logInfo("邮件未启用（MailPassword 为空），跳过中奖邮件")
		}

		// 2) 推送：机主按中奖用户分别推送，每条带购买人用户名（≤20字）
		if Cfg.PushURL != "" {
			byUser := map[int64][]WinRecord{}
			for _, w := range wins {
				byUser[w.User.ID] = append(byUser[w.User.ID], w)
			}
			pushed := 0
			for _, recs := range byUser {
				if err := sendPush(buildWinPush(recs)); err != nil {
					logErr("推送中奖通知失败 用户%d: %v", recs[0].User.ID, err)
				} else {
					pushed++
				}
			}
			logInfo("已向机主推送中奖通知（%d 位用户中奖）", pushed)
		}
	}

	// —— 未中奖通知（只发邮件，不推送）——
	if len(loses) > 0 {
		// 1) 邮件：按用户聚合，每人一封
		if Cfg.MailPassword != "" {
			byUser := map[string][]LoseRecord{}
			for _, l := range loses {
				if l.User.Email != "" {
					byUser[l.User.Email] = append(byUser[l.User.Email], l)
				}
			}
			for email, ls := range byUser {
				html := buildLoseDigestEmail(ls)
				subject := buildLoseSubject(ls)
				if err := sendMail([]string{email}, subject, html); err != nil {
					logErr("发送未中奖汇总邮件失败 %s: %v", email, err)
				} else {
					logInfo("已向 %s 发送未中奖汇总邮件（%d 张）", email, len(ls))
				}
			}
			for _, l := range loses {
				if l.User.Email == "" {
					logInfo("用户 %d 未配置邮箱，跳过未中奖邮件通知", l.User.ID)
				}
			}
		} else {
			logInfo("邮件未启用（MailPassword 为空），跳过未中奖邮件")
		}
	}
}

// buildWinDigestEmail 给单个用户的多张中奖票生成一封汇总邮件（HTML 表格）
func buildWinDigestEmail(ws []WinRecord) string {
	u := ws[0].User
	var rows strings.Builder
	for _, w := range ws {
		rows.WriteString(fmt.Sprintf(`<tr>
<td style="padding:8px 6px;border-bottom:1px solid #eee">%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">红:%s / 蓝:%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">红:%s / 蓝:%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee"><b>%d 红 %d 蓝</b></td>
<td style="padding:8px 6px;border-bottom:1px solid #eee;color:#d4380d"><b>%s</b></td>
</tr>`, TypeName(w.Lot.Type), w.Lot.Issue, w.Lot.RedBalls, w.Lot.BlueBalls, w.Draw.RedBalls, w.Draw.BlueBalls, w.MatchedRed, w.MatchedBlue, prizeTier(w.Lot.Type, w.MatchedRed, w.MatchedBlue)))
	}
	return fmt.Sprintf(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;max-width:560px;margin:0 auto;padding:24px;background:#fff;border:1px solid #eee;border-radius:12px">
<h2 style="color:#d4380d;margin:0 0 12px">🎉 恭喜中奖！</h2>
<p>尊敬的 <b>%s</b>，您共有 <b>%d</b> 张彩票命中奖项，明细如下：</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px">
<tr style="color:#888;text-align:left"><th style="padding:6px">彩种</th><th style="padding:6px">期号</th><th style="padding:6px">您的号码</th><th style="padding:6px">开奖号码</th><th style="padding:6px">命中</th><th style="padding:6px">几等奖</th></tr>
%s
</table>
<p style="margin-top:16px;color:#888;font-size:13px">请凭彩票尽快兑奖。本邮件由大奖来了自动发送。</p>
</div>`, u.Username, len(ws), rows.String())
}

// buildWinSubject 生成中奖邮件标题，明确写出奖级（如「🎉 恭喜！您有3张双色球中四等奖」）。
// 单彩种写明彩种+最高奖级；多彩种统称"彩票"并标注最高奖级。
func buildWinSubject(ws []WinRecord) string {
	n := len(ws)
	tier := bestTierOf(ws)
	typeSet := map[string]bool{}
	for _, w := range ws {
		typeSet[TypeName(w.Lot.Type)] = true
	}
	typeLabel := "彩票"
	if len(typeSet) == 1 {
		for t := range typeSet {
			typeLabel = t
		}
	}
	if tier == "" {
		if len(typeSet) == 1 {
			return fmt.Sprintf("🎉 恭喜！您有 %d 张%s彩票中奖", n, typeLabel)
		}
		return fmt.Sprintf("🎉 恭喜！您有 %d 张彩票中奖", n)
	}
	if len(typeSet) == 1 {
		return fmt.Sprintf("🎉 恭喜！您有 %d 张%s中%s", n, typeLabel, tier)
	}
	return fmt.Sprintf("🎉 恭喜！您有 %d 张彩票中奖（最高%s）", n, tier)
}

// buildWinPush 生成【单用户】中奖推送文本（letserver 限制 msg 最多 20 字符，且必须带购买人 + 奖级）。
// 形如「🎉{用户名}中{n}张{彩种}{奖级}」；用户名超长则截断，确保始终 ≤ 20 rune。
func buildWinPush(recs []WinRecord) string {
	if len(recs) == 0 {
		return "🎉本期有中奖"
	}
	u := recs[0].User
	name := u.Username
	if name == "" {
		name = fmt.Sprintf("用户%d", u.ID)
	}
	n := len(recs)
	// 彩种标签：仅一种则写名称，多种则统称「彩票」
	typeSet := map[string]bool{}
	for _, w := range recs {
		typeSet[TypeName(w.Lot.Type)] = true
	}
	typeLabel := "彩票"
	if len(typeSet) == 1 {
		for t := range typeSet {
			typeLabel = t
		}
	}
	// 取该用户本轮最高奖级（如"三等奖"），写入推送
	tier := bestTierOf(recs)
	suffix := fmt.Sprintf("中%d张%s", n, typeLabel)
	if tier != "" {
		suffix += tier
	}
	text := "🎉" + name + suffix
	if len([]rune(text)) > 20 {
		reserved := 1 + len([]rune(suffix)) // 🎉 + 后缀
		budget := 20 - reserved
		if budget < 1 {
			budget = 1
		}
		runes := []rune(name)
		if len(runes) > budget {
			name = string(runes[:budget])
		}
		text = "🎉" + name + suffix
	}
	return text
}

// buildLoseDigestEmail 给单个用户的多张未中奖票生成一封汇总邮件（HTML 表格）
func buildLoseDigestEmail(ls []LoseRecord) string {
	u := ls[0].User
	var rows strings.Builder
	for _, l := range ls {
		rows.WriteString(fmt.Sprintf(`<tr>
<td style="padding:8px 6px;border-bottom:1px solid #eee">%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">红:%s / 蓝:%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee">红:%s / 蓝:%s</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee;color:#999">%d 红 %d 蓝</td>
<td style="padding:8px 6px;border-bottom:1px solid #eee;color:#999">未中奖</td>
</tr>`, TypeName(l.Lot.Type), l.Lot.Issue, l.Lot.RedBalls, l.Lot.BlueBalls, l.Draw.RedBalls, l.Draw.BlueBalls, l.MatchedRed, l.MatchedBlue))
	}
	return fmt.Sprintf(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;max-width:560px;margin:0 auto;padding:24px;background:#fff;border:1px solid #eee;border-radius:12px">
<h2 style="color:#666;margin:0 0 12px">😅 本期未中奖</h2>
<p>尊敬的 <b>%s</b>，您共有 <b>%d</b> 张彩票已开奖但未命中奖项，明细如下：</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px">
<tr style="color:#888;text-align:left"><th style="padding:6px">彩种</th><th style="padding:6px">期号</th><th style="padding:6px">您的号码</th><th style="padding:6px">开奖号码</th><th style="padding:6px">命中</th><th style="padding:6px">结果</th></tr>
%s
</table>
<p style="margin-top:16px;color:#888;font-size:13px">感谢使用大奖来了，祝您下次好运！本邮件由系统自动发送。</p>
</div>`, u.Username, len(ls), rows.String())
}

// buildLoseSubject 生成未中奖邮件标题
func buildLoseSubject(ls []LoseRecord) string {
	n := len(ls)
	typeSet := map[string]bool{}
	for _, l := range ls {
		typeSet[TypeName(l.Lot.Type)] = true
	}
	if len(typeSet) == 1 {
		var label string
		for t := range typeSet {
			label = t
		}
		return fmt.Sprintf("😅 您的 %d 张%s未中奖", n, label)
	}
	return fmt.Sprintf("😅 您的 %d 张彩票未中奖", n)
}

// prizeTier 根据彩种与命中红/蓝球数返回中奖等级（如"三等奖"）；未中奖返回空串。
// 2026 新规则：双色球增设福运奖（3红+0蓝），大乐透合并为7个奖级。
func prizeTier(lotType string, mr, mb int) string {
	switch TypeName(lotType) {
	case "双色球":
		switch {
		case mr == 6 && mb >= 1:
			return "一等奖"
		case mr == 6:
			return "二等奖"
		case mr == 5 && mb >= 1:
			return "三等奖"
		case mr == 5 || (mr == 4 && mb >= 1):
			return "四等奖"
		case mr == 4 || (mr == 3 && mb >= 1):
			return "五等奖"
		case mb >= 1:
			return "六等奖"
		case mr == 3 && mb == 0:
			return "福运奖"
		}
	case "大乐透":
		switch {
		case mr == 5 && mb >= 2:
			return "一等奖"
		case mr == 5 && mb == 1:
			return "二等奖"
		case mr == 5 || (mr == 4 && mb >= 2):
			return "三等奖"
		case mr == 4 && mb == 1:
			return "四等奖"
		case mr == 4 || (mr == 3 && mb >= 2):
			return "五等奖"
		case mr == 3 && mb == 1 || mr == 2 && mb >= 2:
			return "六等奖"
		case (mr == 3 && mb == 0) || (mr == 2 && mb == 1) || (mr == 1 && mb == 2) || (mr == 0 && mb == 2):
			return "七等奖"
		}
	}
	return ""
}

// bestTierOf 返回一组中奖票中的最高奖级（一等奖优先）；无则空串。
func bestTierOf(recs []WinRecord) string {
	rank := map[string]int{"一等奖": 1, "二等奖": 2, "三等奖": 3, "四等奖": 4, "五等奖": 5, "六等奖": 6, "七等奖": 7, "福运奖": 8}
	best, bestR := "", 99
	for _, w := range recs {
		t := prizeTier(w.Lot.Type, w.MatchedRed, w.MatchedBlue)
		if t == "" {
			continue
		}
		if r, ok := rank[t]; ok && r < bestR {
			bestR, best = r, t
		}
	}
	return best
}

// sendMail 通过 SMTP(Resend 隐式 TLS) 发送 HTML 邮件，支持多个收件人。
// 凭证来自 config.go 的 MAIL_*：host=smtp.resend.com, port=2465, 账号 resend, 密码=API Key, 发件人 prize@...
func sendMail(to []string, subject, html string) error {
	host := Cfg.MailHost
	port := Cfg.MailPort
	from := Cfg.MailFrom
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("smtp 连接 %s 失败: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp 客户端初始化失败: %w", err)
	}
	defer client.Quit()

	if err := client.Auth(smtp.PlainAuth("", Cfg.MailUsername, Cfg.MailPassword, host)); err != nil {
		return fmt.Errorf("smtp 认证失败: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM 失败: %w", err)
	}
	for _, rcpt := range to {
		if rcpt == "" {
			continue
		}
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT %s 失败: %w", rcpt, err)
		}
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA 失败: %w", err)
	}
	msg := buildMIME(from, to, subject, html)
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp 写入邮件失败: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp 结束邮件失败: %w", err)
	}
	return nil
}

// buildMIME 组装符合 RFC 822 的邮件头与 base64 编码的 HTML 正文（兼容中文）
func buildMIME(from string, to []string, subject, html string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: =?UTF-8?B?" + b64(subject) + "?=\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	b.WriteString(b64(html))
	return b.String()
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// sendPush 通过 letserver.run 的 GET 接口推送一条消息（token 固定为机主的）
func sendPush(msg string) error {
	base := strings.TrimRight(Cfg.PushURL, "?&")
	if base == "" {
		return nil
	}
	q := url.Values{}
	if Cfg.PushToken != "" {
		q.Set("token", Cfg.PushToken)
	}
	q.Set("msg", msg)
	full := base + "?" + q.Encode()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(full)
	if err != nil {
		return fmt.Errorf("push 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("push 返回 %d", resp.StatusCode)
	}
	return nil
}

// handleAdminTestMail 手动发送一封测试邮件，验证 SMTP 配置是否可用（管理员）
func handleAdminTestMail(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		to = Cfg.MailFrom
	}
	subject := "大奖来了邮件测试"
	html := fmt.Sprintf(`<p style="font-family:-apple-system,Segoe UI,Roboto,sans-serif">这是一封来自大奖来了的测试邮件，发送时间 %s。</p>
<p style="color:#888;font-size:12px">若您收到此邮件，说明 SMTP(Resend) 配置工作正常。</p>`, time.Now().Format("2006-01-02 15:04:05"))
	if err := sendMail([]string{to}, subject, html); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "msg": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "msg": "测试邮件已发送至 " + to})
}
