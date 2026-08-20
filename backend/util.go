package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ---------- JSON 辅助 ----------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"ok": false, "msg": msg})
}

func readJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// ---------- 号码处理 ----------

// normalizeBalls 把 "5, 12, 03" / "5 12 3" / "5+12+3" 统一成 "03,05,12"
func normalizeBalls(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '+' || r == '|' || r == '/' || r == ';'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, p) // 非数字原样保留
			continue
		}
		out = append(out, pad2(n))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// splitNums 按逗号拆分并 trim
func splitNums(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// intersectCount 返回 a 中有多少个元素出现在 b 中（按字符串相等）
func intersectCount(a, b []string) int {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	c := 0
	for _, x := range a {
		if _, ok := set[x]; ok {
			c++
		}
	}
	return c
}

// MatchResult 比对用户彩票与开奖结果，返回是否中奖及命中数
// 2026 新规则：双色球增设福运奖（3红+0蓝），大乐透合并为7个奖级。
// poolAmount 为该期奖池金额（元），用于判断福运奖是否生效（SSQ奖池≥15亿才触发）。
func MatchResult(lotType, redBalls, blueBalls, drawRed, drawBlue string, poolAmount int64) (won bool, matchedRed, matchedBlue int) {
	userRed := splitNums(redBalls)
	userBlue := splitNums(blueBalls)
	drawRedSet := splitNums(drawRed)
	drawBlueSet := splitNums(drawBlue)
	matchedRed = intersectCount(userRed, drawRedSet)
	matchedBlue = intersectCount(userBlue, drawBlueSet)

	switch lotType {
	case TypeSSQ:
		// 双色球（2026新规）：蓝命中即六等奖以上；红>=4 中奖；3红+0蓝=福运奖（仅奖池≥15亿时生效）
		fyjActive := poolAmount >= 15_0000_0000 // 15亿元
		won = matchedBlue >= 1 || matchedRed >= 4 || (fyjActive && matchedRed == 3 && matchedBlue == 0)
	case TypeDLT:
		// 大乐透（2026新规7等奖级）：
		// 一等5+2 / 二等5+1 / 三等5+0或4+2 / 四等4+1或3+2 / 五等4+0或3+2（注:3+2在四等和五等都有，取四等）
		// 六等3+1或2+2 / 七等3+0或2+1或1+2或0+2
		won = (matchedRed >= 5) ||
			(matchedRed == 4 && matchedBlue >= 1) ||
			(matchedRed == 4 && matchedBlue == 0) ||
			(matchedRed == 3 && matchedBlue >= 2) ||
			(matchedRed == 3 && matchedBlue == 1) ||
			(matchedRed == 3 && matchedBlue == 0) ||
			(matchedRed == 2 && matchedBlue >= 1) ||
			(matchedRed == 1 && matchedBlue >= 2) ||
			(matchedRed == 0 && matchedBlue >= 2)
	default:
		won = matchedBlue >= 1
	}
	return
}

// ---------- 复式 / 胆拖 展开 ----------

// playConfig 返回每种彩种单式的最小号码数及复式允许的最大号码数
func playConfig(t string) (minRed, maxRed, minBlue, maxBlue int) {
	if t == TypeDLT {
		return 5, 12, 2, 12 // 大乐透：5前+2后，复式前≤12、后≤12
	}
	return 6, 20, 1, 16 // 双色球：6红+1蓝，复式红≤20、蓝≤16
}

// combinations 返回从 arr 中任取 k 个的所有组合
func combinations(arr []string, k int) [][]string {
	var res [][]string
	n := len(arr)
	if k < 0 || k > n {
		return res
	}
	idx := make([]int, k)
	var rec func(start, i int)
	rec = func(start, i int) {
		if i == k {
			comb := make([]string, k)
			for j := 0; j < k; j++ {
				comb[j] = arr[idx[j]]
			}
			res = append(res, comb)
			return
		}
		for j := start; j < n; j++ {
			idx[i] = j
			rec(j+1, i+1)
		}
	}
	rec(0, 0)
	return res
}

// diff 返回在 a 中但不在 b 中的元素（保持 a 的顺序）
func diff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	out := make([]string, 0, len(a))
	for _, x := range a {
		if _, ok := set[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

// subset 判断 a 的元素是否全部出现在 b 中
func subset(a, b []string) bool {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	for _, x := range a {
		if _, ok := set[x]; !ok {
			return false
		}
	}
	return true
}

// enumerateTicketCombos 把一张复式/胆拖彩票展开为所有可能的单式组合
// 返回 [][2][]string，每个元素为 {红球组合, 蓝球组合}
func enumerateTicketCombos(t, redBalls, blueBalls, bankerRed, bankerBlue string) [][2][]string {
	minRed, _, minBlue, _ := playConfig(t)
	red := splitNums(redBalls)
	blue := splitNums(blueBalls)
	bRed := splitNums(bankerRed)
	bBlue := splitNums(bankerBlue)

	var redCombos, blueCombos [][]string
	if len(bRed) > 0 {
		dragRed := diff(red, bRed)
		for _, c := range combinations(dragRed, minRed-len(bRed)) {
			redCombos = append(redCombos, append(append([]string{}, bRed...), c...))
		}
	} else {
		redCombos = combinations(red, minRed)
	}
	if len(bBlue) > 0 {
		dragBlue := diff(blue, bBlue)
		for _, c := range combinations(dragBlue, minBlue-len(bBlue)) {
			blueCombos = append(blueCombos, append(append([]string{}, bBlue...), c...))
		}
	} else {
		blueCombos = combinations(blue, minBlue)
	}

	out := make([][2][]string, 0, len(redCombos)*len(blueCombos))
	for _, rc := range redCombos {
		for _, bc := range blueCombos {
			out = append(out, [2][]string{rc, bc})
		}
	}
	return out
}

// ticketBets 计算一张彩票的注数（复式/胆拖按组合计数，不含倍数）
func ticketBets(t, redBalls, blueBalls, bankerRed, bankerBlue string) int {
	minRed, _, minBlue, _ := playConfig(t)
	red := splitNums(redBalls)
	blue := splitNums(blueBalls)
	bRed := splitNums(bankerRed)
	bBlue := splitNums(bankerBlue)
	rc, bc := 1, 1
	if len(bRed) > 0 {
		rc = len(combinations(diff(red, bRed), minRed-len(bRed)))
	} else {
		rc = len(combinations(red, minRed))
	}
	if len(bBlue) > 0 {
		bc = len(combinations(diff(blue, bBlue), minBlue-len(bBlue)))
	} else {
		bc = len(combinations(blue, minBlue))
	}
	return rc * bc
}

// validateTicket 按玩法校验并规范化号码，返回 (red, blue, ok)
// playType: single=单式 / compound=复式 / banker=胆拖
func validateTicket(t, playType, red, blue, bankerRed, bankerBlue string) (string, string, bool) {
	red = normalizeBalls(red)
	blue = normalizeBalls(blue)
	bankerRed = normalizeBalls(bankerRed)
	bankerBlue = normalizeBalls(bankerBlue)
	if t != TypeSSQ && t != TypeDLT {
		return red, blue, false
	}
	minRed, maxRed, minBlue, maxBlue := playConfig(t)
	rn := len(splitNums(red))
	bn := len(splitNums(blue))
	brn := len(splitNums(bankerRed))
	bbn := len(splitNums(bankerBlue))

	switch playType {
	case "compound":
		if rn < minRed || rn > maxRed {
			return red, blue, false
		}
		if bn < minBlue || bn > maxBlue {
			return red, blue, false
		}
		if rn == minRed && bn == minBlue {
			return red, blue, false // 与单式完全相同，应按单式录入
		}
		if brn != 0 || bbn != 0 {
			return red, blue, false // 复式不含胆码
		}
	case "banker":
		if !subset(splitNums(bankerRed), splitNums(red)) || !subset(splitNums(bankerBlue), splitNums(blue)) {
			return red, blue, false // 胆码必须是所选号码的子集
		}
		if brn > minRed-1 || bbn > minBlue-1 {
			return red, blue, false // 胆码数量超限
		}
		if len(diff(splitNums(red), splitNums(bankerRed))) < minRed-brn {
			return red, blue, false // 红球拖码不足
		}
		if len(diff(splitNums(blue), splitNums(bankerBlue))) < minBlue-bbn {
			return red, blue, false // 蓝球拖码不足
		}
		if rn > maxRed || bn > maxBlue {
			return red, blue, false
		}
		if brn == 0 && bbn == 0 {
			return red, blue, false // 胆拖至少一侧要有胆码
		}
	case "single", "":
		if rn != minRed || bn != minBlue {
			return red, blue, false
		}
	default:
		return red, blue, false
	}
	return red, blue, true
}

// validateBalls 兼容旧调用：按单式校验
func validateBalls(t, red, blue string) (string, string, bool) {
	return validateTicket(t, "single", red, blue, "", "")
}

// ---------- 日志 ----------

func logInfo(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}
func logErr(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// ---------- 奖金金额 ----------

// prizeMoney 根据彩种、奖级和奖池金额返回单注奖金描述。
// 一等奖和二等奖为浮动奖，返回"浮动"；其余为固定奖金。
// 大乐透在奖池≥8亿时，三至七等奖自动上浮（2026新规）。
func prizeMoney(lotType, tier string, poolAmount int64) string {
	switch TypeName(lotType) {
	case "双色球":
		switch tier {
		case "一等奖":
			return "浮动（最高500万）"
		case "二等奖":
			return "浮动"
		case "三等奖":
			return "3000元"
		case "四等奖":
			return "200元"
		case "五等奖":
			return "10元"
		case "六等奖":
			return "5元"
		case "福运奖":
			return "5元"
		}
	case "大乐透":
		boost := poolAmount >= 8_0000_0000 // 8亿元
		switch tier {
		case "一等奖":
			return "浮动（最高1000万）"
		case "二等奖":
			return "浮动"
		case "三等奖":
			if boost {
				return "6666元"
			}
			return "5000元"
		case "四等奖":
			if boost {
				return "380元"
			}
			return "300元"
		case "五等奖":
			if boost {
				return "200元"
			}
			return "150元"
		case "六等奖":
			if boost {
				return "18元"
			}
			return "15元"
		case "七等奖":
			if boost {
				return "7元"
			}
			return "5元"
		}
	}
	return ""
}

// poolAmountDesc 把奖池金额（元）格式化为可读字符串，如"6.29亿"
func poolAmountDesc(poolAmount int64) string {
	if poolAmount <= 0 {
		return ""
	}
	yi := float64(poolAmount) / 1_0000_0000
	if yi >= 1 {
		return fmt.Sprintf("%.2f亿", yi)
	}
	wan := float64(poolAmount) / 1_0000
	return fmt.Sprintf("%.0f万", wan)
}
