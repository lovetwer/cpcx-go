import { auth, clearAuth } from '../store/auth'

// 后端 API 地址（前后端分离部署）
// 默认指向当前生产域名；Vercel 部署时在环境变量里设 VITE_API_BASE_URL
// 为 Render 后端地址（如 https://lottery-api.onrender.com）即可覆盖。
const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'https://cpcxapi.800820882.xyz'

// 前端 Web 站点域名：分享链接统一指向这里（区别于上面的后端 API 域名）。
// 部署到 Vercel 时可通过环境变量 VITE_WEB_BASE_URL 覆盖；默认即生产域名。
export const WEB_BASE_URL = import.meta.env.VITE_WEB_BASE_URL || 'https://cpcx.800820882.xyz'

// 统一请求封装：自动带鉴权头；表单(formData)不手动设 Content-Type（浏览器自动加 boundary）。
// 401 统一清登录态并跳回登录页。
export async function request(method, path, body, isForm = false) {
  const headers = {}
  if (!isForm) headers['Content-Type'] = 'application/json'
  if (auth.token) headers['Authorization'] = 'Bearer ' + auth.token

  const res = await fetch(BASE_URL + path, {
    method,
    headers,
    body: isForm ? body : body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    clearAuth()
    if (location.hash !== '#/login') location.hash = '#/login'
  }

  const data = await res.json().catch(() => ({}))
  return { ok: res.ok, status: res.status, data }
}

export const get = (p) => request('GET', p)
export const post = (p, b, form = false) => request('POST', p, b, form)
export const put = (p, b) => request('PUT', p, b)
export const del = (p) => request('DELETE', p)
