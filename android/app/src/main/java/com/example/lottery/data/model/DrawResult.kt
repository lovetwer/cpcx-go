package com.example.lottery.data.model

/** 官方开奖结果（字段名与后端 JSON 一致） */
data class DrawResult(
    val id: Long = 0,
    val type: String = "ssq",
    val issue: String = "",
    val red_balls: String = "",
    val blue_balls: String = "",
    val draw_date: String = "",
    val pool_amount: Long = 0,  // 奖池金额（元）
    val fyj_count: String = "", // 福运奖注数（仅SSQ）
    val fyj_money: String = "", // 福运奖单注奖金（仅SSQ）
    val created_at: String = ""
)
