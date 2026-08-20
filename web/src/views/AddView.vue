<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  apiCreateLottery,
  apiBatchLottery,
  apiRecognize,
  apiListDraw,
} from '../api'
import { playConfig, ticketBets } from '../utils/match'
import { toast } from '../store/toast'
import BallPicker from '../components/BallPicker.vue'
import Modal from '../components/Modal.vue'
import Spinner from '../components/Spinner.vue'

const router = useRouter()

const type = ref('ssq')
const mode = ref('single') // single | compound | banker
const issue = ref('')
const multiple = ref(1)
const picked = ref({ red: [], blue: [], bankerRed: [], bankerBlue: [] })
const saving = ref(false)
const draws = ref([])

const cfg = computed(() => playConfig(type.value))
const weekNames = ['日', '一', '二', '三', '四', '五', '六']
const schedule = computed(() => (type.value === 'ssq' ? [0, 2, 4] : [1, 3, 6]))

// 注数（不含倍数）
const bets = computed(() =>
  ticketBets(type.value, picked.value.red.join(','), picked.value.blue.join(','), picked.value.bankerRed.join(','), picked.value.bankerBlue.join(','))
)
const amount = computed(() => bets.value * multiple.value * 2)

function fmtDate(s) {
  if (!s) return ''
  const dt = new Date(s.replace(/-/g, '/'))
  if (isNaN(dt)) return s
  return `${dt.getMonth() + 1}月${dt.getDate()}日`
}
function weekName(s) {
  if (!s) return ''
  const dt = new Date(s.replace(/-/g, '/'))
  if (isNaN(dt)) return ''
  return weekNames[dt.getDay()]
}

// 本地日期格式化为 YYYY-MM-DD（避免 toISOString 的 UTC 偏移把日期整体往前挪一天）
function isoLocal(d) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// 未开奖日期：从今天起按开奖日生成（今天若恰为开奖日亦可购买）
const upcoming = computed(() => {
  const weeks = schedule.value
  const out = []
  const cur = new Date()
  cur.setHours(0, 0, 0, 0)
  let guard = 0
  while (out.length < 12 && guard < 400) {
    guard++
    if (weeks.includes(cur.getDay())) {
      const d = isoLocal(cur)
      out.push({ issue: d, date: d })
    }
    cur.setDate(cur.getDate() + 1)
  }
  return out
})
const history = computed(() =>
  draws.value
    .slice()
    .sort((a, b) => parseInt(b.issue) - parseInt(a.issue))
    .map((d) => ({ issue: d.issue, date: d.draw_date }))
)

async function loadDraws() {
  try {
    const r = await apiListDraw(type.value)
    draws.value = r.ok ? r.data.list || [] : []
  } catch {
    draws.value = []
  }
}
// 默认开奖日期：最近一期“未开奖·可购买”的日期（即最新可投注期）
function defaultIssue() {
  return upcoming.value[0]?.date || ''
}
onMounted(() => {
  issue.value = defaultIssue()
  loadDraws()
})

function switchType(t) {
  if (t === type.value) return
  type.value = t
  mode.value = 'single'
  picked.value = { red: [], blue: [], bankerRed: [], bankerBlue: [] }
  issue.value = defaultIssue()
  multiple.value = 1
  loadDraws()
}
function switchMode(m) {
  if (m === mode.value) return
  mode.value = m
  picked.value = { red: [], blue: [], bankerRed: [], bankerBlue: [] }
}

const valid = computed(() => {
  const c = cfg.value
  const r = picked.value.red.length
  const b = picked.value.blue.length
  const br = picked.value.bankerRed.length
  const bb = picked.value.bankerBlue.length
  const dr = r - br
  const db = b - bb
  if (!issue.value.trim()) return false
  if (mode.value === 'single') return r === c.minRed && b === c.minBlue
  if (mode.value === 'compound')
    return (
      r >= c.minRed && r <= c.maxRed && b >= c.minBlue && b <= c.maxBlue && (r > c.minRed || b > c.minBlue)
    )
  // banker
  if (br === 0 && bb === 0) return false
  if (br > c.minRed - 1 || bb > c.minBlue - 1) return false
  if (dr < c.minRed - br || db < c.minBlue - bb) return false
  if (r > c.maxRed || b > c.maxBlue) return false
  return true
})

async function save() {
  if (!valid.value) {
    if (!issue.value.trim()) return toast('请选择开奖日期', 'warn')
    return toast('号码不符合当前玩法规则', 'warn')
  }
  saving.value = true
  try {
    const r = await apiCreateLottery({
      type: type.value,
      issue: issue.value.trim(),
      red_balls: picked.value.red.join(','),
      blue_balls: picked.value.blue.join(','),
      play_type: mode.value,
      multiple: multiple.value,
      banker_red: picked.value.bankerRed.join(','),
      banker_blue: picked.value.bankerBlue.join(','),
    })
    if (r.ok) {
      toast('录入成功', 'success')
      issue.value = defaultIssue()
      picked.value = { red: [], blue: [], bankerRed: [], bankerBlue: [] }
      multiple.value = 1
      router.push('/buy')
    } else toast(r.data.msg || '录入失败', 'error')
  } finally {
    saving.value = false
  }
}

// ---------- 图片识别（合并录入：识别后预填选球，用户确认再保存） ----------
const file = ref(null)
const preview = ref('')
const recognizing = ref(false)
const multiParsed = ref([]) // 一次识别到多注时的待保存列表

function setFile(f) {
  if (!f) return
  file.value = f
  preview.value = URL.createObjectURL(f)
}
function onFile(e) {
  setFile(e.target.files?.[0])
  runRecognize()
}

async function runRecognize() {
  if (!file.value) return toast('请先选择彩票截图', 'warn')
  recognizing.value = true
  try {
    const r = await apiRecognize(file.value, true)
    if (r.ok && r.data.parsed && r.data.parsed.length > 1) {
      // 一次识别到多注：列出全部，由用户核对后一键保存
      multiParsed.value = r.data.parsed
      toast('识别到 ' + r.data.parsed.length + ' 注，请核对后点击「保存全部」', 'info')
      file.value = null
      preview.value = ''
    } else if (r.ok && r.data.parsed && r.data.parsed.length === 1) {
      const p = r.data.parsed[0]
      type.value = p.type
      // 单注识别：期号缺失时自动归入最近一期已开奖，避免保存被拒
      issue.value = p.issue || history.value[0]?.issue || defaultIssue()
      mode.value = 'single'
      picked.value = {
        red: (p.red_balls || '').split(',').map((s) => s.trim()).filter(Boolean),
        blue: (p.blue_balls || '').split(',').map((s) => s.trim()).filter(Boolean),
        bankerRed: [],
        bankerBlue: [],
      }
      toast('已从图片识别，请核对后点击「保存彩票」', 'info')
      file.value = null
      preview.value = ''
    } else if (r.ok && r.data.skipped && r.data.skipped.length) {
      toast(r.data.skipped[0].reason || '未能识别出有效号码，请换一张清晰截图或手动录入', 'error')
    } else {
      toast(r.data?.msg || '识别失败，请换一张号码区清晰的截图或手动录入', 'error')
    }
  } finally {
    recognizing.value = false
  }
}

// 一键保存识别到的多注（走批量录入接口）
async function saveMulti() {
  if (!multiParsed.value.length) return
  saving.value = true
  try {
    // OCR 若没识别到期号：优先用最近一期已开奖（更贴合旧票），否则退回最近可购买期
    const fallbackIssue = history.value[0]?.issue || defaultIssue()
    const missingIssue = multiParsed.value.some((p) => !p.issue)
    const items = multiParsed.value.map((p) => ({
      type: p.type,
      issue: (p.issue || fallbackIssue).trim(),
      red_balls: p.red_balls,
      blue_balls: p.blue_balls,
    }))
    const r = await apiBatchLottery(items)
    if (r.ok && r.data) {
      const inserted = r.data.inserted ?? 0
      const failed = r.data.failed ?? 0
      const errors = r.data.errors || []
      if (inserted > 0 && failed === 0) {
        const tip = missingIssue ? '（期号未识别，已归入最近一期，可在录入列表删除重录）' : ''
        toast(`已保存 ${inserted} 注${tip}`, 'success')
        multiParsed.value = []
        file.value = null
        preview.value = ''
        // 全部保存成功，跳到购彩列表查看
        router.push('/buy')
      } else if (inserted > 0) {
        // 部分成功：留下失败的，提示具体原因
        toast(`已保存 ${inserted} 注，${failed} 注失败：${errors[0] || '号码不符合规则'}`, 'warn')
      } else {
        // 全部失败：明确告诉原因，并保留列表让用户核对/修改
        const reason = errors[0] || '号码不符合规则（双色球需6红+1蓝，大乐透需5前+2后）'
        toast(`保存失败：${reason}`, 'error')
      }
    } else {
      toast(r.data?.msg || '保存失败（网络或服务器错误）', 'error')
    }
  } finally {
    saving.value = false
  }
}

// ---------- 批量录入 ----------
const showBatch = ref(false)
const batchText = ref('')
const batching = ref(false)
const batchResult = ref(null)

function parseBatch(text) {
  const items = []
  for (const line of text.split('\n')) {
    const t = line.trim()
    if (!t) continue
    const p = t.split(',')
    if (p.length < 4) continue
    items.push({
      type: p[0].trim().toLowerCase(),
      issue: p[1].trim(),
      red_balls: p[2].trim(),
      blue_balls: p[3].trim(),
    })
  }
  return items
}
async function batchAdd() {
  const items = parseBatch(batchText.value)
  if (!items.length) return toast('没有可解析的记录，格式见下方示例', 'warn')
  batching.value = true
  batchResult.value = null
  try {
    const r = await apiBatchLottery(items)
    if (r.ok) {
      batchResult.value = r.data
      toast(`成功 ${r.data.inserted} 条，失败 ${r.data.failed} 条`, r.data.failed ? 'warn' : 'success')
    } else toast(r.data.msg || '批量失败', 'error')
  } finally {
    batching.value = false
  }
}
</script>

<template>
  <div class="page">
    <!-- 彩种 -->
    <div class="seg">
      <button :class="{ active: type === 'ssq' }" @click="switchType('ssq')">双色球</button>
      <button :class="{ active: type === 'dlt' }" @click="switchType('dlt')">大乐透</button>
    </div>

    <!-- 玩法：单式 / 复式 / 胆拖 -->
    <div class="seg seg-mode">
      <button :class="{ active: mode === 'single' }" @click="switchMode('single')">单式</button>
      <button :class="{ active: mode === 'compound' }" @click="switchMode('compound')">复式</button>
      <button :class="{ active: mode === 'banker' }" @click="switchMode('banker')">胆拖</button>
    </div>

    <!-- 开奖日期（未开奖可购 + 已开奖可补录） -->
    <label class="field" v-if="draws.length">
      <span>开奖日期</span>
      <select v-model="issue" class="select">
        <option value="" disabled>可改选其它开奖日期（已默认最近一期）</option>
        <optgroup label="未开奖 · 可购买">
          <option v-for="d in upcoming" :key="'u' + d.issue" :value="d.date">
            {{ fmtDate(d.date) }} 周{{ weekName(d.date) }} · 未开奖
          </option>
        </optgroup>
        <optgroup label="已开奖 · 可补录旧票" v-if="history.length">
          <option v-for="d in history" :key="'h' + d.issue" :value="d.date">
            {{ fmtDate(d.date) }} 周{{ weekName(d.date) }} · 已开奖
          </option>
        </optgroup>
      </select>
      <span class="field-hint">默认选“未开奖”期，买的是未来开奖的彩票</span>
    </label>
    <label class="field" v-else>
      <span>期号</span>
      <input v-model="issue" placeholder="如 2026-08-20" inputmode="numeric" />
      <span class="field-hint">暂无开奖数据，可手动填写期号</span>
    </label>

    <!-- 选球 -->
    <BallPicker :type="type" :mode="mode" v-model="picked" />

    <!-- 本轮已选（实时展示） -->
    <div class="pick-preview">
      <div class="pp-head">本轮已选（实时）</div>
      <div class="pp-list">
        <div class="pp-group">
          <span class="pp-tag red">{{ cfg.redLabel }}</span>
          <span v-if="!picked.red.length" class="pp-empty">未选</span>
          <span v-for="n in picked.red" :key="'r' + n" class="ball ball-red ball-sm">{{ n }}</span>
        </div>
        <div class="pp-group" v-if="mode === 'banker'">
          <span class="pp-tag red">红胆</span>
          <span v-if="!picked.bankerRed.length" class="pp-empty">未设</span>
          <span v-for="n in picked.bankerRed" :key="'br' + n" class="ball ball-red ball-sm banker">{{ n }}</span>
        </div>
        <div class="pp-group">
          <span class="pp-tag blue">{{ cfg.blueLabel }}</span>
          <span v-if="!picked.blue.length" class="pp-empty">未选</span>
          <span v-for="n in picked.blue" :key="'b' + n" class="ball ball-blue ball-sm">{{ n }}</span>
        </div>
        <div class="pp-group" v-if="mode === 'banker'">
          <span class="pp-tag blue">蓝胆</span>
          <span v-if="!picked.bankerBlue.length" class="pp-empty">未设</span>
          <span v-for="n in picked.bankerBlue" :key="'bb' + n" class="ball ball-blue ball-sm banker">{{ n }}</span>
        </div>
      </div>
      <div class="pp-bets">
        共 <b>{{ bets }}</b> 注 · {{ multiple }} 倍 · 合计 <b>¥{{ amount }}</b>
      </div>
    </div>

    <!-- 图片识别（合并） -->
    <div class="ocr-card">
      <input
        id="ocr-file"
        type="file"
        accept="image/*"
        capture="environment"
        class="hidden-input"
        @change="onFile"
      />
      <label for="ocr-file" class="ocr-drop" :class="{ busy: recognizing }">
        <svg class="ocr-icon" viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
          <rect x="7" y="11" width="34" height="30" rx="5" />
          <circle cx="17" cy="21" r="3.4" />
          <path d="M11 34 L21 25 L28 31 L34 26 L41 33" />
        </svg>
        <span>{{ recognizing ? '识别中…' : '上传彩票截图，自动识别号码' }}</span>
        <Spinner v-if="recognizing" light />
      </label>
      <p v-if="preview" class="ocr-preview-tip">已选择图片，正在识别…</p>
    </div>

    <!-- 多注识别结果 -->
    <div v-if="multiParsed.length > 1" class="ocr-multi">
      <div class="ocr-multi-head">识别到 <b>{{ multiParsed.length }}</b> 注，请核对后一键保存</div>
      <div v-for="(b, i) in multiParsed" :key="i" class="ocr-multi-item">
        <span class="ocr-multi-idx">{{ i + 1 }}</span>
        <span class="ocr-multi-type">{{ b.type === 'dlt' ? '大乐透' : '双色球' }}</span>
        <span class="ocr-multi-issue">{{ b.issue ? ('第' + b.issue + '期') : '期号待识别' }}</span>
        <span class="ocr-multi-balls">
          <span v-for="n in (b.red_balls || '').split(',').filter(Boolean)" :key="'r' + i + n" class="ball ball-red ball-sm">{{ n }}</span>
          <span v-for="n in (b.blue_balls || '').split(',').filter(Boolean)" :key="'b' + i + n" class="ball ball-blue ball-sm">{{ n }}</span>
        </span>
      </div>
      <div class="ocr-multi-actions">
        <button class="btn-primary" :disabled="saving" @click="saveMulti">保存全部 {{ multiParsed.length }} 注</button>
        <button class="btn-ghost" @click="multiParsed = []">取消</button>
      </div>
    </div>

    <!-- 批量录入入口 -->
    <button class="link-btn" @click="showBatch = true">批量录入（多张）</button>

    <!-- 倍数 -->
    <label class="field field-row">
      <span>倍数</span>
      <div class="stepper">
        <button type="button" @click="multiple = Math.max(1, multiple - 1)">−</button>
        <input v-model.number="multiple" inputmode="numeric" min="1" max="99" />
        <button type="button" @click="multiple = Math.min(99, multiple + 1)">+</button>
      </div>
    </label>

    <!-- 底部保存栏 -->
    <div class="save-bar">
      <div class="save-summary">
        <span>{{ cfg.name }} · {{ mode === 'single' ? '单式' : mode === 'compound' ? '复式' : '胆拖' }}</span>
        <span class="muted">开奖日 {{ issue ? fmtDate(issue) : '—' }} · {{ bets }} 注 × {{ multiple }} 倍 = ¥{{ amount }}</span>
      </div>
      <button class="btn btn-primary btn-lg" :disabled="saving" @click="save">
        <Spinner v-if="saving" light /> 保存彩票
      </button>
    </div>

    <!-- 批量录入弹窗 -->
    <Modal :show="showBatch" title="批量录入" @close="showBatch = false">
      <p class="hint">每行一条，格式：<code>彩种,期号,红球,蓝球</code></p>
      <p class="hint">示例：<code>ssq,2024080,03 08 15 20 21 24,09</code></p>
      <textarea
        v-model="batchText"
        rows="7"
        placeholder="ssq,2024080,03 08 15 20 21 24,09&#10;dlt,24080,03 04 07 12 32,01 02"
      ></textarea>
      <div v-if="batchResult" class="batch-result">
        <div class="batch-ok">成功 {{ batchResult.inserted }} 条</div>
        <div v-if="batchResult.failed" class="batch-fail">失败 {{ batchResult.failed }} 条</div>
        <ul v-if="batchResult.errors && batchResult.errors.length">
          <li v-for="(e, i) in batchResult.errors" :key="i">{{ e }}</li>
        </ul>
      </div>
      <div class="modal-foot">
        <button class="btn btn-ghost" @click="showBatch = false">关闭</button>
        <button class="btn btn-primary" :disabled="batching" @click="batchAdd">
          <Spinner v-if="batching" light /> 开始录入
        </button>
      </div>
    </Modal>
  </div>
</template>
