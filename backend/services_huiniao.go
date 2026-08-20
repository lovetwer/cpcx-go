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
	Name        string            `json:"name"`
	Code        string            `json:"code"`       // 期号，如 "2026095"
	Date        string            `json:"date"`       // 如 "2026-08-18(二)"
	Red         string            `json:"red"`        // 如 "04,06,14,21,22,33"
	Blue        string            `json:"blue"`       // 如 "16"
	PoolMoney   string            `json:"poolmoney"`  // 奖池金额（元），如 "628888733"
	FyjCount    string            `json:"fyjCount"`   // 福运奖注数
	FyjMoney    string            `json:"fyjMoney"`   // 福运奖奖金
	PrizeGrades []cwlPrizeGrade   `json:"prizegrades"`
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
	PrizeLevel        string      `json:"prizeLevel"`         // 如 "一等奖"
	StakeCount        string      `json:"stakeCount"`         // 中奖注数
	StakeAmountFormat string      `json:"stakeAmountFormat"`  // 单注奖金（元），如 "8380751"
	Sort              json.Number `json:"sort"`               // 排序号
}

type sportDrawItem struct {
	LotteryDrawNum    string            `json:"lotteryDrawNum"`    // 期号，如 "26094"
	LotteryDrawResult string            `json:"lotteryDrawResult"` // 如 "05 14 15 17 33 01 07"
	LotteryDrawTime   string            `json:"lotteryDrawTime"`   // 如 "2026-08-19"
	PoolBalance       string            `json:"poolBalance"`       // 奖池金额，如 "772,986,652.76"
	PoolBalanceAfter  string            `json:"poolBalanceAfterdraw"` // 开奖后奖池
	PrizeLevelList    []sportPrizeLevel `json:"prizeLevelList"`
}

type sportResp struct {
	Success bool `json:"success"`
	Value   struct {
		List []sportDrawItem `json:"list"`
	} `json:"value"`
}

// ---------- 备用API：中彩网（第三方聚合） ----------
// https://www.mxnzd.cn/api/lottery?type=ssq
// 返回格式：{"code":1,"msg":"success","data":[{"issue":"2026095","date":"2026-08-18","red":"04,06,14,21,22,33","blue":"16","pool":"628888733"}]}

type mxnzdItem struct {
	Issue string `json:"issue"`
	Date  string `json:"date"`
	Red   string `json:"red"`
	Blue  string `json:"blue"`
	Pool  string `json:"pool"`
}

type mxnzdResp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data []mxnzdItem `json:"data"`
}

// ---------- 备用API：api.jisuapi.com（极速API） ----------
// 另一个备用方案

// ---------- 通用 HTTP 请求 ----------

func httpGetJSON(url, referer string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// 完整浏览器 UA，避免被 WAF 拦截
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
	bodyStr := string(body)
	if len(bodyStr) == 0 {
		return nil, fmt.Errorf("空响应")
	}
	first := bodyStr[0]
	if first != '{' && first != '[' {
		// 不是 JSON，可能是 HTML 拦截页
		preview := bodyStr
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

// pullOneType 从官方API拉取单种彩票的开奖结果
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

// pullSSQ 从福彩官方API拉取双色球开奖，失败则尝试备用API
func pullSSQ() (bool, error) {
	// 方案1：福彩官方API
	url := "https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=ssq&pageNo=1&pageSize=30&systemType=PC"
	body, err := httpGetJSON(url, "https://www.cwl.gov.cn/")
	if err != nil {
		logErr("福彩官方API失败，尝试备用源: %v", err)
		// 方案2：备用API
		return pullSSQBackup()
	}

	var cr cwlResp
	if err := json.Unmarshal(body, &cr); err != nil {
		logErr("解析福彩官方API失败，尝试备用源: %v", err)
		return pullSSQBackup()
	}
	if cr.State != 0 {
		logErr("福彩API返回 state=%d message=%s，尝试备用源", cr.State, cr.Message)
		return pullSSQBackup()
	}

	anyNew := false
	for _, it := range cr.Result {
		if it.Code == "" || it.Red == "" {
			continue
		}
		// 提取日期部分：从 "2026-08-18(二)" 中取 "2026-08-18"
		drawDate := it.Date
		if idx := strings.Index(drawDate, "("); idx > 0 {
			drawDate = drawDate[:idx]
		}
		// 解析奖池金额（单位：元，字符串数字）
		poolAmount := parsePoolAmount(it.PoolMoney)
		// 解析福运奖信息
		fyjCount, fyjMoney := parseFyjInfo(it.FyjCount, it.FyjMoney)

		isNew, err := upsertDrawWithPool(TypeSSQ, it.Code, normalizeBalls(it.Red), normalizeBalls(it.Blue), drawDate, poolAmount, fyjCount, fyjMoney)
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeSSQ, it.Code, err)
		}
		if isNew {
			anyNew = true
		}
	}
	return anyNew, nil
}

// pullSSQBackup 备用API拉取双色球（用于官方API被WAF拦截时）
func pullSSQBackup() (bool, error) {
	// 备用API：中彩网聚合API
	urls := []string{
		"https://www.mxnzd.cn/api/lottery?type=ssq&limit=30",
		"https://api.apiopen.top/api/lottery/ssq?size=30",
	}

	for _, url := range urls {
		body, err := httpGetJSON(url, "")
		if err != nil {
			logErr("备用API %s 失败: %v", url, err)
			continue
		}

		// 尝试解析 mxnzd 格式
		var mr mxnzdResp
		if err := json.Unmarshal(body, &mr); err == nil && mr.Code == 1 && len(mr.Data) > 0 {
			logInfo("备用API(%s) 成功获取 %d 条双色球记录", url, len(mr.Data))
			anyNew := false
			for _, it := range mr.Data {
				if it.Issue == "" || it.Red == "" {
					continue
				}
				poolAmount := parsePoolAmount(it.Pool)
				isNew, err := upsertDrawWithPool(TypeSSQ, it.Issue, normalizeBalls(it.Red), normalizeBalls(it.Blue), it.Date, poolAmount, "", "")
				if err != nil {
					logErr("写入 %s 期号 %s 失败: %v", TypeSSQ, it.Issue, err)
				}
				if isNew {
					anyNew = true
				}
			}
			return anyNew, nil
		}

		// 尝试解析 apiopen 格式
		var ar struct {
			Code int `json:"code"`
			Data struct {
				List []struct {
					Issue  string `json:"term"`
					Date   string `json:"date"`
					Red    string `json:"red"`
					Blue   string `json:"blue"`
					Pool   string `json:"pool"`
				} `json:"list"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &ar); err == nil && ar.Code == 200 && len(ar.Data.List) > 0 {
			logInfo("备用API(%s) 成功获取 %d 条双色球记录", url, len(ar.Data.List))
			anyNew := false
			for _, it := range ar.Data.List {
				if it.Issue == "" || it.Red == "" {
					continue
				}
				poolAmount := parsePoolAmount(it.Pool)
				isNew, err := upsertDrawWithPool(TypeSSQ, it.Issue, normalizeBalls(it.Red), normalizeBalls(it.Blue), it.Date, poolAmount, "", "")
				if err != nil {
					logErr("写入 %s 期号 %s 失败: %v", TypeSSQ, it.Issue, err)
				}
				if isNew {
					anyNew = true
				}
			}
			return anyNew, nil
		}
	}

	return false, fmt.Errorf("所有备用API均失败")
}

// pullDLT 从体彩官方API拉取大乐透开奖，失败则尝试备用API
func pullDLT() (bool, error) {
	// 方案1：体彩官方API
	url := "https://webapi.sporttery.cn/gateway/lottery/getHistoryPageListV1.qry?gameNo=85&provinceId=0&pageSize=30&pageNo=1&isVerify=1"
	body, err := httpGetJSON(url, "https://www.sporttery.cn/")
	if err != nil {
		logErr("体彩官方API失败，尝试备用源: %v", err)
		return pullDLTBackup()
	}

	var sr sportResp
	if err := json.Unmarshal(body, &sr); err != nil {
		logErr("解析体彩官方API失败，尝试备用源: %v", err)
		return pullDLTBackup()
	}
	if !sr.Success {
		logErr("体彩API返回 success=false，尝试备用源")
		return pullDLTBackup()
	}

	anyNew := false
	for _, it := range sr.Value.List {
		if it.LotteryDrawNum == "" || it.LotteryDrawResult == "" {
			continue
		}
		// lotteryDrawResult 格式："05 14 15 17 33 01 07"
		// 前5个是前区（红球），后2个是后区（蓝球）
		parts := strings.Fields(it.LotteryDrawResult)
		if len(parts) < 7 {
			logErr("大乐透期号 %s 开奖号码不足7个: %s", it.LotteryDrawNum, it.LotteryDrawResult)
			continue
		}
		red := strings.Join(parts[:5], ",")
		blue := strings.Join(parts[5:7], ",")
		// 解析奖池金额（带逗号和小数，如 "772,986,652.76"）
		poolAmount := parsePoolAmount(it.PoolBalanceAfter)

		isNew, err := upsertDrawWithPool(TypeDLT, it.LotteryDrawNum, normalizeBalls(red), normalizeBalls(blue), it.LotteryDrawTime, poolAmount, "", "")
		if err != nil {
			logErr("写入 %s 期号 %s 失败: %v", TypeDLT, it.LotteryDrawNum, err)
		}
		if isNew {
			anyNew = true
		}
	}
	return anyNew, nil
}

// pullDLTBackup 备用API拉取大乐透（用于官方API被WAF拦截时）
func pullDLTBackup() (bool, error) {
	urls := []string{
		"https://www.mxnzd.cn/api/lottery?type=dlt&limit=30",
		"https://api.apiopen.top/api/lottery/dlt?size=30",
	}

	for _, url := range urls {
		body, err := httpGetJSON(url, "")
		if err != nil {
			logErr("备用API %s 失败: %v", url, err)
			continue
		}

		// 尝试解析 mxnzd 格式
		var mr mxnzdResp
		if err := json.Unmarshal(body, &mr); err == nil && mr.Code == 1 && len(mr.Data) > 0 {
			logInfo("备用API(%s) 成功获取 %d 条大乐透记录", url, len(mr.Data))
			anyNew := false
			for _, it := range mr.Data {
				if it.Issue == "" || it.Red == "" {
					continue
				}
				poolAmount := parsePoolAmount(it.Pool)
				isNew, err := upsertDrawWithPool(TypeDLT, it.Issue, normalizeBalls(it.Red), normalizeBalls(it.Blue), it.Date, poolAmount, "", "")
				if err != nil {
					logErr("写入 %s 期号 %s 失败: %v", TypeDLT, it.Issue, err)
				}
				if isNew {
					anyNew = true
				}
			}
			return anyNew, nil
		}

		// 尝试解析 apiopen 格式
		var ar struct {
			Code int `json:"code"`
			Data struct {
				List []struct {
					Issue  string `json:"term"`
					Date   string `json:"date"`
					Red    string `json:"red"`
					Blue   string `json:"blue"`
					Pool   string `json:"pool"`
				} `json:"list"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &ar); err == nil && ar.Code == 200 && len(ar.Data.List) > 0 {
			logInfo("备用API(%s) 成功获取 %d 条大乐透记录", url, len(ar.Data.List))
			anyNew := false
			for _, it := range ar.Data.List {
				if it.Issue == "" || it.Red == "" {
					continue
				}
				poolAmount := parsePoolAmount(it.Pool)
				isNew, err := upsertDrawWithPool(TypeDLT, it.Issue, normalizeBalls(it.Red), normalizeBalls(it.Blue), it.Date, poolAmount, "", "")
				if err != nil {
					logErr("写入 %s 期号 %s 失败: %v", TypeDLT, it.Issue, err)
				}
				if isNew {
					anyNew = true
				}
			}
			return anyNew, nil
		}
	}

	return false, fmt.Errorf("所有备用API均失败")
}

// parsePoolAmount 把 "628888733" 或 "772,986,652.76" 解析成整数（元）
func parsePoolAmount(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 可能有小数部分，取整
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
