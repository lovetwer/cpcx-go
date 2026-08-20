// 中奖匹配与等级计算（纯前端，依据现有 /api/lottery 与 /api/draw 数据实时计算）
// 规则与中国福利彩票 / 体彩官方一致。

export function splitNums(s) {
  return (s || '')
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

export function intersectCount(a, b) {
  const set = new Set(b)
  return a.filter((x) => set.has(x)).length
}

// 命中的号码（用于球上打标）
export function hitBalls(userRed, userBlue, drawRed, drawBlue) {
  const dr = new Set(splitNums(drawRed))
  const db = new Set(splitNums(drawBlue))
  return {
    hitRed: splitNums(userRed).filter((x) => dr.has(x)),
    hitBlue: splitNums(userBlue).filter((x) => db.has(x)),
  }
}

// 计算中奖等级。返回 { mr, mb, tier, won }
// tier: ''（未中奖）/ '一等奖' ... '七等奖' / '福运奖'
// poolAmount: 该期奖池金额（元），用于判断福运奖是否生效（SSQ奖池≥15亿才触发）
export function matchTier(type, userRed, userBlue, drawRed, drawBlue, poolAmount = 0) {
  const mr = intersectCount(splitNums(userRed), splitNums(drawRed))
  const mb = intersectCount(splitNums(userBlue), splitNums(drawBlue))
  let tier = ''
  if (type === 'ssq') {
    // 双色球（2026新规）：福运奖只在奖池≥15亿时生效
    const fyjActive = poolAmount >= 1500000000 // 15亿元
    if (mr === 6 && mb === 1) tier = '一等奖'
    else if (mr === 6 && mb === 0) tier = '二等奖'
    else if (mr === 5 && mb === 1) tier = '三等奖'
    else if ((mr === 5 && mb === 0) || (mr === 4 && mb === 1)) tier = '四等奖'
    else if ((mr === 4 && mb === 0) || (mr === 3 && mb === 1)) tier = '五等奖'
    else if (mb === 1) tier = '六等奖'
    else if (mr === 3 && mb === 0 && fyjActive) tier = '福运奖'
  } else {
    // 大乐透（2026新规7等奖级）
    if (mr === 5 && mb === 2) tier = '一等奖'
    else if (mr === 5 && mb === 1) tier = '二等奖'
    else if ((mr === 5 && mb === 0) || (mr === 4 && mb === 2)) tier = '三等奖'
    else if (mr === 4 && mb === 1) tier = '四等奖'
    else if ((mr === 4 && mb === 0) || (mr === 3 && mb === 2)) tier = '五等奖'
    else if ((mr === 3 && mb === 1) || (mr === 2 && mb === 2)) tier = '六等奖'
    else if ((mr === 3 && mb === 0) || (mr === 2 && mb === 1) || (mr === 1 && mb === 2) || (mr === 0 && mb === 2)) tier = '七等奖'
  }
  return { mr, mb, tier, won: tier !== '' }
}

// ---------- 复式 / 胆拖 展开 ----------

export function playConfig(type) {
  if (type === 'dlt') {
    return { minRed: 5, maxRed: 12, minBlue: 2, maxBlue: 12, redCount: 35, blueCount: 12, name: '大乐透', redLabel: '前区', blueLabel: '后区' }
  }
  return { minRed: 6, maxRed: 20, minBlue: 1, maxBlue: 16, redCount: 33, blueCount: 16, name: '双色球', redLabel: '红球', blueLabel: '蓝球' }
}

function combinations(arr, k) {
  const res = []
  const n = arr.length
  if (k < 0 || k > n) return res
  const idx = new Array(k)
  const rec = (start, i) => {
    if (i === k) {
      res.push(idx.map((j) => arr[j]))
      return
    }
    for (let j = start; j < n; j++) {
      idx[i] = j
      rec(j + 1, i + 1)
    }
  }
  rec(0, 0)
  return res
}

function diff(a, b) {
  const set = new Set(b)
  return a.filter((x) => !set.has(x))
}

// enumerateCombos 把复式/胆拖展开为所有单式组合，返回 [{red:[],blue:[]}]
export function enumerateCombos(type, red, blue, bankerRed, bankerBlue) {
  const cfg = playConfig(type)
  const reds = splitNums(red)
  const blues = splitNums(blue)
  const bRed = splitNums(bankerRed)
  const bBlue = splitNums(bankerBlue)
  let redCombos, blueCombos
  if (bRed.length) {
    redCombos = combinations(diff(reds, bRed), cfg.minRed - bRed.length).map((c) => [...bRed, ...c])
  } else {
    redCombos = combinations(reds, cfg.minRed)
  }
  if (bBlue.length) {
    blueCombos = combinations(diff(blues, bBlue), cfg.minBlue - bBlue.length).map((c) => [...bBlue, ...c])
  } else {
    blueCombos = combinations(blues, cfg.minBlue)
  }
  const out = []
  for (const rc of redCombos) for (const bc of blueCombos) out.push({ red: rc, blue: bc })
  return out
}

// ticketBets 计算注数（不含倍数）
export function ticketBets(type, red, blue, bankerRed, bankerBlue) {
  const cfg = playConfig(type)
  const reds = splitNums(red)
  const blues = splitNums(blue)
  const bRed = splitNums(bankerRed)
  const bBlue = splitNums(bankerBlue)
  let rc = 1
  let bc = 1
  if (bRed.length) rc = combinations(diff(reds, bRed), cfg.minRed - bRed.length).length
  else rc = combinations(reds, cfg.minRed).length
  if (bBlue.length) bc = combinations(diff(blues, bBlue), cfg.minBlue - bBlue.length).length
  else bc = combinations(blues, cfg.minBlue).length
  return rc * bc
}

const TIER_RANK = { 一等奖: 1, 二等奖: 2, 三等奖: 3, 四等奖: 4, 五等奖: 5, 六等奖: 6, 七等奖: 7, 福运奖: 8 }

// matchTicket 处理复式/胆拖，返回最佳中奖等级与命中球（命中球取所选号码与开奖号的交集）
// poolAmount: 该期奖池金额（元），用于判断福运奖是否生效及大乐透奖金升级
export function matchTicket(type, red, blue, bankerRed, bankerBlue, drawRed, drawBlue, poolAmount = 0) {
  const bets = ticketBets(type, red, blue, bankerRed, bankerBlue)
  const combos = enumerateCombos(type, red, blue, bankerRed, bankerBlue)
  let best = ''
  let bestMr = 0
  let bestMb = 0
  for (const c of combos) {
    const r = matchTier(type, c.red.join(','), c.blue.join(','), drawRed, drawBlue, poolAmount)
    // 不管是否中奖，都记录最大命中数（用于 UI 展示"命中 X+Y"）
    if (r.mr > bestMr || (r.mr === bestMr && r.mb > bestMb)) {
      bestMr = r.mr
      bestMb = r.mb
    }
    // 只在中奖时更新最佳等级
    if (r.tier && (!best || TIER_RANK[r.tier] < TIER_RANK[best])) {
      best = r.tier
    }
  }
  // 命中球：所选号码中出现在开奖结果里的（直观展示哪些号中了）
  const dr = new Set(splitNums(drawRed))
  const db = new Set(splitNums(drawBlue))
  const hitRed = splitNums(red).filter((x) => dr.has(x))
  const hitBlue = splitNums(blue).filter((x) => db.has(x))
  return {
    won: best !== '',
    tier: best,
    bestMr,
    bestMb,
    bets,
    hitRed,
    hitBlue,
    matchText: `命中 ${bestMr}+${bestMb}`,
  }
}

// 计算单注奖金描述。一等奖/二等奖为浮动奖，其余为固定奖金。
// 大乐透在奖池≥8亿时，三至七等奖自动上浮（2026新规）。
export function prizeMoney(type, tier, poolAmount = 0) {
  if (type === 'ssq') {
    switch (tier) {
      case '一等奖': return '浮动（最高500万）'
      case '二等奖': return '浮动'
      case '三等奖': return '3000元'
      case '四等奖': return '200元'
      case '五等奖': return '10元'
      case '六等奖': return '5元'
      case '福运奖': return '5元'
      default: return ''
    }
  } else {
    const boost = poolAmount >= 800000000 // 8亿元
    switch (tier) {
      case '一等奖': return '浮动（最高1000万）'
      case '二等奖': return '浮动'
      case '三等奖': return boost ? '6666元' : '5000元'
      case '四等奖': return boost ? '380元' : '300元'
      case '五等奖': return boost ? '200元' : '150元'
      case '六等奖': return boost ? '18元' : '15元'
      case '七等奖': return boost ? '7元' : '5元'
      default: return ''
    }
  }
}

// 把奖池金额（元）格式化为可读字符串，如"6.29亿"
export function poolAmountDesc(poolAmount) {
  if (!poolAmount || poolAmount <= 0) return ''
  const yi = poolAmount / 100000000
  if (yi >= 1) return yi.toFixed(2) + '亿'
  const wan = poolAmount / 10000
  return Math.round(wan) + '万'
}

// 等级徽标配色（统一红色主题，按深浅区分：一等奖最深 → 六等奖最浅）
export const TIER_STYLE = {
  一等奖: { bg: 'rgba(200,24,46,0.18)', fg: '#c0152f', ring: 'rgba(200,24,46,0.45)' },
  二等奖: { bg: 'rgba(216,52,63,0.15)', fg: '#d8343f', ring: 'rgba(216,52,63,0.42)' },
  三等奖: { bg: 'rgba(232,90,95,0.14)', fg: '#d8343f', ring: 'rgba(216,52,63,0.38)' },
  四等奖: { bg: 'rgba(240,122,126,0.13)', fg: '#d8343f', ring: 'rgba(216,52,63,0.34)' },
  五等奖: { bg: 'rgba(245,154,157,0.12)', fg: '#d8343f', ring: 'rgba(216,52,63,0.30)' },
  六等奖: { bg: 'rgba(250,184,186,0.11)', fg: '#c0152f', ring: 'rgba(216,52,63,0.26)' },
  七等奖: { bg: 'rgba(250,200,200,0.10)', fg: '#c0152f', ring: 'rgba(216,52,63,0.22)' },
  福运奖: { bg: 'rgba(255,193,7,0.15)', fg: '#b8860b', ring: 'rgba(255,193,7,0.40)' },
}
