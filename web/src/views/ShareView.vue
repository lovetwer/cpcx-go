<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { apiGetShare } from '../api'
import { matchTicket, TIER_STYLE } from '../utils/match'
import Balls from '../components/Balls.vue'
import Spinner from '../components/Spinner.vue'
import Modal from '../components/Modal.vue'

const route = useRoute()
const list = ref([])
const showDisclaimer = ref(false)
const loading = ref(true)
const error = ref('')

function fmtIssue(s) {
  if (!s) return '—'
  const p = ('' + s).split('-')
  if (p.length === 3) return `${p[0]}年${+p[1]}月${+p[2]}日`
  return s
}

const PLAY_LABEL = { single: '单式', compound: '复式', banker: '胆拖' }

function tierNum(t) {
  if (!t) return ''
  const s = '' + t
  const dm = s.match(/\d/)
  if (dm) return dm[0]
  const cn = { '一': '1', '二': '2', '两': '2', '三': '3', '四': '4', '五': '5', '六': '6', '七': '7', '福': '8' }
  for (const ch of s) {
    if (cn[ch]) return cn[ch]
  }
  return ''
}

const enriched = computed(() =>
  list.value.map((it) => {
    const e = { ...it, tier: it.prize_tier || '', tierNum: '', hitRed: [], hitBlue: [], matchText: '', playLabel: PLAY_LABEL[it.play_type] || '单式' }
    if (it.draw_red && it.draw_blue) {
      const m = matchTicket(it.type, it.red_balls, it.blue_balls, it.banker_red, it.banker_blue, it.draw_red, it.draw_blue, it.pool_amount || 0)
      e.tier = it.prize_tier || m.tier
      e.tierNum = tierNum(e.tier)
      e.matchText = `命中 ${m.bestMr}+${m.bestMb}`
      e.hitRed = m.hitRed
      e.hitBlue = m.hitBlue
    }
    return e
  })
)

const stats = computed(() => {
  const l = list.value
  return {
    total: l.length,
    wins: l.filter((x) => x.status === '已中奖' || x.prize_tier).length,
    pending: l.filter((x) => x.status === '未开奖').length,
  }
})

function tierStyle(tier) {
  const s = TIER_STYLE[tier]
  if (!s) return {}
  return { background: s.bg, color: s.fg, boxShadow: `inset 0 0 0 1px ${s.ring}` }
}

function statusBadge(it) {
  if (it.tier) return null
  switch (it.status) {
    case '未开奖': return { text: '待开奖', cls: 'badge-pending' }
    case '未中奖': return { text: '未中奖', cls: 'badge-muted' }
    default: return { text: '已中奖', cls: 'badge-muted' }
  }
}

onMounted(async () => {
  const code = route.query.code
  if (!code) {
    error.value = '分享码缺失'
    loading.value = false
    return
  }
  const r = await apiGetShare(code)
  if (!r.ok) {
    error.value = r.data.msg || '分享不存在或已过期'
    loading.value = false
    return
  }
  list.value = r.data.list || []
  loading.value = false
})
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <div class="eyebrow">分享</div>
        <h2>彩票分享</h2>
        <p class="page-sub" v-if="!loading && !error">共 {{ stats.total }} 张 · 已中奖 {{ stats.wins }} · 待开奖 {{ stats.pending }}</p>
      </div>
    </header>

    <div v-if="loading" class="state-box"><Spinner /></div>

    <div v-else-if="error" class="state-box empty">
      <p>{{ error }}</p>
    </div>

    <div v-else-if="!list.length" class="state-box empty">
      <p>分享中没有彩票</p>
    </div>

    <div v-else class="lottery-grid stagger">
      <article
        v-for="(it, idx) in enriched"
        :key="idx"
        class="card ticket"
        :class="['ticket-' + it.type, { 'ticket-won': it.tier, 'ticket-pending': it.status === '未开奖', ['tier-' + it.tierNum]: it.tierNum }]"
      >
        <div class="ticket-top">
          <span class="chip" :class="it.type === 'dlt' ? 'chip-dlt' : 'chip-ssq'">
            {{ it.type === 'dlt' ? '大乐透' : '双色球' }}
          </span>
          <span class="ticket-issue">{{ fmtIssue(it.issue) }}</span>
          <span class="chip chip-play">{{ it.playLabel }}</span>
          <span v-if="it.multiple > 1" class="chip chip-mult">{{ it.multiple }}倍</span>
          <span v-if="statusBadge(it)" class="badge" :class="statusBadge(it).cls">{{ statusBadge(it).text }}</span>
          <span v-else-if="it.tier" class="tier-badge" :style="tierStyle(it.tier)">{{ it.tier }}</span>
        </div>

        <Balls :type="it.type" :red="it.red_balls" :blue="it.blue_balls" :hit-red="it.hitRed" :hit-blue="it.hitBlue" />

        <div class="ticket-foot">
          <span v-if="it.matchText" class="match-text">{{ it.matchText }}</span>
          <span v-else class="muted-sm">{{ it.status === '未开奖' ? '该期尚未开奖' : '未命中' }}</span>
          <span class="muted-sm">{{ it.bets || 1 }} 注</span>
        </div>

        <div v-if="it.draw_red" class="ticket-draw">
          <span class="muted-sm">开奖：{{ it.draw_red }}{{ it.draw_blue ? ' + ' + it.draw_blue : '' }}</span>
        </div>
      </article>
    </div>

    <!-- 下载 APP -->
    <section class="card download-card">
      <img class="download-icon" src="/logo.png" alt="APP" />
      <div class="download-body">
        <div class="download-title">下载 APP 体验更多功能</div>
        <div class="download-desc">拍照识票、一键录入、中奖推送，体验更流畅</div>
      </div>
      <a class="btn btn-primary download-btn" href="https://github.com/lovetwer/cpcx-android/releases/latest" target="_blank" rel="noopener">下载</a>
    </section>

    <p class="share-footer">由大奖来了生成</p>

    <!-- 免责声明 -->
    <div class="disclaimer-share">
      <p>本应用仅供学习交流使用，不涉及博彩或投注。开奖数据以官方公告为准，请遵守国家法律法规。使用即视为同意<a href="javascript:void(0)" @click="showDisclaimer = true">《免责声明》</a>。</p>
    </div>

    <!-- 完整免责声明弹窗 -->
    <Modal :show="showDisclaimer" title="免责声明" @close="showDisclaimer = false">
      <div class="disclaimer-full">
        <p><strong>一、项目性质</strong></p>
        <p>本应用（大奖来了）是一个个人学习与技术交流项目，仅供学习编程、数据库设计、前后端开发等用途，不涉及任何形式的博彩、投注或资金交易。</p>
        <p><strong>二、数据来源</strong></p>
        <p>应用中展示的开奖数据、彩票信息均来自公开渠道，仅供参考。所有开奖结果以中国体育彩票、中国福利彩票官方公告为准，本应用不对数据准确性做任何保证。</p>
        <p><strong>三、使用规范</strong></p>
        <p>用户在使用本应用时应严格遵守中华人民共和国相关法律法规。不得将本应用用于任何违法违规用途，包括但不限于组织博彩、赌博、非法集资等。</p>
        <p><strong>四、责任限制</strong></p>
        <p>因使用本应用而产生的任何直接或间接损失，开发者不承担任何法律责任。用户应自行承担使用本应用的一切风险。</p>
        <p><strong>五、最终解释</strong></p>
        <p>开发者保留对本声明的最终解释权。本声明可能不定期更新，继续使用即视为接受修改后的内容。</p>
      </div>
      <div class="modal-foot">
        <button class="btn btn-primary btn-block" @click="showDisclaimer = false">我已知晓</button>
      </div>
    </Modal>
  </div>
</template>

<style scoped>
.share-footer {
  text-align: center;
  color: var(--muted, #999);
  font-size: 12px;
  margin-top: 24px;
}
.ticket-draw {
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px dashed rgba(0,0,0,0.06);
}
.download-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  margin-top: 24px;
}
.download-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  flex-shrink: 0;
  object-fit: cover;
}
.download-body {
  flex: 1;
  min-width: 0;
}
.download-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text, #222);
}
.download-desc {
  font-size: 12px;
  color: var(--muted, #999);
  margin-top: 2px;
}
.download-btn {
  flex-shrink: 0;
  padding: 8px 18px;
  font-size: 14px;
  text-decoration: none;
}
.disclaimer-share {
  margin-top: 16px;
  padding: 12px 16px;
  background: var(--surface-2, #f1f1f4);
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted, #999);
  text-align: center;
}
.disclaimer-share p {
  margin: 0;
}
.disclaimer-share a {
  color: var(--primary, #ff4d4d);
  font-weight: 600;
}
.disclaimer-full {
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted, #616061);
}
.disclaimer-full p {
  margin: 0 0 10px;
}
.disclaimer-full p:last-child {
  margin-bottom: 0;
}
.disclaimer-full strong {
  color: var(--text, #1d1c1d);
  font-size: 14px;
}
</style>
