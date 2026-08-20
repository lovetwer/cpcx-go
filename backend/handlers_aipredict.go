package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// handleAIPredict AI 预测接口（需登录）：获取近30期开奖记录，调用 Agnes AI 预测
func handleAIPredict(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := readJSON(r, &req); err != nil || req.Type == "" {
		req.Type = "ssq"
	}

	if Cfg.AgnesAPIKey == "" {
		writeError(w, http.StatusInternalServerError, "服务器未配置 AI Key")
		return
	}

	// 1. 获取近30期开奖记录
	rows, err := DB.Query("SELECT issue, red_balls, blue_balls, draw_date FROM draw_results WHERE type=? ORDER BY issue DESC LIMIT 30", req.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询开奖记录失败")
		return
	}
	defer rows.Close()

	type DrawItem struct {
		Issue     string `json:"issue"`
		RedBalls  string `json:"red_balls"`
		BlueBalls string `json:"blue_balls"`
		DrawDate  string `json:"draw_date"`
	}
	var draws []DrawItem
	for rows.Next() {
		var d DrawItem
		if err := rows.Scan(&d.Issue, &d.RedBalls, &d.BlueBalls, &d.DrawDate); err != nil {
			continue
		}
		draws = append(draws, d)
	}
	sort.Slice(draws, func(i, j int) bool {
		return draws[i].Issue > draws[j].Issue
	})

	if len(draws) == 0 {
		writeError(w, http.StatusBadRequest, "暂无开奖记录，无法预测")
		return
	}

	// 2. 构建历史文本
	isDlt := req.Type == "dlt"
	redLabel := "红球"
	blueLabel := "蓝球"
	if isDlt {
		redLabel = "前区"
		blueLabel = "后区"
	}
	var lines []string
	for _, d := range draws {
		lines = append(lines, fmt.Sprintf("第%s期(%s): %s%s %s%s", d.Issue, d.DrawDate, redLabel, d.RedBalls, blueLabel, d.BlueBalls))
	}
	historyText := strings.Join(lines, "\n")

	// 3. 构建提示词
	redCount := 6
	blueCount := 1
	redRange := "01-33"
	blueRange := "01-16"
	if isDlt {
		redCount = 5
		blueCount = 2
		redRange = "01-35"
		blueRange = "01-12"
	}

	gameName := "双色球"
	if isDlt {
		gameName = "大乐透"
	}

	last := draws[0]
	lastDrawText := fmt.Sprintf("上期：%s%s %s%s", redLabel, last.RedBalls, blueLabel, last.BlueBalls)

	system := "你是一个专业的彩票号码预测助手。你的任务是根据历史开奖数据直接输出预测号码。不要进行长篇分析，直接给出结果。必须输出纯JSON格式。"

	userPrompt := fmt.Sprintf(`根据近30期%s开奖记录：
%s

%s

请直接预测下一期号码，要求：
- %s%d个（范围%s），%s%d个（范围%s）
- 考虑冷热号、区间分布、遗漏值、奇偶平衡
- 直接输出JSON，不要任何解释文字

格式：{"red":["01","02",...],"blue":["01",...],"reason":"一句话简要说明选号思路"}`,
		gameName, historyText, lastDrawText,
		redLabel, redCount, redRange, blueLabel, blueCount, blueRange)

	// 4. 调用 Agnes AI
	aiReq := map[string]interface{}{
		"model": Cfg.AgnesModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.7,
		"max_tokens":   2048,
	}
	aiBody, _ := json.Marshal(aiReq)

	httpReq, err := http.NewRequest("POST", Cfg.AgnesAPIURL, bytes.NewReader(aiBody))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建 AI 请求失败")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+Cfg.AgnesAPIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI 服务请求失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		writeError(w, http.StatusBadGateway, "AI 服务返回错误: "+string(body))
		return
	}

	// 5. 解析 AI 返回
	var aiResp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &aiResp); err != nil {
		writeError(w, http.StatusInternalServerError, "解析 AI 响应失败")
		return
	}

	content := ""
	if len(aiResp.Choices) > 0 {
		content = aiResp.Choices[0].Message.Content
		if content == "" {
			content = aiResp.Choices[0].Message.ReasoningContent
		}
	}
	if content == "" {
		writeError(w, http.StatusInternalServerError, "AI 返回内容为空")
		return
	}

	// 6. 从内容中提取 JSON
	jsonStr := extractJSONFromContent(content)
	var parsed struct {
		Red    []string `json:"red"`
		Blue   []string `json:"blue"`
		Reason string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		writeError(w, http.StatusInternalServerError, "AI 返回格式无法解析: "+jsonStr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"red":   parsed.Red,
		"blue":  parsed.Blue,
		"reason": parsed.Reason,
	})
}

// extractJSONFromContent 从文本中提取 JSON 对象
func extractJSONFromContent(content string) string {
	s := strings.TrimSpace(content)
	// 尝试提取代码块
	if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if strings.HasPrefix(s, "json") {
			s = s[4:]
		}
		if idx2 := strings.Index(s, "```"); idx2 >= 0 {
			s = s[:idx2]
		}
		s = strings.TrimSpace(s)
	}
	// 提取花括号内容
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
