import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  base: '/panel/',
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
    // CodeMirror 6 的各子包都依赖 @codemirror/state 等核心包；
    // 若 bun hoist 出多份会导致 `instanceof` 失败、抛出
    // "Unrecognized extension value in extension set"。强制去重。
    dedupe: [
      '@codemirror/state',
      '@codemirror/view',
      '@codemirror/language',
      '@codemirror/commands',
      '@codemirror/search',
      '@codemirror/autocomplete',
      '@lezer/common',
      '@lezer/highlight',
      '@lezer/lr',
    ],
  },
  server: {
    proxy: {
      '/panel/api': {
        target: 'http://localhost:9999',
        changeOrigin: true,
        ws: true,
      },
      '/panel/webhooks': {
        target: 'http://localhost:9999',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../backend/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue', 'vue-router', 'pinia'],
          'vendor-ui': ['tdesign-vue-next', 'tdesign-icons-vue-next'],
          'vendor-charts': ['echarts'],
        },
      },
    },
  },
})
