<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiRegister, apiLogin, apiDeviceLogin } from '../api'
import { setAuth, getDeviceId } from '../store/auth'
import { toast } from '../store/toast'
import Spinner from '../components/Spinner.vue'

const router = useRouter()
const mode = ref('device') // device | login | register
const username = ref('')
const password = ref('')
const email = ref('')
const loading = ref(false)
const agreed = ref(false)
const showDisclaimer = ref(false)

async function submit() {
  if (!agreed.value) {
    toast('请先阅读并同意免责声明', 'warn')
    return
  }
  loading.value = true
  try {
    let r
    if (mode.value === 'login') {
      if (!username.value || !password.value) {
        toast('请输入用户名和密码', 'warn')
        return
      }
      r = await apiLogin(username.value, password.value)
    } else if (mode.value === 'register') {
      if (!username.value || !password.value) {
        toast('请输入用户名和密码', 'warn')
        return
      }
      r = await apiRegister(username.value, password.value, email.value, getDeviceId())
    } else {
      r = await apiDeviceLogin(getDeviceId(), email.value, navigator.userAgent.includes('Mobile') ? 'WebMobile' : 'WebPC')
    }

    if (r.ok && r.data.token) {
      setAuth(r.data.token, r.data.user)
      toast('登录成功', 'success')
      router.push('/add')
    } else {
      toast(r.data.msg || '操作失败', 'error')
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-head">
        <img src="/logo.png" alt="logo" class="login-logo-img" />
        <h1>大奖来了</h1>
        <p>双色球 · 大乐透 彩票管理</p>
      </div>

      <div class="tabs">
        <button :class="{ active: mode === 'device' }" @click="mode = 'device'">一键登录</button>
        <button :class="{ active: mode === 'login' }" @click="mode = 'login'">登录</button>
        <button :class="{ active: mode === 'register' }" @click="mode = 'register'">注册</button>
      </div>

      <form class="login-form" @submit.prevent="submit">
        <label v-if="mode !== 'device'">
          <span>用户名</span>
          <input v-model="username" autocomplete="username" placeholder="请输入用户名" />
        </label>

        <label v-if="mode === 'login' || mode === 'register'">
          <span>密码</span>
          <input v-model="password" type="password" autocomplete="current-password" placeholder="请输入密码" />
        </label>

        <label v-if="mode === 'register' || mode === 'device'">
          <span>邮箱（选填，中奖时用于通知）</span>
          <input v-model="email" type="email" placeholder="you@example.com" />
        </label>

        <button class="btn btn-primary btn-block btn-lg" :disabled="loading">
          <Spinner v-if="loading" light />
          <span>{{ mode === 'device' ? '一键登录' : mode === 'register' ? '注册并登录' : '登录' }}</span>
        </button>
      </form>

      <p class="login-tip">
        一键登录：本机生成持久设备号，下次打开自动登录，无需记密码。
      </p>

      <!-- 免责声明 -->
      <div class="disclaimer-check">
        <label class="disclaimer-label">
          <input type="checkbox" v-model="agreed" class="disclaimer-checkbox" />
          <span>我已阅读并同意</span>
          <button type="button" class="disclaimer-link" @click="showDisclaimer = !showDisclaimer">《免责声明》</button>
        </label>
      </div>

      <Transition name="disclaimer">
        <div v-if="showDisclaimer" class="disclaimer-body">
          <p><strong>免责声明</strong></p>
          <p>本应用（大奖来了）仅供学习与交流使用，不涉及任何形式的博彩、投注或资金交易。</p>
          <p>应用中展示的开奖数据、彩票信息仅供参考，不构成任何购彩建议。请以中国体育彩票/中国福利彩票官方公告为准。</p>
          <p>用户在使用本应用时应遵守国家法律法规。因使用本应用而产生的任何直接或间接损失，开发者不承担任何责任。</p>
          <p>继续使用即视为您已阅读、理解并同意上述全部内容。</p>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.disclaimer-check {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}
.disclaimer-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--muted);
  cursor: pointer;
  flex-direction: row;
}
.disclaimer-checkbox {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  accent-color: var(--primary);
  cursor: pointer;
}
.disclaimer-link {
  background: none;
  border: none;
  color: var(--primary);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
  text-decoration: underline;
  text-underline-offset: 2px;
}
.disclaimer-body {
  margin-top: 12px;
  padding: 14px 16px;
  background: var(--surface-2);
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--muted);
}
.disclaimer-body p {
  margin: 0 0 6px;
}
.disclaimer-body p:last-child {
  margin-bottom: 0;
}
.disclaimer-body strong {
  color: var(--text);
  font-size: 13px;
}
.disclaimer-enter-active,
.disclaimer-leave-active {
  transition: opacity 220ms ease, max-height 220ms ease;
  overflow: hidden;
  max-height: 300px;
}
.disclaimer-enter-from,
.disclaimer-leave-to {
  opacity: 0;
  max-height: 0;
}
</style>
