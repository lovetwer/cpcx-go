<script setup>
import { computed, ref } from 'vue'
import { playConfig } from '../utils/match'
import { toast } from '../store/toast'
import { apiAIPredict } from '../api'
import AIPredictOverlay from './AIPredictOverlay.vue'

const props = defineProps({
  type: { type: String, default: 'ssq' }, // ssq | dlt
  mode: { type: String, default: 'single' }, // single | compound | banker
  modelValue: { type: Object, required: true }, // { red, blue, bankerRed, bankerBlue }
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const cfg = computed(() => playConfig(props.type))
const gc = (g) =>
  g === 'red'
    ? { count: cfg.value.redCount, max: cfg.value.maxRed, label: cfg.value.redLabel, bankerMax: cfg.value.minRed - 1 }
    : { count: cfg.value.blueCount, max: cfg.value.maxBlue, label: cfg.value.blueLabel, bankerMax: cfg.value.minBlue - 1 }

function nums(n) {
  return Array.from({ length: n }, (_, i) => String(i + 1).padStart(2, '0'))
}
const redNums = computed(() => nums(cfg.value.redCount))
const blueNums = computed(() => nums(cfg.value.blueCount))

function arr(g) {
  return props.modelValue[g] || []
}
function banker(g) {
  return props.modelValue['banker' + (g === 'red' ? 'Red' : 'Blue')] || []
}
function emitVal(patch) {
  emit('update:modelValue', {
    red: [],
    blue: [],
    bankerRed: [],
    bankerBlue: [],
    ...props.modelValue,
    ...patch,
  })
}

function toggleNormal(g, n) {
  if (props.disabled) return
  const m = gc(g)
  const a = arr(g).slice()
  const i = a.indexOf(n)
  if (i >= 0) a.splice(i, 1)
  else {
    if (a.length >= m.max) {
      toast(`最多选择 ${m.max} 个${m.label}`)
      return
    }
    a.push(n)
  }
  emitVal({ [g]: a })
}

function toggleBanker(g, n) {
  if (props.disabled) return
  const bkey = 'banker' + (g === 'red' ? 'Red' : 'Blue')
  const b = banker(g).slice()
  const normal = arr(g).slice()
  const bi = b.indexOf(n)
  if (bi >= 0) {
    b.splice(bi, 1) // 取消胆码，该号转为拖码（保留在 red）
  } else {
    if (b.length >= gc(g).bankerMax) {
      toast(`胆码最多 ${gc(g).bankerMax} 个${gc(g).label}`)
      return
    }
    b.push(n)
    if (!normal.includes(n)) normal.push(n)
  }
  emitVal({ [g]: normal, [bkey]: b })
}

function toggleDrag(g, n) {
  if (props.disabled) return
  if (banker(g).includes(n)) {
    toast('该号已设为胆码')
    return
  }
  const normal = arr(g).slice()
  const i = normal.indexOf(n)
  if (i >= 0) normal.splice(i, 1)
  else {
    if (normal.length >= gc(g).max) {
      toast(`最多选择 ${gc(g).max} 个${gc(g).label}`)
      return
    }
    normal.push(n)
  }
  emitVal({ [g]: normal })
}

const aiLoading = ref(false)
const aiReason = ref('')

function shuffle(arr) {
  const a = arr.slice()
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}
function randomPick() {
  if (props.disabled) return
  const reds = shuffle(redNums.value).slice(0, cfg.value.minRed).sort()
  const blues = shuffle(blueNums.value).slice(0, cfg.value.minBlue).sort()
  emitVal({ red: reds, blue: blues, bankerRed: [], bankerBlue: [] })
  aiReason.value = ''
}
async function aiPredict() {
  if (props.disabled || aiLoading.value) return
  aiLoading.value = true
  aiReason.value = ''
  try {
    const r = await apiAIPredict(props.type)
    const reds = (r.red || []).slice(0, cfg.value.maxRed).sort()
    const blues = (r.blue || []).slice(0, cfg.value.maxBlue).sort()
    if (reds.length < cfg.value.minRed || blues.length < cfg.value.minBlue) {
      toast('AI 返回号码数量不足，已退回机选', 'warn')
      randomPick()
      return
    }
    emitVal({ red: reds, blue: blues, bankerRed: [], bankerBlue: [] })
    aiReason.value = r.reason || ''
    toast('AI 预测已填充', 'success')
  } catch (e) {
    toast(e.message || 'AI 预测失败，已退回机选', 'warn')
    randomPick()
  } finally {
    aiLoading.value = false
  }
}
function clearAll() {
  emitVal({ red: [], blue: [], bankerRed: [], bankerBlue: [] })
}

const redCount = computed(() => arr('red').length)
const blueCount = computed(() => arr('blue').length)
const bankerRedCount = computed(() => banker('red').length)
const bankerBlueCount = computed(() => banker('blue').length)
const dragRedCount = computed(() => redCount.value - bankerRedCount.value)
const dragBlueCount = computed(() => blueCount.value - bankerBlueCount.value)
</script>

<template>
  <div class="picker" :class="{ disabled }">
    <!-- 单式 / 复式 -->
    <template v-if="mode !== 'banker'">
      <div class="picker-group">
        <div class="picker-head">
          <span class="picker-label">{{ cfg.redLabel }}</span>
          <span class="picker-count" :class="{ full: redCount >= gc('red').max }">
            已选 {{ redCount }}/{{ gc('red').max }}
          </span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in redNums"
            :key="'r' + n"
            type="button"
            class="pick red"
            :class="{ on: arr('red').includes(n) }"
            :disabled="disabled || (!arr('red').includes(n) && redCount >= gc('red').max)"
            @click="toggleNormal('red', n)"
          >
            {{ n }}
          </button>
        </div>
      </div>
      <div class="picker-group">
        <div class="picker-head">
          <span class="picker-label">{{ cfg.blueLabel }}</span>
          <span class="picker-count" :class="{ full: blueCount >= gc('blue').max }">
            已选 {{ blueCount }}/{{ gc('blue').max }}
          </span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in blueNums"
            :key="'b' + n"
            type="button"
            class="pick blue"
            :class="{ on: arr('blue').includes(n) }"
            :disabled="disabled || (!arr('blue').includes(n) && blueCount >= gc('blue').max)"
            @click="toggleNormal('blue', n)"
          >
            {{ n }}
          </button>
        </div>
      </div>
    </template>

    <!-- 胆拖 -->
    <template v-else>
      <div class="picker-group">
        <div class="picker-head">
          <span class="picker-label">{{ cfg.redLabel }} · 胆码</span>
          <span class="picker-count" :class="{ full: bankerRedCount >= gc('red').bankerMax }">
            胆 {{ bankerRedCount }}/{{ gc('red').bankerMax }}
          </span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in redNums"
            :key="'br' + n"
            type="button"
            class="pick red"
            :class="{ banker: banker('red').includes(n) }"
            :disabled="disabled"
            @click="toggleBanker('red', n)"
          >
            {{ n }}
          </button>
        </div>
        <div class="picker-head" style="margin-top: 12px">
          <span class="picker-label">{{ cfg.redLabel }} · 拖码</span>
          <span class="picker-count">拖 {{ dragRedCount }}</span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in redNums"
            :key="'dr' + n"
            type="button"
            class="pick red"
            :class="{ on: arr('red').includes(n) && !banker('red').includes(n) }"
            :disabled="disabled || (banker('red').includes(n)) || (!arr('red').includes(n) && redCount >= gc('red').max)"
            @click="toggleDrag('red', n)"
          >
            {{ n }}
          </button>
        </div>
      </div>

      <div class="picker-group">
        <div class="picker-head">
          <span class="picker-label">{{ cfg.blueLabel }} · 胆码</span>
          <span class="picker-count" :class="{ full: bankerBlueCount >= gc('blue').bankerMax }">
            胆 {{ bankerBlueCount }}/{{ gc('blue').bankerMax }}
          </span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in blueNums"
            :key="'bb' + n"
            type="button"
            class="pick blue"
            :class="{ banker: banker('blue').includes(n) }"
            :disabled="disabled"
            @click="toggleBanker('blue', n)"
          >
            {{ n }}
          </button>
        </div>
        <div class="picker-head" style="margin-top: 12px">
          <span class="picker-label">{{ cfg.blueLabel }} · 拖码</span>
          <span class="picker-count">拖 {{ dragBlueCount }}</span>
        </div>
        <div class="picker-grid">
          <button
            v-for="n in blueNums"
            :key="'db' + n"
            type="button"
            class="pick blue"
            :class="{ on: arr('blue').includes(n) && !banker('blue').includes(n) }"
            :disabled="disabled || (banker('blue').includes(n)) || (!arr('blue').includes(n) && blueCount >= gc('blue').max)"
            @click="toggleDrag('blue', n)"
          >
            {{ n }}
          </button>
        </div>
      </div>
    </template>

    <div class="picker-tools">
      <button type="button" class="tool-btn ai-btn" :disabled="disabled || aiLoading" @click="aiPredict">
        {{ aiLoading ? '预测中…' : 'AI预测' }}
      </button>
      <button type="button" class="tool-btn" :disabled="disabled || (!redCount && !blueCount)" @click="clearAll">
        清空
      </button>
    </div>
    <div v-if="aiReason" class="ai-reason">{{ aiReason }}</div>
    <!-- AI 预测遮罩 -->
    <AIPredictOverlay :show="aiLoading" />
  </div>
</template>
