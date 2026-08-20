package main

import (
	"strings"
	"testing"
)

func TestBuildWinPushLength(t *testing.T) {
	mk := func(usr User, typ, lotIssue, drawIssue string, mr, mb int) WinRecord {
		return WinRecord{
			User:        usr,
			Lot:         Lottery{Type: typ, Issue: lotIssue},
			Draw:        DrawResult{Type: typ, Issue: drawIssue, PoolAmount: 20_0000_0000}, // 奖池20亿，福运奖生效
			MatchedRed:  mr,
			MatchedBlue: mb,
		}
	}
	owner := User{ID: 6, Username: "lovetwer", Email: "x@x.com"}

	cases := []struct {
		name      string
		usr       User
		wins      []WinRecord
		wantTier  string // 期望推送里出现的奖级（取最高奖级）
	}{
		{"单张-双色球五等奖", owner, []WinRecord{mk(owner, "ssq", "2026-08-13", "2026093", 3, 1)}, "五等奖"},
		{"单张-双色球三等奖", owner, []WinRecord{mk(owner, "ssq", "2026-08-13", "2026093", 5, 1)}, "三等奖"},
		{"单张-双色球一等奖", owner, []WinRecord{mk(owner, "ssq", "2026-08-13", "2026093", 6, 1)}, "一等奖"},
		{"单张-大乐透二等奖", owner, []WinRecord{mk(owner, "dlt", "2026-08-15", "26092", 5, 1)}, "二等奖"},
		{"多张-同彩种(取最高)", owner, []WinRecord{mk(owner, "ssq", "2026-08-13", "2026093", 3, 1), mk(owner, "ssq", "2026-08-13", "2026093", 5, 0), mk(owner, "ssq", "2026-08-13", "2026093", 1, 1)}, "四等奖"},
		{"多张-两彩种(取最高)", owner, []WinRecord{mk(owner, "ssq", "2026-08-13", "2026093", 3, 1), mk(owner, "dlt", "2026-08-15", "26092", 2, 1)}, "五等奖"},
		{"长用户名截断", User{ID: 99, Username: "zhangsan_super_long_name", Email: "y@y.com"}, []WinRecord{mk(User{ID: 99, Username: "zhangsan_super_long_name", Email: "y@y.com"}, "ssq", "2026-08-13", "2026093", 3, 1)}, "五等奖"},
		{"无用户名回退", User{ID: 123, Username: "", Email: "z@z.com"}, []WinRecord{mk(User{ID: 123, Username: "", Email: "z@z.com"}, "ssq", "2026-08-13", "2026093", 3, 1)}, "五等奖"},
	}

	for _, c := range cases {
		msg := buildWinPush(c.wins)
		r := []rune(msg)
		ok := len(r) <= 20
		// 必须带购买人：正常名查全名(或前8字前缀)，空用户名查"用户"回退
		var hasName bool
		if c.usr.Username == "" {
			hasName = strings.Contains(msg, "用户")
		} else {
			runes := []rune(c.usr.Username)
			k := 8
			if len(runes) < k {
				k = len(runes)
			}
			hasName = strings.Contains(msg, string(runes[:k]))
		}
		hasTier := strings.Contains(msg, c.wantTier)
		t.Logf("[%s] len=%d msg=%q  %s  含购买人=%v 含奖级%s=%v",
			c.name, len(r), msg, map[bool]string{true: "OK≤20", false: "超长!"}[ok], hasName, c.wantTier, hasTier)
		if !ok {
			t.Errorf("推送文案超过20字符: %q (%d)", msg, len(r))
		}
		if !hasName {
			t.Errorf("推送文案未带购买人: %q", msg)
		}
		if !hasTier {
			t.Errorf("推送文案未带奖级%s: %q", c.wantTier, msg)
		}
	}
}

func TestPrizeTier(t *testing.T) {
	// 福运奖只在奖池≥15亿时生效
	const highPool = int64(20_0000_0000) // 20亿
	const lowPool = int64(10_0000_0000)  // 10亿

	cases := []struct {
		typ     string
		mr, mb  int
		pool    int64
		want    string
	}{
		// 双色球（2026新规）
		{"ssq", 6, 1, 0, "一等奖"},
		{"ssq", 6, 0, 0, "二等奖"},
		{"ssq", 5, 1, 0, "三等奖"},
		{"ssq", 5, 0, 0, "四等奖"},
		{"ssq", 4, 1, 0, "四等奖"},
		{"ssq", 4, 0, 0, "五等奖"},
		{"ssq", 3, 1, 0, "五等奖"},
		{"ssq", 2, 1, 0, "六等奖"},
		{"ssq", 0, 1, 0, "六等奖"},
		{"ssq", 3, 0, highPool, "福运奖"},  // 2026新规：3红+0蓝，奖池≥15亿
		{"ssq", 3, 0, lowPool, ""},        // 奖池<15亿，福运奖不生效
		{"ssq", 2, 0, highPool, ""},       // 2红+0蓝未中奖
		// 大乐透（2026新规7等奖级）
		{"dlt", 5, 2, 0, "一等奖"},
		{"dlt", 5, 1, 0, "二等奖"},
		{"dlt", 5, 0, 0, "三等奖"},
		{"dlt", 4, 2, 0, "三等奖"},
		{"dlt", 4, 1, 0, "四等奖"},
		{"dlt", 3, 2, 0, "五等奖"},   // 2026新规：3+2从四等移到五等
		{"dlt", 4, 0, 0, "五等奖"},
		{"dlt", 3, 1, 0, "六等奖"},
		{"dlt", 2, 2, 0, "六等奖"},
		{"dlt", 3, 0, 0, "七等奖"},   // 2026新规
		{"dlt", 2, 1, 0, "七等奖"},
		{"dlt", 1, 2, 0, "七等奖"},
		{"dlt", 0, 2, 0, "七等奖"},
		{"dlt", 2, 0, 0, ""},       // 2+0未中奖
	}
	for _, c := range cases {
		got := prizeTier(c.typ, c.mr, c.mb, c.pool)
		if got != c.want {
			t.Errorf("prizeTier(%s,%d,%d,pool=%d)=%q, want %q", c.typ, c.mr, c.mb, c.pool, got, c.want)
		}
	}
}

func TestBuildWinSubject(t *testing.T) {
	mk := func(typ, issue string, mr, mb int) WinRecord {
		return WinRecord{
			User:        User{ID: 6, Username: "lovetwer", Email: "x@x.com"},
			Lot:         Lottery{Type: typ, Issue: issue},
			Draw:        DrawResult{Type: typ, Issue: issue, PoolAmount: 20_0000_0000},
			MatchedRed:  mr,
			MatchedBlue: mb,
		}
	}
	cases := []struct {
		name    string
		ws      []WinRecord
		wantSub string // 期望标题包含的子串（购买人无关，看奖级）
	}{
		{"单张双色球三等奖", []WinRecord{mk("ssq", "2026-08-13", 5, 1)}, "1 张双色球中三等奖"},
		{"单张双色球一等奖", []WinRecord{mk("ssq", "2026-08-13", 6, 1)}, "1 张双色球中一等奖"},
		{"单张大乐透二等奖", []WinRecord{mk("dlt", "2026-08-15", 5, 1)}, "1 张大乐透中二等奖"},
		{"多张同彩种取最高", []WinRecord{mk("ssq", "2026-08-13", 3, 1), mk("ssq", "2026-08-13", 5, 0)}, "2 张双色球中四等奖"},
		{"多张两彩种", []WinRecord{mk("ssq", "2026-08-13", 3, 1), mk("dlt", "2026-08-15", 2, 1)}, "2 张彩票中奖（最高五等奖）"},
	}
	for _, c := range cases {
		subj := buildWinSubject(c.ws)
		ok := strings.Contains(subj, c.wantSub)
		t.Logf("[%s] 标题=%q  期望含=%q  %v", c.name, subj, c.wantSub, ok)
		if !ok {
			t.Errorf("邮件标题未写明白奖级: %q (期望含 %q)", subj, c.wantSub)
		}
	}
}
