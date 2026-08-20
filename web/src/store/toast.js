import { reactive } from 'vue'

let seq = 0
export const toasts = reactive([])

export function toast(message, type = 'info', timeout = 2600) {
  const id = ++seq
  toasts.push({ id, message, type })
  setTimeout(() => dismiss(id), timeout)
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i !== -1) toasts.splice(i, 1)
}
