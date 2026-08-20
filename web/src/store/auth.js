import { reactive } from 'vue'

const TOKEN_KEY = 'lm_token'
const USER_KEY = 'lm_user'
const DEVICE_KEY = 'lm_device'

function loadUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
  } catch {
    return null
  }
}

export const auth = reactive({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: loadUser(),
})

export function setAuth(token, user) {
  auth.token = token
  auth.user = user
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearAuth() {
  auth.token = ''
  auth.user = null
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export function isAuthed() {
  return !!auth.token
}

// 设备号：浏览器里用持久化的随机串模拟设备 ID，实现“一键登录”
export function getDeviceId() {
  let d = localStorage.getItem(DEVICE_KEY)
  if (!d) {
    d = 'web-' + Math.random().toString(36).slice(2, 10) + Date.now().toString(36)
    localStorage.setItem(DEVICE_KEY, d)
  }
  return d
}
