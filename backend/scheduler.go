package main

import (
	"net/http"
	"os"
	"time"
)

// startScheduler 启动定时拉奖与定时核对。
//
// 拉奖逻辑：
//   - 每天晚上 21:20 开始，每 5 分钟拉一次
//   - 拉到新数据（当期开奖号已发布）就停
//   - 星期六不拉（双色球、大乐透都不开奖）
//   - 超过 23:30 也停（不会再出了）
//
// 核对+通知逻辑：
//   - 固定时刻 09:00 / 21:40 / 22:00 / 22:30 / 23:00 跑 ScheduledCheckAndNotify
//   - 只核对 status=待开奖 的彩票，发通知给首次中奖的用户
func startScheduler(cfg Config) {
	// 启动时只拉一次奖，不跑核对（避免重启后重复发通知）
	go func() {
		time.Sleep(5 * time.Second)
		_, _ = PullLottery()
	}()

	// 拉奖定时器：21:20 ~ 23:30 之间每 5 分钟拉一次
	go startPullSchedule()

	// 保活：每 10 分钟 ping 自己的 /health，防止 Render 免费层休眠
	go startKeepAlive()

	// 固定时刻核对+通知：09:00 / 21:40 / 22:00 / 22:30 / 23:00
	startFixedSchedule()
}

// startPullSchedule 每天晚上 21:20 开始拉奖，每 5 分钟一次，
// 拉到新数据就停，超过 23:30 也停。星期六不拉。
func startPullSchedule() {
	for {
		now := time.Now()

		// 星期六不拉（双色球周二/四/日，大乐透周一/三/六——周六只有大乐透，但大乐透周一三六开奖，周六是开奖日但晚上才出）
		// 用户要求周六不拉，两个都不拉
		if now.Weekday() == time.Saturday {
			// 算到明天 21:20
			tomorrow := now.AddDate(0, 0, 1)
			next := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 21, 20, 0, 0, now.Location())
			d := next.Sub(now)
			logInfo("今天是星期六，跳过拉奖，下次拉奖: %s", next.Format("2006-01-02 15:04"))
			time.Sleep(d)
			continue
		}

		// 计算今天 21:20 的时间点
		startTime := time.Date(now.Year(), now.Month(), now.Day(), 21, 20, 0, 0, now.Location())
		endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 30, 0, 0, now.Location())

		// 还没到 21:20，等到 21:20
		if now.Before(startTime) {
			d := startTime.Sub(now)
			logInfo("拉奖等待中，今晚 21:20 开始，约 %s 后", d.Round(time.Second))
			time.Sleep(d)
			continue
		}

		// 已经过了 23:30，等明天 21:20
		if now.After(endTime) {
			tomorrow := now.AddDate(0, 0, 1)
			next := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 21, 20, 0, 0, now.Location())
			d := next.Sub(now)
			logInfo("今天拉奖窗口已过（23:30），下次拉奖: %s", next.Format("2006-01-02 15:04"))
			time.Sleep(d)
			continue
		}

		// 在 21:20 ~ 23:30 窗口内，执行拉奖
		anyNew, err := PullLottery()
		if err != nil {
			logErr("拉奖失败: %v", err)
		}
		if anyNew {
			logInfo("拉到新开奖数据，停止本轮拉奖")
			// 拉到了，等明天 21:20
			tomorrow := now.AddDate(0, 0, 1)
			next := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 21, 20, 0, 0, now.Location())
			d := next.Sub(now)
			time.Sleep(d)
			continue
		}

		// 没拉到新数据，5 分钟后再试
		logInfo("暂无新开奖数据，5 分钟后重试")
		time.Sleep(5 * time.Minute)
	}
}

// startKeepAlive 每 10 分钟 ping 自己的 /health，防止 Render 免费层休眠。
func startKeepAlive() {
	// Render 自动注入 RENDER_EXTERNAL_URL；没有则用配置的地址
	baseURL := os.Getenv("RENDER_EXTERNAL_URL")
	if baseURL == "" {
		baseURL = "https://cpcx-app.onrender.com"
	}
	url := baseURL + "/health"

	ticker := time.NewTicker(10 * time.Minute)
	client := &http.Client{Timeout: 10 * time.Second}

	go func() {
		// 启动后先 ping 一次
		pingSelf(client, url)
		for range ticker.C {
			pingSelf(client, url)
		}
	}()
}

func pingSelf(client *http.Client, url string) {
	resp, err := client.Get(url)
	if err != nil {
		logErr("保活 ping 失败: %v", err)
		return
	}
	resp.Body.Close()
	logInfo("保活 ping 成功: %s → %d", url, resp.StatusCode)
}
