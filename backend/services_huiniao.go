package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------- 福彩双色球官方API ----------
// https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&pageNo=1&pageSize=30&systemType=PC
// 需带 User-Agent + Referer: https://www.cwl.gov.cn/

type cwlPrizeGrade struct {
	Type      string `json:"type"`
	TypeNum   string `json:"typenum"`
	TypeMoney string `json:"typemoney"`
}

type cwlDrawItem struct {
	Name        string          `json:"name"`
	Code        string          `json:"code"`       // 期号，如 "2026095"
	Date        string          `json:"date"`       // 如 "2026-08-18(二)"
	Red         string          `json:"red"`        // 如 "04,06,14,21,22,33"
	Blue        string          `json:"blue"`       // 如 "16"
	PoolMoney   string          `json:"poolmoney"`  // 奖池金额（元），如 "628888733"
	FyjCount    string          `json:"fyjCount"`   // 福运奖注数
	FyjMoney    string          `json:"fyjMoney"`   // 福运奖奖金
	PrizeGrades []cwlPrizeGrade `json:"prizegrades"`
}

type cwlResp struct {
	State   int           `json:"state"`
	Message string        `json:"message"`
	Result  []cwlDrawItem `json:"result"`
}

// ---------- 体彩大乐透官方API ----------
// https://webapi.sporttery.cn/gateway/lottery/getHistoryPageListV1.qry?gameNo=85&provinceId=0&pageSize=30&pageNo=1&isVerify=1
// 需带 User-Agent + Referer: https://www.sporttery.cn/

type sportPrizeLevel struct {
	PrizeLevel        string      `json:"prizeLevel"`
	StakeCount        string      `json:"stakeCount"`
	StakeAmountFormat string      `json:"stakeAmountFormat"`
	Sort              json.Number `json:"sort"`
}

type sportDrawItem struct {
	LotteryDrawNum    string            `json:"lotteryDrawNum"`
	LotteryDrawResult string            `json:"lotteryDrawResult"`
	LotteryDrawTime   string            `json:"lotteryDrawTime"`
	PoolBalance       string            `json:"poolBalance"`
	PoolBalanceAfter  string            `json:"poolBalanceAfterdraw"`
	PrizeLevelList    []sportPrizeLevel `json:"prizeLevelList"`
}

type sportResp struct {
	Success bool `json:"success"`
	Value   struct {
		List []sportDrawItem `json:"list"`
	} `json:"value"`
}

// ---------- 灰鸟API（备用，无奖池但海外可用） ----------
// http://api.huiniao.top/interface/home/lotteryHistory?type=ssq&page=1&limit=30
// type: ssq=双色球, dlt=大乐透
// 双色球: one~six=红球, seven=蓝球
// 大乐透: one~five=前区, six~seven=后区

type huiniaoItem struct {
	Code      string `json:"code"`       // 期号
	Day       string `json:"day"`        // 开奖日期 "2026-08-20"
	One       string `json:"one"`
	Two       string `json:"two"`
	Three     string `json:"three"`
	Four      string `json:"four"`
	Five      string `json:"five"`
	Six       string `json:"six"`
	Seven     string `json:"seven"`
	OpenTime  string `json:"open_time"`  // "2026-08-20 21:15:00"
}

type huiniaoResp struct {
	Code int    `json:"code"`
	Info string `json:"info"`
	Data struct {
		Last huiniaoItem   `json:"last"`
		List []huiniaoItem `json:"list"`
	} `json:"data"`
}

// ---------- 通用 HTTP 请求 ----------

func httpGetJSON(url, referer string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查是否为 JSON（避免 HTML/WAF 拦截页）
	if len(body) == 0 {
		return nil, fmt.Errorf("空响应")
	}
	first := body[0]
	if first != '{' && first != '[' {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("非JSON响应(首字符'%c'): %s", first, preview)
	}

	return body, nil
}

// ---------- 统一拉奖入口 ----------

// PullLottery 从官方API抓取双色球/大乐透最近开奖并入库，返回 anyNew=true 表示有新数据入库
func PullLottery() (bool, error) {
	types := []string{TypeSSQ, TypeDLT}
	var lastErr error
	anyNew := false
	for _, t := range types {
		isNew, err := pullOneType(t)
		if err != nil {
			logErr("拉取 %s 失败: %v", TypeName(t), err)
			lastErr = err
		} else if isNew {
			anyNew = true
			logInfo("拉取 %s 完成（有新数据）", TypeName(t))
		} else {
			logInfo("拉取 %s 完成（无新数据）", TypeName(t))
		}
	}
	return anyNew, lastErr
}

// pullOneType 拉取单种彩票的开奖结果
func pullOneType(t string) (bool, error) {
	switch t {
	case TypeSSQ:
		return pullSSQ()
	case TypeDLT:
		return pullDLT()
	default:
		return false, fmt.Errorf("未知彩种: %s", t)
	}
}

// pullSSQ 拉取双色球开奖
// 策略：优先官方API（有奖池），失败用灰鸟API（无奖池）
func pullSSQ() (bool, error) {
	// 方案1：福彩官方API（有奖池金额）
	url := "https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&pageNo=1&pageSize=30&systemType=PC"
	body, err := httpGetJSON(url, "https://www.cwl.gov.cn/")
	if err != nil {
		logErr("福彩官方API失败，切换灰鸟API: %v", err)
		return pullSSQHuiniao()
	}

	var cr cwlResp
	if err := json.Unmarshal(body, &cr); err != nil {
		logErr("解析福彩官方API失败，切换灰鸟API: %v", err)
		return pullSSQHuiniao()
	}
	if cr.State != 0 {
		logErr("福彩API返回 state=%d message=%s，切换灰鸟API", cr.State, cr.Message)
		return pullSSQHuiniao()
	}

	anyNew := false
	for _, it := range cr.Result {
		if it.Code == "" || it.Red == "" {
			continue
		}
		drawDate := it.Date
		if idx := strings.Index(drawDate, "("); idx > 0 {
			drawDate = drawDate[:idx]
		}
		poolAmount := parsePoolAmount(it.PoolMoney)
		fyjCount, fyjMoney := parseFyjInfo(it.FyjCount, it.FyjMoney)

		isNew, err := upsertDrawWithPool(TypeSSQ, it.Code, normalizeBalls(it.Red), normalizeBalls(it.Blue), drawDate, poolAmount, fyjCount, fyjMoney)
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeSSQ, it.Code, err)
		}
		if isNew {
			anyNew = true
		}
	}
	logInfo("福彩官方API成功（含奖池）")
	return anyNew, nil
}

// pullSSQHuiniao 灰鸟API拉取双色球（备用，无奖池）
func pullSSQHuiniao() (bool, error) {
	url := "http://api.huiniao.top/interface/home/lotteryHistory?type=ssq&page=1&limit=30"
	body, err := httpGetJSON(url, "")
	if err != nil {
		return false, fmt.Errorf("灰鸟API失败: %w", err)
	}

	var hr huiniaoResp
	if err := json.Unmarshal(body, &hr); err != nil {
		return false, fmt.Errorf("解析灰鸟API失败: %w", err)
	}
	if hr.Code != 1 || len(hr.Data.List) == 0 {
		return false, fmt.Errorf("灰鸟API返回 code=%d info=%s", hr.Code, hr.Info)
	}

	anyNew := false
	for _, it := range hr.Data.List {
		if it.Code == "" || it.One == "" {
			continue
		}
		// 双色球: one~six=红球, seven=蓝球
		red := strings.Join([]string{it.One, it.Two, it.Three, it.Four, it.Five, it.Six}, ",")
		blue := it.Seven
		// 日期取 day 字段（"2026-08-20"）
		drawDate := it.Day
		// 灰鸟API无奖池信息，poolAmount=0
		isNew, err := upsertDrawWithPool(TypeSSQ, it.Code, normalizeBalls(red), normalizeBalls(blue), drawDate, 0, "", "")
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeSSQ, it.Code, err)
		}
		if isNew {
			anyNew = true
		}
	}
	logInfo("灰鸟API成功（无奖池）")
	return anyNew, nil
}

// pullDLT 拉取大乐透开奖
// 策略：优先官方API（有奖池），失败用灰鸟API（无奖池）
func pullDLT() (bool, error) {
	// 方案1：体彩官方API（有奖池金额）
	url := "https://webapi.sporttery.cn/gateway/lottery/getHistoryPageListV1.qry?gameNo=85&provinceId=0&pageSize=30&pageNo=1&isVerify=1"
	body, err := httpGetJSON(url, "https://www.sporttery.cn/")
	if err != nil {
		logErr("体彩官方API失败，切换灰鸟API: %v", err)
		return pullDLTHuiniao()
	}

	var sr sportResp
	if err := json.Unmarshal(body, &sr); err != nil {
		logErr("解析体彩官方API失败，切换灰鸟API: %v", err)
		return pullDLTHuiniao()
	}
	if !sr.Success {
		logErr("体彩API返回 success=false，切换灰鸟API")
		return pullDLTHuiniao()
	}

	anyNew := false
	for _, it := range sr.Value.List {
		if it.LotteryDrawNum == "" || it.LotteryDrawResult == "" {
			continue
		}
		parts := strings.Fields(it.LotteryDrawResult)
		if len(parts) < 7 {
			logErr("大乐透期号 %s 开奖号码不足7个: %s", it.LotteryDrawNum, it.LotteryDrawResult)
			continue
		}
		red := strings.Join(parts[:5], ",")
		blue := strings.Join(parts[5:7], ",")
		poolAmount := parsePoolAmount(it.PoolBalanceAfter)

		isNew, err := upsertDrawWithPool(TypeDLT, it.LotteryDrawNum, normalizeBalls(red), normalizeBalls(blue), it.LotteryDrawTime, poolAmount, "", "")
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeDLT, it.LotteryDrawNum, err)
		}
		if isNew {
			anyNew = true
		}
	}
	logInfo("体彩官方API成功（含奖池）")
	return anyNew, nil
}

// pullDLTHuiniao 灰鸟API拉取大乐透（备用，无奖池）
func pullDLTHuiniao() (bool, error) {
	url := "http://api.huiniao.top/interface/home/lotteryHistory?type=dlt&page=1&limit=30"
	body, err := httpGetJSON(url, "")
	if err != nil {
		return false, fmt.Errorf("灰鸟API失败: %w", err)
	}

	var hr huiniaoResp
	if err := json.Unmarshal(body, &hr); err != nil {
		return false, fmt.Errorf("解析灰鸟API失败: %w", err)
	}
	if hr.Code != 1 || len(hr.Data.List) == 0 {
		return false, fmt.Errorf("灰鸟API返回 code=%d info=%s", hr.Code, hr.Info)
	}

	anyNew := false
	for _, it := range hr.Data.List {
		if it.Code == "" || it.One == "" {
			continue
		}
		// 大乐透: one~five=前区(红球), six~seven=后区(蓝球)
		red := strings.Join([]string{it.One, it.Two, it.Three, it.Four, it.Five}, ",")
		blue := strings.Join([]string{it.Six, it.Seven}, ",")
		drawDate := it.Day
		// 灰鸟API无奖池信息，poolAmount=0
		isNew, err := upsertDrawWithPool(TypeDLT, it.Code, normalizeBalls(red), normalizeBalls(blue), drawDate, 0, "", "")
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeDLT, it.Code, err)
		}
		if isNew {
			anyNew = true
		}
	}
	logInfo("灰鸟API成功（无奖池）")
	return anyNew, nil
}

// parsePoolAmount 把 "628888733" 或 "772,986,652.76" 解析成整数（元）
func parsePoolAmount(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if idx := strings.Index(s, "."); idx > 0 {
		s = s[:idx]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseFyjInfo 解析福运奖注数和奖金
func parseFyjInfo(fyjCount, fyjMoney string) (string, string) {
	fyjCount = strings.TrimSpace(fyjCount)
	fyjMoney = strings.TrimSpace(fyjMoney)
	if fyjCount == "" && fyjMoney == "" {
		return "", ""
	}
	return fyjCount, fyjMoney
}

// upsertDrawWithPool 插入或更新开奖记录（含奖池金额），返回 isNew=true 表示本次新增了数据
func upsertDrawWithPool(t, issue, red, blue, drawDate string, poolAmount int64, fyjCount, fyjMoney string) (isNew bool, err error) {
	res, err := DB.Exec(`INSERT INTO draw_results(type,issue,red_balls,blue_balls,draw_date,pool_amount,fyj_count,fyj_money)
		VALUES(?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE red_balls=VALUES(red_balls), blue_balls=VALUES(blue_balls), draw_date=VALUES(draw_date), pool_amount=VALUES(pool_amount), fyj_count=VALUES(fyj_count), fyj_money=VALUES(fyj_money)`,
		t, issue, red, blue, drawDate, poolAmount, fyjCount, fyjMoney)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// upsertDraw 兼容旧调用（不含奖池金额）
func upsertDraw(t, issue, red, blue, drawDate string) (isNew bool, err error) {
	return upsertDrawWithPool(t, issue, red, blue, drawDate, 0, "", "")
}

// GetLatestDraw 取某彩种最近一期开奖
func GetLatestDraw(t string) (DrawResult, bool) {
	var d DrawResult
	err := DB.QueryRow("SELECT id,type,issue,red_balls,blue_balls,draw_date,pool_amount,fyj_count,fyj_money,created_at FROM draw_results WHERE type=? ORDER BY id DESC LIMIT 1",
		t).Scan(&d.ID, &d.Type, &d.Issue, &d.RedBalls, &d.BlueBalls, &d.DrawDate, &d.PoolAmount, &d.FyjCount, &d.FyjMoney, &d.CreatedAt)
	return d, err == nil
}

// GetDrawByIssue 按彩种+期号取开奖结果
func GetDrawByIssue(t, issue string) (DrawResult, bool) {
	var d DrawResult
	err := DB.QueryRow("SELECT id,type,issue,red_balls,blue_balls,draw_date,pool_amount,fyj_count,fyj_money,created_at FROM draw_results WHERE type=? AND issue=?",
		t, issue).Scan(&d.ID, &d.Type, &d.Issue, &d.RedBalls, &d.BlueBalls, &d.DrawDate, &d.PoolAmount, &d.FyjCount, &d.FyjMoney, &d.CreatedAt)
	return d, err == nil
}
