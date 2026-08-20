<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { apiMe, apiUpdateMe, apiListLottery, apiDeleteMe } from '../api'
import { auth, clearAuth, setAuth } from '../store/auth'
import { toast } from '../store/toast'
import Spinner from '../components/Spinner.vue'
import Modal from '../components/Modal.vue'

const router = useRouter()
const editing = ref(false)
const form = ref({ username: '', email: '' })
const saving = ref(false)
const stats = ref({ total: 0, wins: 0, pending: 0 })
const loadingStats = ref(false)
const showDisclaimer = ref(false)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

async function deleteAccount() {
  deleting.value = true
  try {
    const r = await apiDeleteMe()
    if (r.ok) {
      clearAuth()
      toast('账号已注销', 'info')
      router.push('/login')
    } else {
      toast(r.data?.msg || '注销失败', 'error')
    }
  } catch (e) {
    toast(e?.message || '注销失败', 'error')
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
  }
}

// 修改密码
const pwdPanel = ref(false)
const pwdForm = ref({ old_password: '', password: '', confirm: '' })
const savingPwd = ref(false)

function togglePwdPanel() {
  if (pwdPanel.value) {
    pwdPanel.value = false
  } else {
    pwdForm.value = { old_password: '', password: '', confirm: '' }
    pwdPanel.value = true
  }
}

async function savePassword() {
  const newP = pwdForm.value.password.trim()
  const confirm = pwdForm.value.confirm.trim()
  const old = pwdForm.value.old_password.trim()
  if (!newP) return toast('请输入新密码', 'warn')
  if (newP.length < 6) return toast('密码至少 6 位', 'warn')
  if (newP !== confirm) return toast('两次输入的新密码不一致', 'warn')
  savingPwd.value = true
  try {
    const payload = { password: newP }
    if (old) payload.old_password = old
    const r = await apiUpdateMe(payload)
    if (r.ok) {
      if (r.data?.user) setAuth(auth.token, r.data.user)
      toast('密码已修改，已双端同步', 'success')
      pwdPanel.value = false
    } else toast(r.data?.msg || '修改失败', 'error')
  } catch (e) {
    toast(e?.message || '修改失败', 'error')
  } finally {
    savingPwd.value = false
  }
}

function startEdit() {
  form.value = {
    username: auth.user?.username || '',
    email: auth.user?.email || '',
  }
  editing.value = true
}

async function saveProfile() {
  if (!form.value.email && !form.value.username) return toast('没有修改', 'warn')
  saving.value = true
  try {
    const payload = {}
    if (form.value.email) payload.email = form.value.email
    if (form.value.username) payload.username = form.value.username
    const r = await apiUpdateMe(payload)
    if (r.ok) {
      setAuth(auth.token, r.data.user)
      toast('资料已更新', 'success')
      editing.value = false
    } else toast(r.data.msg || '更新失败', 'error')
  } finally {
    saving.value = false
  }
}

async function loadStats() {
  loadingStats.value = true
  try {
    const r = await apiListLottery({})
    const l = r.ok ? r.data.list || [] : []
    stats.value = {
      total: l.length,
      wins: l.filter((x) => x.status === '已中奖').length,
      pending: l.filter((x) => x.status === '未开奖').length,
    }
  } finally {
    loadingStats.value = false
  }
}

function logout() {
  clearAuth()
  toast('已退出登录', 'info')
  router.push('/login')
}

const regDate = computed(() => {
  const c = auth.user?.created_at
  if (!c) return '—'
  return String(c).slice(0, 10)
})

onMounted(() => {
  apiMe().then((r) => {
    if (r.ok && r.data.user) setAuth(auth.token, r.data.user)
  })
  loadStats()
})
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <div class="eyebrow">我的</div>
        <h2>个人中心</h2>
        <p class="page-sub">管理账户资料与购彩记录</p>
      </div>
    </header>

    <!-- 用户卡片 -->
    <section class="card profile-card">
      <div class="profile-top">
        <span class="avatar lg">{{ (auth.user?.username || '?').slice(0, 1).toUpperCase() }}</span>
        <div class="profile-id">
          <div class="profile-name">{{ auth.user?.username }}</div>
          <div class="profile-sub">设备号 {{ (auth.user?.device_id || '—').slice(0, 10) }}…</div>
        </div>
      </div>

      <div class="profile-field">
        <span class="field-label">用户名</span>
        <span v-if="!editing" class="field-value">{{ auth.user?.username }}</span>
        <input v-else v-model="form.username" placeholder="用户名" />
      </div>
      <div class="profile-field">
        <span class="field-label">邮箱</span>
        <span v-if="!editing" class="field-value">{{ auth.user?.email || '未填写' }}</span>
        <input v-else v-model="form.email" type="email" placeholder="用于接收中奖通知" />
      </div>
      <div class="profile-field">
        <span class="field-label">注册时间</span>
        <span class="field-value">{{ regDate }}</span>
      </div>

      <div class="profile-actions">
        <template v-if="!editing">
          <button class="btn btn-ghost" @click="startEdit">编辑资料</button>
          <button class="btn btn-ghost" @click="togglePwdPanel">{{ pwdPanel ? '收起密码' : '修改密码' }}</button>
        </template>
        <template v-else>
          <button class="btn btn-ghost" @click="editing = false">取消</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveProfile">
            <Spinner v-if="saving" light /> 保存
          </button>
        </template>
      </div>
    </section>

    <!-- 修改密码 -->
    <section class="card profile-card" v-if="pwdPanel">
      <div class="profile-top">
        <span class="avatar lg">🔑</span>
        <div class="profile-id">
          <div class="profile-name">修改密码</div>
          <div class="profile-sub">原密码留空可直接设置（一键登录账户无需原密码）</div>
        </div>
      </div>

      <div class="profile-field">
        <span class="field-label">原密码</span>
        <input v-model="pwdForm.old_password" type="password" placeholder="选填，已有密码时填写" />
      </div>
      <div class="profile-field">
        <span class="field-label">新密码</span>
        <input v-model="pwdForm.password" type="password" placeholder="至少 6 位" />
      </div>
      <div class="profile-field">
        <span class="field-label">确认新密码</span>
        <input v-model="pwdForm.confirm" type="password" placeholder="再次输入新密码" />
      </div>

      <div class="profile-actions">
        <button class="btn btn-ghost" @click="togglePwdPanel">取消</button>
        <button class="btn btn-primary" :disabled="savingPwd" @click="savePassword">
          <Spinner v-if="savingPwd" light /> 保存
        </button>
      </div>
    </section>

    <!-- 数据统计 -->
    <section class="stat-row">
      <div class="card stat">
        <div class="stat-num">{{ loadingStats ? '—' : stats.total }}</div>
        <div class="stat-label">彩票总数</div>
      </div>
      <div class="card stat">
        <div class="stat-num win">{{ loadingStats ? '—' : stats.wins }}</div>
        <div class="stat-label">中奖次数</div>
      </div>
      <div class="card stat">
        <div class="stat-num pending">{{ loadingStats ? '—' : stats.pending }}</div>
        <div class="stat-label">待开奖</div>
      </div>
    </section>

    <!-- 下载 APP -->
    <section class="card download-card">
      <img class="download-icon" src="/logo.png" alt="APP" />
      <div class="download-body">
        <div class="download-title">下载 APP 体验更多功能</div>
        <div class="download-desc">拍照识票、一键录入、中奖推送，体验更流畅</div>
      </div>
      <a class="btn btn-primary download-btn" href="https://github.com/lovetwer/cpcx-android/releases/latest" target="_blank" rel="noopener">下载</a>
    </section>

    <button class="btn btn-block btn-danger logout-btn" @click="logout">退出登录</button>

    <button class="btn btn-block btn-text delete-account-btn" @click="showDeleteConfirm = true">注销账号</button>

    <!-- 注销账号确认弹窗 -->
    <Modal :show="showDeleteConfirm" title="注销账号" @close="showDeleteConfirm = false">
      <div class="delete-warning">
        <p class="delete-warning-title">⚠️ 危险操作</p>
        <p>注销后，您的账号、所有彩票记录和分享将<strong>永久删除</strong>，且<strong>不可恢复</strong>。</p>
        <p>确定要注销账号吗？</p>
      </div>
      <div class="modal-foot">
        <button class="btn btn-ghost" @click="showDeleteConfirm = false" :disabled="deleting">取消</button>
        <button class="btn btn-danger" @click="deleteAccount" :disabled="deleting">
          <Spinner v-if="deleting" light /> 确认注销
        </button>
      </div>
    </Modal>

    <!-- 免责声明 -->
    <div class="disclaimer-footer">
      <p>本应用仅供学习交流使用，不涉及博彩或投注。开奖数据以官方公告为准，请遵守国家法律法规。</p>
      <p>继续使用即视为您已阅读并同意<a href="javascript:void(0)" @click="showDisclaimer = true">《免责声明》</a>全部内容。</p>
    </div>

    <!-- 完整免责声明弹窗 -->
    <Modal :show="showDisclaimer" title="免责声明" @close="showDisclaimer = false">
      <div class="disclaimer-full">
        <p><strong>一、项目性质</strong></p>
        <p>本应用（彩票管家）是一个个人学习与技术交流项目，仅供学习编程、数据库设计、前后端开发等用途，不涉及任何形式的博彩、投注或资金交易。</p>
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
.download-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
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
  color: var(--text);
}
.download-desc {
  font-size: 12px;
  color: var(--muted);
  margin-top: 2px;
}
.download-btn {
  flex-shrink: 0;
  padding: 8px 18px;
  font-size: 14px;
  text-decoration: none;
}
.disclaimer-footer {
  margin-top: 20px;
  padding: 14px 16px;
  background: var(--surface-2);
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  text-align: center;
}
.disclaimer-footer p {
  margin: 0 0 4px;
}
.disclaimer-footer p:last-child {
  margin-bottom: 0;
}
.disclaimer-footer a {
  color: var(--primary);
  font-weight: 600;
}
.disclaimer-full {
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted);
}
.disclaimer-full p {
  margin: 0 0 10px;
}
.disclaimer-full p:last-child {
  margin-bottom: 0;
}
.disclaimer-full strong {
  color: var(--text);
  font-size: 14px;
}
.delete-account-btn {
  margin-top: 8px;
  color: #e53e3e;
  font-size: 13px;
  opacity: 0.8;
}
.delete-account-btn:hover {
  opacity: 1;
}
.delete-warning {
  font-size: 14px;
  line-height: 1.7;
  color: var(--muted);
}
.delete-warning p {
  margin: 0 0 10px;
}
.delete-warning-title {
  color: #e53e3e;
  font-weight: 700;
  font-size: 15px;
}
.delete-warning strong {
  color: #e53e3e;
}
</style>
