import { get, post, put, del } from './http'

// ---------- AI 预测 ----------
// 调用后端接口，API Key 保存在服务器环境变量中，前端不暴露
export async function apiAIPredict(type) {
  const r = await post('/api/ai/predict', { type: type || 'ssq' })
  if (!r.ok) throw new Error(r.data.msg || 'AI 预测失败')
  const isDlt = type === 'dlt'
  const fmt = (arr, range) => {
    if (!Array.isArray(arr)) return []
    return arr.map((n) => String(parseInt(n, 10)).padStart(2, '0')).filter((n) => {
      const v = parseInt(n, 10)
      return v >= 1 && v <= range
    })
  }
  return {
    red: fmt(r.data.red, isDlt ? 35 : 33),
    blue: fmt(r.data.blue, isDlt ? 12 : 16),
    reason: r.data.reason || '',
  }
}

// ---------- 用户 ----------
export const apiRegister = (username, password, email, deviceId) =>
  post('/api/register', { username, password, email, device_id: deviceId })

export const apiLogin = (username, password) =>
  post('/api/login', { username, password })

export const apiDeviceLogin = (deviceId, email, deviceModel) =>
  post('/api/login/device', { device_id: deviceId, email, device_model: deviceModel || 'Web' })

export const apiMe = () => get('/api/me')
export const apiUpdateMe = (payload) => put('/api/me', payload)
export const apiDeleteMe = () => del('/api/me')

// ---------- 彩票管理 ----------
export const apiListLottery = (params = {}) => {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v) q.append(k, v)
  }
  const s = q.toString()
  return get('/api/lottery' + (s ? '?' + s : ''))
}

export const apiCreateLottery = (payload) => post('/api/lottery', payload)
export const apiBatchLottery = (items) => post('/api/lottery/batch', { items })
export const apiDeleteLottery = (id) => del('/api/lottery/' + id)
export const apiUpdateStatus = (id, status) =>
  put('/api/lottery/' + id + '/status', { status })

// ---------- 开奖 / 拉奖 / 核对 ----------
export const apiListDraw = (type) => get('/api/draw?type=' + (type || ''))
export const apiPull = () => post('/api/admin/pull')
export const apiCheck = () => post('/api/admin/check')

// ---------- 图片识别 ----------
// dryRun=true 时只解析返回号码、不入库，便于在录入页预填选球后由用户确认保存
export const apiRecognize = (file, dryRun = false) => {
  const fd = new FormData()
  fd.append('image', file)
  if (dryRun) fd.append('dry_run', '1')
  return post('/api/lottery/recognize', fd, true)
}

// ---------- 分享 ----------
// 创建分享链接（需登录）
export const apiCreateShare = (ids) => post('/api/share', { ids })
// 查看分享（公开，无需登录）
export const apiGetShare = (code) => get('/api/share?code=' + code)
// 前端 Web 站点域名（分享链接统一前缀）
export { WEB_BASE_URL } from './http'
