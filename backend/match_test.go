package main

import "testing"

func TestNormalizeBalls(t *testing.T) {
	cases := []struct{ in, want string }{
		{"5, 12, 3", "03,05,12"},
		{" 05+08+15 ", "05,08,15"},
		{"10 20 30", "10,20,30"},
	}
	for _, c := range cases {
		if got := normalizeBalls(c.in); got != c.want {
			t.Errorf("normalizeBalls(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestMatchSSQ(t *testing.T) {
	// 福运奖只在奖池≥15亿时生效，测试用20亿(20_0000_0000)代表生效，10亿代表不生效
	cases := []struct {
		red, blue, dRed, dBlue string
		poolAmount             int64
		won                    bool
	}{
		{"05,08,15,20,21,24", "09", "05,08,15,20,21,24", "09", 0, true},      // 6+1 一等奖
		{"05,08,15,20,21,24", "10", "05,08,15,20,21,24", "09", 0, true},      // 6+0 二等奖
		{"01,02,03,04,05,06", "07", "01,02,03,09,10,11", "08", 20_0000_0000, true},  // 3+0 福运奖（奖池≥15亿）
		{"01,02,03,04,05,06", "07", "01,02,03,09,10,11", "08", 10_0000_0000, false}, // 3+0 福运奖未生效（奖池<15亿）
		{"01,02,03,04,05,06", "07", "01,02,03,09,10,11", "07", 0, true},      // 3+1 五等奖
		{"01,02,03,04,05,06", "07", "11,12,13,14,15,16", "07", 0, true},      // 0+1 六等奖
		{"01,02,03,04,05,06", "07", "11,12,13,14,15,16", "08", 0, false},     // 0+0 未中奖
	}
	for _, c := range cases {
		won, _, _ := MatchResult(TypeSSQ, c.red, c.blue, c.dRed, c.dBlue, c.poolAmount)
		if won != c.won {
			t.Errorf("ssq %s/%s vs %s/%s pool=%d => won=%v want %v", c.red, c.blue, c.dRed, c.dBlue, c.poolAmount, won, c.won)
		}
	}
}

func TestMatchDLT(t *testing.T) {
	cases := []struct {
		red, blue, dRed, dBlue string
		won                    bool
	}{
		{"03,04,07,12,32", "01,02", "03,04,07,12,32", "01,02", true},  // 5+2 一等奖
		{"03,04,07,12,32", "01,02", "03,04,07,12,32", "09,08", true},  // 5+0 三等奖
		{"03,04,07,12,32", "01,02", "03,04,07,99,98", "09,08", true},  // 3+0 七等奖（2026新规）
		{"03,04,07,12,32", "01,02", "03,04,07,99,98", "01,02", true},  // 3+2 五等奖
		{"03,04,07,12,32", "01,02", "99,98,97,96,95", "01,02", true},  // 0+2 七等奖
		{"03,04,07,12,32", "01,02", "99,98,97,96,95", "09,08", false}, // 0+0 未中奖
	}
	for _, c := range cases {
		won, _, _ := MatchResult(TypeDLT, c.red, c.blue, c.dRed, c.dBlue, 0)
		if won != c.won {
			t.Errorf("dlt %s/%s vs %s/%s => won=%v want %v", c.red, c.blue, c.dRed, c.dBlue, won, c.won)
		}
	}
}
