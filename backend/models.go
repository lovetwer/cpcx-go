package main

import "time"

// 彩票类型
const (
	TypeSSQ = "ssq" // 双色球
	TypeDLT = "dlt" // 大乐透
)

// 彩票状态
const (
	StatusPending  = "未开奖" // 该期尚未开奖
	StatusNoWin    = "未中奖"
	StatusWin      = "已中奖"
	StatusRedeemed = "已兑奖"
)

// User 用户
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // 不对外暴露
	DeviceID  string    `json:"device_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Lottery 用户录入的彩票
type Lottery struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Type       string    `json:"type"`
	Issue      string    `json:"issue"`
	RedBalls   string    `json:"red_balls"`
	BlueBalls  string    `json:"blue_balls"`
	PlayType   string    `json:"play_type"`   // single=单式 / compound=复式 / banker=胆拖
	Multiple   int       `json:"multiple"`    // 倍数
	BankerRed  string    `json:"banker_red"`  // 红球胆码（逗号分隔，仅胆拖）
	BankerBlue string    `json:"banker_blue"` // 蓝球/后区胆码
	Bets       int       `json:"bets"`        // 注数
	Status     string    `json:"status"`
	PrizeTier  string    `json:"prize_tier"`  // 中奖等级（五等奖/六等奖…），空表示未中奖
	CreatedAt  time.Time `json:"created_at"`
}

// DrawResult 官方开奖结果
type DrawResult struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Issue      string `json:"issue"`
	RedBalls   string `json:"red_balls"`
	BlueBalls  string `json:"blue_balls"`
	DrawDate   string `json:"draw_date"`
	PoolAmount int64  `json:"pool_amount"` // 奖池金额（元），0=未获取
	FyjCount   string `json:"fyj_count"`   // 福运奖注数（仅SSQ）
	FyjMoney   string `json:"fyj_money"`   // 福运奖单注奖金（仅SSQ）
	CreatedAt  time.Time `json:"created_at"`
}

// TypeName 返回中文彩种名
func TypeName(t string) string {
	switch t {
	case TypeSSQ:
		return "双色球"
	case TypeDLT:
		return "大乐透"
	default:
		return t
	}
}
