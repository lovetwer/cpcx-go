package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// OCRResult 一张被识别出的彩票
type OCRResult struct {
	Type      string // ssq / dlt
	Issue     string
	RedBalls  string
	BlueBalls string
}

// RecognizeLottery 调用 Agnes AI（与 AI 预测相同的大模型服务）进行彩票截图识别。
// Agnes 是通用视觉语言模型，能直接理解图片内容并按 JSON 格式输出彩票号码，
// 无需像纯 OCR 模型那样做三轮温度重试 + 正则兜底。
//
// 复用 Cfg.AgnesAPIKey / Cfg.AgnesAPIURL / Cfg.AgnesModel 配置，
// 与 AI 预测接口共享同一套凭证。
func RecognizeLottery(imageBytes []byte, filename string) ([]OCRResult, error) {
	key := Cfg.AgnesAPIKey
	if key == "" {
		return nil, fmt.Errorf("未配置 AGNES_API_KEY（Agnes AI 的 API Key）")
	}
	url := Cfg.AgnesAPIURL
	if url == "" {
		url = "https://apihub.agnes-ai.com/v1/chat/completions"
	}
	model := Cfg.AgnesModel
	if model == "" {
		model = "agnes-2.5-flash"
	}

	// 图片编码为 data URL
	mime := detectMime(filename, imageBytes)
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imageBytes)

	// Agnes 是通用大模型，能用结构化 prompt 直接让它输出 JSON
	systemMsg := "你是一个中国彩票识别助手。你会收到一张彩票截图，需要识别出其中的投注号码并以严格的JSON格式返回。"

	promptText := `请识别这张彩票图片中的投注号码，以JSON数组格式返回（不要包含任何解释文字、不要用代码围栏）。

识别规则：
1. 彩种：如果票面有"双色球"字样，type为"ssq"；有"大乐透"字样，type为"dlt"
2. 期号：形如"第2026093期"中的数字部分，如无法识别则留空字符串""
3. 红球/前区号码：双色球是6个(01-33)，大乐透是5个(01-35)，用两位数逗号分隔
4. 蓝球/后区号码：双色球是1个(01-16)，大乐透是2个(01-12)，用两位数逗号分隔
5. 一张票可能有多注，每注输出一个数组元素
6. 忽略条形码、金额、日期、说明文字等非投注号码信息

输出格式示例：
[{"type":"ssq","issue":"2026093","red_balls":"03,07,15,20,21,24","blue_balls":"09"}]
或
[{"type":"dlt","issue":"2026092","red_balls":"03,07,15,20,21","blue_balls":"01,09"}]

请只输出JSON数组，不要输出其他任何内容。`

	client := &http.Client{Timeout: 90 * time.Second}

	payload := map[string]interface{}{
		"model":       model,
		"temperature": 0.1, // 低温保证精确识别
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": systemMsg,
			},
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
					{"type": "text", "text": promptText},
				},
			},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Agnes AI 请求失败: %w", err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Agnes AI 返回 %d: %s", resp.StatusCode, string(raw))
	}

	// 解析 OpenAI 兼容返回格式
	var aiResp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &aiResp); err != nil {
		return nil, fmt.Errorf("Agnes AI 返回解析失败: %w (原始: %s)", err, string(raw))
	}
	if aiResp.Error != nil && aiResp.Error.Message != "" {
		return nil, fmt.Errorf("Agnes AI 报错: %s", aiResp.Error.Message)
	}
	if len(aiResp.Choices) == 0 {
		return nil, fmt.Errorf("Agnes AI 未返回内容: %s", string(raw))
	}

	content := aiResp.Choices[0].Message.Content
	if content == "" {
		content = aiResp.Choices[0].Message.ReasoningContent
	}
	if content == "" {
		return nil, fmt.Errorf("Agnes AI 返回内容为空")
	}

	// 尝试解析模型返回的 JSON
	results, err := parseOCRText(content)
	if err == nil {
		// 从模型原始文本里补抓期号（以防 JSON 中期号为空但文本中有）
		if issue := extractIssueFromText(content); issue != "" {
			for i := range results {
				if results[i].Issue == "" {
					results[i].Issue = issue
				}
			}
		}
		logInfo("Agnes AI 识别成功，解析出 %d 注", len(results))
		return results, nil
	}

	// 兜底：把模型原始返回落盘，便于排查
	debugContent := "=== " + time.Now().Format("2006-01-02 15:04:05") + " ===\n" + content + "\n"
	_ = os.WriteFile("ocr_debug.log", []byte(debugContent), 0644)
	return nil, err
}

// extractIssueFromText 从模型原始转写文本里抠期号：匹配“第2026093期”或“期号2024090”
// （也可能是一串日期如 2026-08-13，这里只抓标准期号写法，避免误把条形码当期号）。
func extractIssueFromText(s string) string {
	if m := reIssueA.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if m := reIssueB.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// parseOCRText 从模型回复文本中抽取并解析彩票 JSON。
// 主路径：去掉可能的 ```json 代码围栏后，提取 JSON 数组 / 对象。
// 兜底：若模型没给 JSON，尝试用正则从自然文本里抠出 期号/红球/蓝球。
func parseOCRText(content string) ([]OCRResult, error) {
	content = strings.TrimSpace(content)
	content = stripCodeFence(content)

	// 1) 优先解析 JSON 数组
	if arr := extractJSONArray(content); arr != nil {
		var results []OCRResult
		for _, item := range arr {
			if r, ok := toOCRResult(item); ok {
				results = append(results, r)
			}
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// 2) 尝试单个 JSON 对象
	if obj := extractJSONObject(content); obj != nil {
		if r, ok := toOCRResult(obj); ok {
			return []OCRResult{r}, nil
		}
	}

	// 3) 兜底：正则从文本里抠（支持一张票多注）
	if results, ok := regexExtractAll(content); ok {
		return results, nil
	}

	// 全部解析失败时给出可读报错：若是模型只返回重复的机器码，单独提示；
	// 若是只读到页眉/页脚说明文字（读错区域），也单独提示；
	// 否则截断过长的原始返回，避免把上千字符的机械重复塞进响应。
	if msg, degenerate := degenerateMessage(content); degenerate {
		return nil, fmt.Errorf("%s\n原始返回: %s", msg, sanitizeRaw(content))
	}
	if msg, boiler := boilerplateMessage(content); boiler {
		return nil, fmt.Errorf("%s\n原始返回: %s", msg, sanitizeRaw(content))
	}
	return nil, fmt.Errorf("未能从 OCR 返回中解析出彩票信息，原始返回: %s", sanitizeRaw(content))
}

// isAllDigits 判断字符串是否全为数字（允许为空串返回 false）。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// countNumericTokens 统计像 “1.” “2” “003” 这类纯数字 token（忽略末尾的点号）的数量。
func countNumericTokens(words []string) int {
	n := 0
	for _, w := range words {
		if isAllDigits(strings.TrimRight(w, ".")) {
			n++
		}
	}
	return n
}

// isDegenerate 判断模型返回是否为“无有效票面号码”的退化输出。覆盖两类：
//  1. 少量短语重复很多次（如 “2024080-13 17:01:28” ×N）——uniq 数很少；
//  2. 长串连续数字枚举（如 “1. 2. 3. ... 999.”）——表面唯一词很多，但绝大多数是数字 token。
func isDegenerate(s string) bool {
	words := strings.Fields(s)
	if len(words) < 20 {
		return false
	}
	// 1) 少量短语重复
	uniq := map[string]int{}
	for _, w := range words {
		uniq[w]++
	}
	if len(uniq) <= 5 {
		return true
	}
	// 2) 长串数字枚举：数字 token 占绝大多数且数量很大
	numTokens := countNumericTokens(words)
	return numTokens >= 15 && float64(numTokens) >= 0.6*float64(len(words))
}

// degenerateMessage 检测模型返回是否为无有效票面号码的退化输出。
// 命中返回 (可读提示, true)。区分“机器码重复”和“长串数字枚举”两类。
func degenerateMessage(s string) (string, bool) {
	if !isDegenerate(s) {
		return "", false
	}
	words := strings.Fields(s)
	// 数字枚举型（1. 2. 3. ... 999.）：绝大多数 token 是数字
	if countNumericTokens(words) >= 15 {
		return "模型未识别到有效的彩票票面号码（疑似把版面读成了一长串编号/序号，而非投注号码球）。请换一张号码区清晰的截图，或手动录入", true
	}
	// 少量短语重复型（机器码）
	uniq := map[string]int{}
	for _, w := range words {
		uniq[w]++
	}
	top := ""
	maxc := 0
	for w, c := range uniq {
		if c > maxc {
			maxc = c
			top = w
		}
	}
	return fmt.Sprintf("模型未识别到可读的彩票号码（疑似只读到机器码，如“%s”，重复 %d 次）。请换一张号码区清晰的截图，或手动录入", top, len(words)), true
}

// sanitizeRaw 截断过长的模型原始返回，避免把上千字符塞进报错信息。
func sanitizeRaw(s string) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= 200 {
		return string(runes)
	}
	return string(runes[:200]) + " …（已截断）"
}

// boilerplateMarkers 是彩票票面页眉/页脚常见的“说明文字”关键词（非投注号码）。
var boilerplateMarkers = []string{
	"感谢您为公益事业", "奖池", "福利彩票发行中心", "承销", "公益",
	"贡献", "亿元", "地址：", "地址:", "中国福利彩票", "体育彩票",
}

// looksLikeTicketBoilerplate 判断模型是否只读到页眉/页脚的说明文字而漏掉了投注号码。
// 命中条件：抽不到有效号码，且出现多个说明性关键词。
func looksLikeTicketBoilerplate(s string) bool {
	// 已经能从文本里抽出号码，说明读到了有效内容，不算读错区域
	if _, ok := regexExtract(s); ok {
		return false
	}
	hit := 0
	for _, m := range boilerplateMarkers {
		if strings.Contains(s, m) {
			hit++
		}
	}
	return hit >= 2
}

// boilerplateMessage 检测模型是否只读了页眉/页脚说明文字（读错区域）。
// 命中返回 (可读提示, true)。
func boilerplateMessage(s string) (string, bool) {
	if !looksLikeTicketBoilerplate(s) {
		return "", false
	}
	return "模型读到了彩票，但只识别到页眉/页脚的说明文字（如“感谢您为公益事业贡献…元”“奖池余额…亿元”“…福利彩票发行中心承销”），没有找到投注号码球。请换一张号码区清晰的截图，或手动录入", true
}

// toOCRResult 把 map 规整成 OCRResult（含彩种映射、号码归一化）
func toOCRResult(m map[string]interface{}) (OCRResult, bool) {
	r := OCRResult{
		Type:      mapType(pickString(m, "type", "lottery_type", "彩票类型", "彩种")),
		Issue:     pickString(m, "issue", "code", "qiHao", "期号", "qihao"),
		RedBalls:  pickString(m, "red", "red_balls", "front", "qianqu", "红球", "前区"),
		BlueBalls: pickString(m, "blue", "blue_balls", "back", "houqu", "蓝球", "后区"),
	}
	if r.Type == "" {
		r.Type = TypeSSQ
	}
	r.RedBalls = normalizeBalls(r.RedBalls)
	r.BlueBalls = normalizeBalls(r.BlueBalls)
	// 期号允许为空（很多票面模型读不到期号），空期号在保存环节回退到最近一期，
	// 这里不要因空期号直接丢弃整条记录，否则号码明明识别对了却存不进去。
	return r, true
}

func mapType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ssq", "双色球", "双色球(ssq)", "shuangseqiu":
		return TypeSSQ
	case "dlt", "大乐透", "daletou":
		return TypeDLT
	default:
		// 命中“大乐透”字样才算 dlt
		if strings.Contains(s, "大乐透") {
			return TypeDLT
		}
		return ""
	}
}

func pickString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return strings.TrimSpace(t)
			case float64:
				return fmt.Sprintf("%v", int64(t))
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

// ---------------- JSON / 文本工具 ----------------

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// 去掉首行 ```json 之类的围栏
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

func extractJSONArray(s string) []map[string]interface{} {
	start := strings.Index(s, "[")
	if start < 0 {
		return nil
	}
	end := strings.LastIndex(s, "]")
	if end <= start {
		return nil
	}
	sub := s[start : end+1]
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(sub), &arr); err != nil {
		return nil
	}
	return arr
}

func extractJSONObject(s string) map[string]interface{} {
	start := strings.Index(s, "{")
	if start < 0 {
		return nil
	}
	end := strings.LastIndex(s, "}")
	if end <= start {
		return nil
	}
	sub := s[start : end+1]
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(sub), &m); err != nil {
		return nil
	}
	return m
}

var (
	reIssueA = regexp.MustCompile(`第\s*([0-9]{6,8})\s*期`)       // 第2026093期
	reIssueB = regexp.MustCompile(`期号[号:：]?\s*([0-9]{4,8})`) // 期号2024080
	reType   = regexp.MustCompile(`(双色球|大乐透)`)
	reBall   = regexp.MustCompile(`\b([0-9]{1,2})\b`)                       // 1~2 位数字（连续长数字串不会整体命中）
	reBracket = regexp.MustCompile(`\[[^\]]*\]`)                            // 去掉 [1倍] 等括号内容
	reMult    = regexp.MustCompile(`\d*倍`)                                 // 去掉 1倍/2倍（含前面的倍数数字）
	reCircle  = regexp.MustCompile(`[⓪①②③④⑤⑥⑦⑧⑨⑩]`)                 // 去掉 ① ② … 投注序号
)

// footerBoilerplate 需要剔除的页眉/页脚说明文字关键词（只放绝不会出现在投注行里的强标记）
var footerBoilerplate = []string{
	"感谢您为公益事业", "奖池", "福利彩票发行中心承销", "承销", "公益",
	"贡献", "亿元", "地址", "中国福利彩票", "体育彩票", "销售",
	"开奖日期", "打印", "条形码", "条码", "元",
}

// cleanForExtract 去掉页脚说明行并合并连续重复行（模型常把“承销中心”刷几百遍）
func cleanForExtract(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	prev := ""
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		for _, b := range footerBoilerplate {
			if strings.Contains(t, b) {
				t = "" // 命中说明文字，整行丢弃
				break
			}
		}
		if t == "" {
			continue
		}
		if t == prev { // 合并连续重复行
			continue
		}
		out = append(out, t)
		prev = t
	}
	return strings.Join(out, "\n")
}

// validBalls 从文本提取 1..35 的数字球（忽略 >35 的，如条形码 43、日期片段）
// 会跳过：年份(2024/2026)、日期(2024-08-16)、金额(1.44 5.6)等噪声
var reDateOrYear = regexp.MustCompile(`\b(19|20)\d{2}\b`)   // 年份 2024
var reDateFull = regexp.MustCompile(`\d{4}[-/]\d{1,2}[-/]\d{1,2}`) // 2024-08-16
var reMoney = regexp.MustCompile(`\d+\.\d+`)                     // 1.44 5.6

func validBalls(s string) []string {
	// 先移除年份、日期、金额等噪声
	s = reDateFull.ReplaceAllString(s, " ")
	s = reDateOrYear.ReplaceAllString(s, " ")
	s = reMoney.ReplaceAllString(s, " ")
	matches := reBall.FindAllStringSubmatch(s, -1)
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err == nil && n >= 1 && n <= 35 {
			b := fmt.Sprintf("%02d", n)
			if !seen[b] {
				out = append(out, b)
				seen[b] = true
			}
		}
	}
	return out
}

// lineBalls 判断一行是否像投注号码行，并切分红球/蓝球。
// 支持多种分隔方式：
//   - "03 07 29 30 32 33-13" （短横分隔）
//   - "03 07 29 30 32 33 + 13" （加号分隔）
//   - "红球 03 07 29 30 32 33 蓝球 13" （中文标记分隔）
//   - "03 07 29 30 32 33 13" （无分隔，按数量切分）
func lineBalls(t string) (red, blue []string, ok bool) {
	// 先剥掉投注行装饰：① 序号、[1倍] 倍注、括号内容
	t = reCircle.ReplaceAllString(t, " ")
	t = reBracket.ReplaceAllString(t, " ")
	t = reMult.ReplaceAllString(t, " ")

	// 尝试用多种分隔符切分红蓝
	splitters := []string{"-", "+", "蓝球", "后区", "蓝:"}
	var leftStr, rightStr string
	found := false
	for _, sp := range splitters {
		idx := strings.Index(t, sp)
		if idx >= 0 {
			leftStr = strings.TrimSpace(t[:idx])
			rightStr = strings.TrimSpace(t[idx+len(sp):])
			found = true
			break
		}
	}

	if found {
		// 移除leftStr中的“红球”/“前区”等中文标记
		leftStr = strings.ReplaceAll(leftStr, "红球", " ")
		leftStr = strings.ReplaceAll(leftStr, "前区", " ")
		leftStr = strings.ReplaceAll(leftStr, "红:", " ")
		leftNums := validBalls(leftStr)
		rightNums := validBalls(rightStr)
		if len(leftNums) >= 5 && len(rightNums) >= 1 && len(rightNums) <= 2 {
			return leftNums, rightNums, true
		}
	}

	// 无明确分隔：用全行提取的数字按数量切分
	all := validBalls(t)
	switch len(all) {
	case 7:
		return all[:6], all[6:], true
	case 6:
		return all[:5], all[5:], true
	}
	return nil, nil, false
}

// regexExtractAll 兜底：从自然转写文本里抠出所有投注行（同一张票可能有多注）
func regexExtractAll(s string) ([]OCRResult, bool) {
	clean := cleanForExtract(s)

	// 期号（同一张票共用）
	issue := ""
	if m := reIssueA.FindStringSubmatch(clean); m != nil {
		issue = m[1]
	} else if m := reIssueB.FindStringSubmatch(clean); m != nil {
		issue = m[1]
	}
	// 全局默认彩种（同一张票多数情况只一种）
	globalType := TypeSSQ
	if m := reType.FindStringSubmatch(clean); m != nil {
		globalType = mapType(m[1])
	}

	var results []OCRResult
	for _, ln := range strings.Split(clean, "\n") {
		red, blue, ok := lineBalls(ln)
		if !ok {
			continue
		}
		// 逐行判断彩种：如果当前行包含"大乐透"或"前区/后区"，就用对应类型；
		// 否则用全局默认
		lineType := globalType
		if m := reType.FindStringSubmatch(ln); m != nil {
			lineType = mapType(m[1])
		} else if strings.Contains(ln, "前区") || strings.Contains(ln, "后区") {
			lineType = TypeDLT
		} else if strings.Contains(ln, "红球") || strings.Contains(ln, "蓝球") {
			lineType = TypeSSQ
		}
		// 逐行提取期号（不同行可能期号不同）
		lineIssue := issue
		if m := reIssueA.FindStringSubmatch(ln); m != nil {
			lineIssue = m[1]
		} else if m := reIssueB.FindStringSubmatch(ln); m != nil {
			lineIssue = m[1]
		}
		results = append(results, OCRResult{
			Type:      lineType,
			Issue:     lineIssue,
			RedBalls:  strings.Join(red, ","),
			BlueBalls: strings.Join(blue, ","),
		})
	}
	if len(results) == 0 {
		return nil, false
	}
	return results, true
}

// regexExtract 单注兼容入口（取首注），供读错区域检测等使用
func regexExtract(s string) (OCRResult, bool) {
	all, ok := regexExtractAll(s)
	if !ok || len(all) == 0 {
		return OCRResult{}, false
	}
	return all[0], true
}

func detectMime(filename string, b []byte) string {
	if len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return "image/png"
	}
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return "image/jpeg"
	}
	if len(b) >= 3 && b[0] == 'G' && b[1] == 'I' && b[2] == 'F' {
		return "image/gif"
	}
	if strings.Contains(strings.ToLower(filename), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}
