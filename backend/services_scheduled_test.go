package main

import (
	"strings"
	"testing"
)

func TestBuildSummaryEmail(t *testing.T) {
	rows := []summaryRow{
		{Username: "alice", Type: TypeSSQ, Issue: "2026-08-13", RedBalls: "03,07,29,30,32,33", BlueBalls: "13",
			Status: StatusWin, Drawn: true, DrawRed: "01,02,03,04,05,06", DrawBlue: "13", MatchedRed: 1, MatchedBlue: 1, PrizeTier: "六等奖"},
		{Username: "bob", Type: TypeDLT, Issue: "2026-08-15", RedBalls: "01,02,03,04,05", BlueBalls: "06,07",
			Status: StatusNoWin, Drawn: true, DrawRed: "11,12,13,14,15", DrawBlue: "01,02"},
		{Username: "carol", Type: TypeSSQ, Issue: "2026-08-20", RedBalls: "07,08,09,10,11,12", BlueBalls: "01",
			Status: StatusPending, Drawn: false},
	}
	html := buildSummaryEmail(rows)
	for _, want := range []string{
		"彩票核对汇总",
		"扫描", "3", "张未开奖订单",
		"已中奖（1 张）", "alice", "六等奖", "1 红 1 蓝",
		"已开奖未中奖（1 张）", "bob",
		"尚未开奖（1 张）", "carol", "该期尚未开奖",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("汇总邮件缺少预期内容: %q", want)
		}
	}
	// 确认 HTML 结构基本闭合
	if !strings.HasPrefix(html, "<div") || !strings.HasSuffix(html, "</div>") {
		t.Errorf("汇总邮件 HTML 结构异常")
	}
}
