<script setup>
import { ref, onMounted } from 'vue'
import { apiListDraw } from '../api'
import Balls from '../components/Balls.vue'
import Spinner from '../components/Spinner.vue'
import { poolAmountDesc } from '../utils/match'

const type = ref('ssq')
const list = ref([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const r = await apiListDraw(type.value)
    const arr = r.ok ? r.data.list || [] : []
    // 按时间倒序：最新一期排最上方（期号为定长数字，字符串倒序即可）
    arr.sort((a, b) => (b.issue || '').localeCompare(a.issue || ''))
    list.value = arr
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <div class="eyebrow">开奖数据</div>
        <h2>开奖结果</h2>
        <p class="page-sub">官方开奖数据 · 定时自动拉取</p>
      </div>
    </header>

    <div class="tabs tabs-block">
      <button :class="{ active: type === 'ssq' }" @click="(type = 'ssq'), load()">双色球</button>
      <button :class="{ active: type === 'dlt' }" @click="(type = 'dlt'), load()">大乐透</button>
    </div>

    <div v-if="loading && !list.length" class="state-box"><Spinner /></div>
    <div v-else-if="!list.length" class="state-box empty">
      <svg class="empty-art" viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M14 22 H50 V44 H14 Z" />
        <path d="M14 22 L32 36 L50 22" />
      </svg>
      <p>暂无开奖数据</p>
    </div>

    <div v-else class="draw-list stagger">
      <article v-for="d in list" :key="d.id" class="card draw-row">
        <div class="draw-issue">
          <span class="chip" :class="d.type === 'dlt' ? 'chip-dlt' : 'chip-ssq'">
            {{ d.type === 'dlt' ? '大乐透' : '双色球' }}
          </span>
          <div>
            <div class="draw-no">第 {{ d.issue }} 期</div>
            <div class="draw-date">{{ d.draw_date || '—' }}</div>
          </div>
          <span v-if="d.pool_amount > 0" class="pool-badge">奖池 {{ poolAmountDesc(d.pool_amount) }}</span>
        </div>
        <Balls :type="d.type" :red="d.red_balls" :blue="d.blue_balls" />
      </article>
    </div>
  </div>
</template>
