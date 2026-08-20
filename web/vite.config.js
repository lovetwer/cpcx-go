import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 生产构建直接输出到后端 static/dist，由 Go embed 一起打包成单文件。
// base 用相对路径，保证在任何挂载路径下资源都能正确加载。
// dev 模式通过 proxy 把 /api、/lottery、/health 转发到本地 Go 后端（默认 8080）。
export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/lottery/ai-generate': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
})
