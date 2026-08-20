<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { apiListLottery, apiListDraw, apiDeleteLottery, apiCreateShare, WEB_BASE_URL } from '../api'
import { toast } from '../store/toast'
import { matchTicket, TIER_STYLE } from '../utils/match'
import Balls from '../components/Balls.vue'
import Spinner from '../components/Spinner.vue'

const list = ref([])
const loading = ref(false)
const filter = ref({ type: '', status: '' })
const drawMap = ref({})
// 兼容老数据：有些 ticket.issue 存的是数字期号，用这张表反查成开奖日期再渲染
const drawByIssue = ref({})
// 最新两期开奖（用于头部右侧轮播）
const latestDraws = ref([])
const curIdx = ref(0)
let rotateTimer = null

const STATUS_OPTIONS = ['', '未开奖', '未中奖', '已中奖']

async function loadDraws() {
  const map = {}
  const issueMap = {}
  const latest = []
  for (const t of ['ssq', 'dlt']) {
    const r = await apiListDraw(t)
    if (!r.ok) continue
    const list = r.data.list || []
    for (const d of list) {
      if (d.draw_date) map[t + '_' + d.draw_date] = d
      if (d.issue) issueMap[t + '_' + d.issue] = d
    }
    // 每个彩种单独按 issue 倒序，取最新 1 期
    list.sort((a, b) => {
      const av = parseInt(a.issue, 10); const bv = parseInt(b.issue, 10)
      if (!isNaN(av) && !isNaN(bv)) return bv - av
      return String(b.issue ?? '').localeCompare(String(a.issue ?? ''))
    })
    if (list.length) latest.push(list[0])
  }
  drawMap.value = map
  drawByIssue.value = issueMap
  // 轮播：最新一期双色球 + 最新一期大乐透（各 1 个）
  latestDraws.value = latest
  if (curIdx.value >= latestDraws.value.length) curIdx.value = 0
}

async function load() {
  loading.value = true
  try {
    await loadDraws()
    const r = await apiListLottery(filter.value)
    list.value = r.ok ? r.data.list || [] : []
    if (!r.ok) toast(r.data.msg || '加载失败', 'error')
  } finally {
    loading.value = false
  }
}

const currentDraw = computed(() => latestDraws.value[curIdx.value] || null)

function startRotate() {
  stopRotate()
  if (latestDraws.value.length < 2) return
  rotateTimer = setInterval(() => {
    curIdx.value = (curIdx.value + 1) % latestDraws.value.length
  }, 7000)
}
function stopRotate() {
  if (rotateTimer) { clearInterval(rotateTimer); rotateTimer = null }
}
// 手动切换并重算自动轮播计时
function goTo(i) {
  if (latestDraws.value.length < 2) return
  curIdx.value = (i + latestDraws.value.length) % latestDraws.value.length
  startRotate()
}

// 手势左右滑动切换
let touchStartX = 0
let touchStartY = 0
function onTouchStart(e) {
  const t = e.changedTouches[0]
  touchStartX = t.clientX
  touchStartY = t.clientY
}
function onTouchEnd(e) {
  const t = e.changedTouches[0]
  const dx = t.clientX - touchStartX
  const dy = t.clientY - touchStartY
  if (Math.abs(dx) > 30 && Math.abs(dx) > Math.abs(dy)) {
    goTo(curIdx.value + (dx < 0 ? 1 : -1))
  }
}

// 富化：计算中奖等级、命中球（支持复式/胆拖）
// prize_tier 可能存中文“五等奖/六等奖”（老表迁移）或数字“5/5等奖”，统一映射成 1~8
function tierNum(t) {
  if (!t) return ''
  const s = '' + t
  // 优先数字（兼容“5”“5等奖”写法）
  const dm = s.match(/\d/)
  if (dm) return dm[0]
  // 中文等级：一~七等奖 + 福运奖
  const cn = { '一': '1', '二': '2', '两': '2', '三': '3', '四': '4', '五': '5', '六': '6', '七': '7', '福': '8' }
  for (const ch of s) {
    if (cn[ch]) return cn[ch]
  }
  return ''
}
// 把 ticket.issue 统一解析成开奖日期：已是 yyyy-mm-dd 直接用；老数据里存的是数字期号，
// 通过 drawByIssue 反查成 draw_date；查不到的（极少见，例如未开奖的未来期）原样保留。
function resolveIssueDate(it) {
  const s = it.issue
  if (!s) return s
  if (typeof s === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  const d = drawByIssue.value[it.type + '_' + s]
  if (d && d.draw_date) return d.draw_date
  return s
}

// issue 字段统一存开奖日期(yyyy-mm-dd)，这里格式化为“2026年8月20日”
function fmtIssue(s) {
  if (!s) return '—'
  const p = ('' + s).split('-')
  if (p.length === 3) return `${p[0]}年${+p[1]}月${+p[2]}日`
  return s
}
const PLAY_LABEL = { single: '单式', compound: '复式', banker: '胆拖' }
const enriched = computed(() =>
  list.value.map((it) => {
    const e = { ...it, tier: it.prize_tier || '', tierNum: '', hitRed: [], hitBlue: [], matchText: '', bets: it.bets || 0, playLabel: PLAY_LABEL[it.play_type] || '单式' }
    // 展示用期号统一转成开奖日期（老数据里若存的是数字期号，这里会自动换算）
    e.displayIssue = resolveIssueDate(it)
    // 用日期查开奖表；若老数据存的是数字期号，再尝试用原值查（兜底）
    const d = drawMap.value[it.type + '_' + e.displayIssue] || drawMap.value[it.type + '_' + it.issue]
    if (!d) return e
    // 即使状态是"未开奖"，如果该期已开奖（drawMap 能查到），也计算命中信息
    const m = matchTicket(it.type, it.red_balls, it.blue_balls, it.banker_red, it.banker_blue, d.red_balls, d.blue_balls, d.pool_amount || 0)
    // 优先使用库中已记录的中奖等级（如老表迁移来的五/六等奖），缺失时实时计算
    e.tier = it.prize_tier || m.tier
    e.tierNum = tierNum(e.tier)
    e.matchText = `命中 ${m.bestMr}+${m.bestMb}`
    e.hitRed = m.hitRed
    e.hitBlue = m.hitBlue
    e.bets = m.bets || it.bets || 0
    return e
  })
)

const stats = computed(() => {
  const l = list.value
  return {
    total: l.length,
    pending: l.filter((x) => x.status === '未开奖').length,
    wins: l.filter((x) => x.status === '已中奖').length,
  }
})

function tierStyle(tier) {
  const s = TIER_STYLE[tier]
  if (!s) return {}
  return {
    background: s.bg,
    color: s.fg,
    boxShadow: `inset 0 0 0 1px ${s.ring}`,
  }
}

function statusBadge(it) {
  switch (it.status) {
    case '未开奖':
      return { text: '待开奖', cls: 'badge-pending' }
    case '未中奖':
      return { text: '未中奖', cls: 'badge-muted' }
    default:
      return null // 已中奖由等级徽标展示
  }
}

// ===== 长按进入多选模式 =====
const selectMode = ref(false)
const selectedIds = ref([])
let lpTimer = null
let lpFired = false

function enterSelect(it) {
  if (!selectMode.value) selectMode.value = true
  if (!selectedIds.value.includes(it.id)) selectedIds.value = [...selectedIds.value, it.id]
}
function onContextMenu(it) {
  lpFired = true
  enterSelect(it)
}
function onCardTouchStart(it) {
  lpFired = false
  lpTimer = setTimeout(() => { lpFired = true; enterSelect(it) }, 500)
}
function onCardTouchEnd() {
  if (lpTimer) { clearTimeout(lpTimer); lpTimer = null }
}
function onCardClick(it) {
  if (lpFired) { lpFired = false; return } // 长按后的 click 忽略，避免立刻取消选择
  if (selectMode.value) toggleSelect(it)
}
function toggleSelect(it) {
  if (selectedIds.value.includes(it.id)) selectedIds.value = selectedIds.value.filter((x) => x !== it.id)
  else selectedIds.value = [...selectedIds.value, it.id]
}
function exitSelect() {
  selectMode.value = false
  selectedIds.value = []
}
async function deleteSelected() {
  const ids = selectedIds.value
  if (!ids.length) return
  if (!confirm(`确定删除选中的 ${ids.length} 张彩票？`)) return
  let ok = 0
  for (const id of ids) {
    const r = await apiDeleteLottery(id)
    if (r.ok) ok++
  }
  toast(`已删除 ${ok} 张`, 'success')
  exitSelect()
  load()
}
async function doShare(items) {
  if (!items.length) { toast('还没有彩票可分享', 'error'); return }
  toast('正在生成分享链接…', 'info')
  const r = await apiCreateShare(items.map((it) => it.id))
  if (!r.ok) {
    toast(r.data.msg || '生成失败', 'error')
    return
  }
  const code = r.data.code
  const url = `${WEB_BASE_URL}/#/share?code=${code}`
  if (navigator.share) {
    try { await navigator.share({ title: '我的彩票', text: '看看我的彩票', url }) } catch (e) { /* 用户取消 */ }
  } else if (navigator.clipboard) {
    try { await navigator.clipboard.writeText(url); toast('分享链接已复制', 'success') }
    catch (e) { toast('复制失败，请手动复制', 'error') }
  } else {
    toast('请手动复制链接', 'error')
  }
  exitSelect()
}
function shareSelected() {
  doShare(enriched.value.filter((it) => selectedIds.value.includes(it.id)))
}

onMounted(async () => {
  await load()
  startRotate()
})
onBeforeUnmount(stopRotate)
</script>

<template>
  <div class="page" :class="{ 'has-select-bar': selectMode }">
    <header class="page-head">
      <div>
        <div class="eyebrow">购彩</div>
        <h2>我的购彩</h2>
        <p class="page-sub">共 {{ stats.total }} 张 · 待开奖 {{ stats.pending }} · 已中奖 {{ stats.wins }}</p>
      </div>
      <div v-if="currentDraw" :key="curIdx" class="latest-draw" @touchstart.passive="onTouchStart" @touchend.passive="onTouchEnd">
        <div class="latest-meta">
          <div class="latest-issue">第 {{ currentDraw.issue }} 期</div>
          <div class="latest-date">{{ currentDraw.draw_date }}</div>
        </div>
        <Balls :type="currentDraw.type" :red="currentDraw.red_balls" :blue="currentDraw.blue_balls" stacked />
        <div v-if="latestDraws.length > 1" class="latest-dots">
          <span
            v-for="(_, i) in latestDraws"
            :key="i"
            class="dot"
            :class="{ active: i === curIdx }"
            @click="goTo(i)"
          ></span>
        </div>
      </div>
    </header>

    <div class="toolbar">
      <select v-model="filter.type" @change="load">
        <option value="">全部彩种</option>
        <option value="ssq">双色球</option>
        <option value="dlt">大乐透</option>
      </select>
      <select v-model="filter.status" @change="load">
        <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">{{ s || '全部状态' }}</option>
      </select>
    </div>
    <p class="select-hint">长按卡片可进入多选，批量分享或删除</p>

    <div v-if="loading && !list.length" class="state-box"><Spinner /></div>

    <div v-else-if="!list.length" class="state-box empty">
      <svg class="empty-art" viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <rect x="10" y="14" width="44" height="36" rx="6" />
        <path d="M10 25 H54" />
        <circle cx="20" cy="20" r="1.6" fill="currentColor" stroke="none" />
        <circle cx="28" cy="20" r="1.6" fill="currentColor" stroke="none" />
        <path d="M16 33 H32 M16 40 H26" />
      </svg>
      <p>还没有彩票，去「录入」页添加吧</p>
    </div>

    <div v-else class="lottery-grid stagger">
      <article
        v-for="it in enriched"
        :key="it.id"
        class="card ticket"
        :class="['ticket-' + it.type, { 'ticket-won': it.status === '已中奖' || it.tier, 'ticket-pending': it.status === '未开奖', 'is-selected': selectMode && selectedIds.includes(it.id), ['tier-' + it.tierNum]: it.tierNum }]"
        @contextmenu.prevent="onContextMenu(it)"
        @touchstart.passive="onCardTouchStart(it)"
        @touchend.passive="onCardTouchEnd"
        @click="onCardClick(it)"
      >
        <div class="ticket-top">
          <span class="chip" :class="it.type === 'dlt' ? 'chip-dlt' : 'chip-ssq'">
            {{ it.type === 'dlt' ? '大乐透' : '双色球' }}
          </span>
          <span class="ticket-issue">{{ fmtIssue(it.displayIssue) }}</span>
          <span class="chip chip-play">{{ it.playLabel }}</span>
          <span v-if="it.multiple > 1" class="chip chip-mult">{{ it.multiple }}倍</span>
          <template v-if="!selectMode">
            <span v-if="statusBadge(it)" class="badge" :class="statusBadge(it).cls">{{ statusBadge(it).text }}</span>
            <span v-else-if="it.tier" class="tier-badge" :style="tierStyle(it.tier)">{{ it.tier }}</span>
            <span v-else class="badge badge-muted">已中奖</span>
          </template>
          <span v-else class="select-check" :class="{ on: selectedIds.includes(it.id) }"></span>
        </div>

        <Balls :type="it.type" :red="it.red_balls" :blue="it.blue_balls" :hit-red="it.hitRed" :hit-blue="it.hitBlue" />

        <div class="ticket-foot">
          <span v-if="it.matchText" class="match-text">{{ it.matchText }}</span>
          <span v-else class="muted-sm">{{ it.status === '未开奖' ? '该期尚未开奖' : '未命中' }}</span>
          <span class="muted-sm">{{ it.bets }} 注</span>
        </div>
      </article>
    </div>

    <!-- 多选操作条：仅在长按进入选择态后出现 -->
    <div v-if="selectMode" class="select-bar">
      <button class="btn btn-ghost" @click="exitSelect">取消</button>
      <span class="select-count">已选 {{ selectedIds.length }} 张</span>
      <button class="btn btn-primary" @click="shareSelected">分享</button>
      <button class="btn btn-danger" @click="deleteSelected">删除</button>
    </div>
  </div>
</template>
