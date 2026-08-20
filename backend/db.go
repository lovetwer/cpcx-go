package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DB 是全局数据库连接
var DB *sql.DB

// InitDB 连接 MySQL 并初始化表结构
func InitDB(cfg Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local&timeout=5s&multiStatements=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	DB.SetMaxOpenConns(20)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(time.Hour)
	if err = DB.Ping(); err != nil {
		return err
	}
	log.Println("✅ 已连接 MySQL")
	return initSchema()
}

func initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    username   VARCHAR(64)  NOT NULL,
    password   VARCHAR(255) NOT NULL,
    device_id  VARCHAR(128) NOT NULL DEFAULT '',
    email      VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_username (username),
    KEY idx_device (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS lotteries (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    type       VARCHAR(16)  NOT NULL COMMENT 'ssq/dlt',
    issue      VARCHAR(32)  NOT NULL COMMENT '期号',
    red_balls  VARCHAR(128) NOT NULL COMMENT '红球/前区，逗号分隔，已补零排序',
    blue_balls VARCHAR(64)  NOT NULL COMMENT '蓝球/后区，逗号分隔',
    status     VARCHAR(16)  NOT NULL DEFAULT '未开奖',
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_user_created (user_id, created_at),
    KEY idx_type_issue (type, issue)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS draw_results (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    type       VARCHAR(16)  NOT NULL,
    issue      VARCHAR(32)  NOT NULL,
    red_balls  VARCHAR(128) NOT NULL,
    blue_balls VARCHAR(64)  NOT NULL,
    draw_date  VARCHAR(32)  NOT NULL DEFAULT '',
    pool_amount BIGINT      NOT NULL DEFAULT 0 COMMENT '奖池金额（元）',
    fyj_count  VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '福运奖注数（仅SSQ）',
    fyj_money  VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '福运奖单注奖金（仅SSQ）',
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_draw (type, issue)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shares (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    code       VARCHAR(16)  NOT NULL,
    user_id    BIGINT       NOT NULL,
    lottery_ids TEXT        NOT NULL,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}
	if err := migrateLotteries(); err != nil {
		return err
	}
	if err := migrateLotteriesIndex(); err != nil {
		return err
	}
	if err := migrateLotteriesIssue(); err != nil {
		return err
	}
	return migrateDrawResults()
}

// migrateLotteriesIssue 把 lotteries.issue 里的"期号数字"格式统一转成"yyyy-mm-dd"日期格式，
// 解决老数据/OCR未匹配到 draw_results 时存入期号导致排序错乱的问题。
// 幂等：日期格式不受影响；无对应 draw_results 的期号保持原样（避免误改）。
func migrateLotteriesIssue() error {
	q := `
UPDATE lotteries l
JOIN draw_results d ON d.type = l.type AND d.issue = l.issue
SET l.issue = d.draw_date
WHERE l.issue NOT LIKE '____-__-__' AND d.draw_date != ''`
	res, err := DB.Exec(q)
	if err != nil {
		logInfo("migrate: lotteries.issue 转换跳过: %v", err)
		return nil
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		logInfo("migrate: lotteries.issue 期号→日期转换 %d 行", n)
	}
	return nil
}

// migrateLotteriesIndex 为 lotteries 增加 (user_id, created_at) 联合索引，
// 配合列表查询“按录入时间倒序”的排序，避免文件排序、提升查询效率（幂等）。
func migrateLotteriesIndex() error {
	var cnt int
	_ = DB.QueryRow(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='lotteries' AND index_name='idx_user_created'",
	).Scan(&cnt)
	if cnt > 0 {
		return nil
	}
	if _, err := DB.Exec("ALTER TABLE lotteries ADD INDEX idx_user_created (user_id, created_at)"); err != nil {
		return err
	}
	logInfo("migrate: added index lotteries.idx_user_created")
	return nil
}

// migrateLotteries 给 lotteries 表增量补充“玩法/倍数/胆码/注数”字段（幂等，已存在则跳过）
func migrateLotteries() error {
	cols := []string{
		"play_type VARCHAR(16) NOT NULL DEFAULT 'single' COMMENT 'single/compound/banker'",
		"multiple INT NOT NULL DEFAULT 1",
		"banker_red VARCHAR(128) NOT NULL DEFAULT ''",
		"banker_blue VARCHAR(64) NOT NULL DEFAULT ''",
		"bets INT NOT NULL DEFAULT 1",
		"prize_tier VARCHAR(16) NOT NULL DEFAULT '' COMMENT '中奖等级：五等奖/六等奖…空=未中奖'",
	}
	for _, c := range cols {
		name := strings.Fields(c)[0]
		var cnt int
		_ = DB.QueryRow(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='lotteries' AND column_name=?",
			name,
		).Scan(&cnt)
		if cnt > 0 {
			continue
		}
		if _, err := DB.Exec("ALTER TABLE lotteries ADD COLUMN " + c); err != nil {
			return err
		}
		logInfo("migrate: added column lotteries.%s", name)
	}
	return backfillCpPrizeTier()
}

// migrateDrawResults 给 draw_results 表增量补充奖池金额、福运奖字段（幂等）
func migrateDrawResults() error {
	cols := []string{
		"pool_amount BIGINT NOT NULL DEFAULT 0 COMMENT '奖池金额（元）'",
		"fyj_count VARCHAR(16) NOT NULL DEFAULT '' COMMENT '福运奖注数（仅SSQ）'",
		"fyj_money VARCHAR(16) NOT NULL DEFAULT '' COMMENT '福运奖单注奖金（仅SSQ）'",
	}
	for _, c := range cols {
		name := strings.Fields(c)[0]
		var cnt int
		_ = DB.QueryRow(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='draw_results' AND column_name=?",
			name,
		).Scan(&cnt)
		if cnt > 0 {
			continue
		}
		if _, err := DB.Exec("ALTER TABLE draw_results ADD COLUMN " + c); err != nil {
			return err
		}
		logInfo("migrate: added column draw_results.%s", name)
	}
	return nil
}

// backfillCpPrizeTier 把旧表 cp 的 status 映射回写进 lotteries：
//   cp.status 空 -> 未开奖；0 -> 未中奖；1~6 -> 已中奖 + 对应中奖等级（N等奖）
// 通过 (users.username=cp.user) + (lotteries.issue=cp.openTime) + 红球 + 蓝球 关联。
// 幂等：重复执行结果一致；cp 表缺失或不匹配时不影响其它数据。
func backfillCpPrizeTier() error {
	q := `
UPDATE lotteries l
JOIN users u ON u.id = l.user_id
JOIN cp ON cp.user = u.username
        AND cp.openTime = l.issue
        AND cp.redBall = l.red_balls
        AND cp.blueBall = l.blue_balls
SET l.status = CASE
        WHEN cp.status IS NULL OR cp.status = '' OR cp.status = '0' THEN '未中奖'
        WHEN cp.status BETWEEN 1 AND 6 THEN '已中奖'
        ELSE '未中奖' END,
    l.prize_tier = CASE
        WHEN cp.status BETWEEN 1 AND 6 THEN CONCAT(ELT(cp.status,'一','二','三','四','五','六'),'等奖')
        ELSE '' END`
	res, err := DB.Exec(q)
	if err != nil {
		// cp 表可能已被清理：视为无需回填，不阻断启动
		logInfo("backfill: 跳过（cp 表不可用）: %v", err)
		return nil
	}
	n, _ := res.RowsAffected()
	logInfo("backfill: cp->lotteries 回写 %d 行（含中奖等级）", n)
	return nil
}
