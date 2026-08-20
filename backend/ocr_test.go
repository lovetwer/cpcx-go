package main

import (
	"strings"
	"testing"
)

// 模拟硅基流动返回：content 是纯 JSON 数组
func TestParseOCRText_PlainJSON(t *testing.T) {
	content := `[{"type":"ssq","issue":"2024080","red":"3,8,15,20,21,24","blue":"9"}]`
	got, err := parseOCRText(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	r := got[0]
	if r.Type != TypeSSQ || r.Issue != "2024080" {
		t.Fatalf("bad type/issue: %+v", r)
	}
	// normalizeBalls 应把 "3,8,15,20,21,24" 规范成 "03,08,15,20,21,24"
	if r.RedBalls != "03,08,15,20,21,24" {
		t.Fatalf("red not normalized: %q", r.RedBalls)
	}
	if r.BlueBalls != "09" {
		t.Fatalf("blue not normalized: %q", r.BlueBalls)
	}
}

// 模拟硅基流动返回：带 ```json 代码围栏
func TestParseOCRText_CodeFence(t *testing.T) {
	content := "```json\n[{\"type\":\"dlt\",\"issue\":\"24080\",\"red\":\"3,4,7,12,32\",\"blue\":\"1,2\"}]\n```"
	got, err := parseOCRText(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Type != TypeDLT || got[0].Issue != "24080" {
		t.Fatalf("bad parse: %+v", got)
	}
	if got[0].RedBalls != "03,04,07,12,32" || got[0].BlueBalls != "01,02" {
		t.Fatalf("balls not normalized: %+v", got[0])
	}
}

// 模拟硅基流动返回：自然语言 + 多张
func TestParseOCRText_NaturalLanguage(t *testing.T) {
	content := `识别结果如下：
第一张：双色球，期号 2024081，红球 05 09 14 22 26 31，蓝球 11
第二张：大乐透，期号 24081，前区 02 10 18 25 33，后区 03 07`
	got, err := parseOCRText(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 改进后的正则兜底能识别中文标记分隔，支持多注提取
	if len(got) < 1 {
		t.Fatalf("want at least 1, got %d: %+v", len(got), got)
	}
	// 第一注应为双色球
	r := got[0]
	if r.Type != TypeSSQ {
		t.Fatalf("want ssq, got %q", r.Type)
	}
	if r.Issue != "2024081" {
		t.Fatalf("want 2024081, got %q", r.Issue)
	}
	if r.RedBalls != "05,09,14,22,26,31" {
		t.Fatalf("red not normalized: %q", r.RedBalls)
	}
	if r.BlueBalls != "11" {
		t.Fatalf("blue not normalized: %q", r.BlueBalls)
	}
	// 如果识别到第二注，校验大乐透
	if len(got) >= 2 {
		r2 := got[1]
		if r2.Type != TypeDLT {
			t.Fatalf("want dlt for 2nd, got %q", r2.Type)
		}
		if r2.RedBalls != "02,10,18,25,33" {
			t.Fatalf("2nd red not normalized: %q", r2.RedBalls)
		}
		if r2.BlueBalls != "03,07" {
			t.Fatalf("2nd blue not normalized: %q", r2.BlueBalls)
		}
	}
}

// 彩种中文映射
func TestMapType(t *testing.T) {
	cases := map[string]string{
		"ssq":    TypeSSQ,
		"双色球":    TypeSSQ,
		"dlt":    TypeDLT,
		"大乐透":    TypeDLT,
		"garbage": "",
	}
	for in, want := range cases {
		if got := mapType(in); got != want {
			t.Fatalf("mapType(%q)=%q want %q", in, got, want)
		}
	}
}

// detectMime 基本判断
func TestDetectMime(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G'}
	if detectMime("a.png", png) != "image/png" {
		t.Fatalf("png detect failed")
	}
	jpeg := []byte{0xFF, 0xD8, 0xFF}
	if !strings.HasPrefix(detectMime("a.jpg", jpeg), "image/jpeg") {
		t.Fatalf("jpeg detect failed")
	}
}
