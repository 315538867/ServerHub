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
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        // Split heavy vendors so changes in one don't bust caches for others
        // and parallel HTTP fetches let large bundles arrive faster.
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('echarts') || id.includes('zrender') || id.includes('tslib')) return 'vendor-charts'
          if (id.includes('xterm')) return 'vendor-xterm'
          if (id.includes('@codemirror') || id.includes('@lezer')) return 'vendor-codemirror'
          if (id.includes('lucide-vue-next')) return 'vendor-icons'
          if (id.includes('naive-ui') || id.includes('vueuc') || id.includes('@css-render') || id.includes('seemly')) return 'vendor-ui'
          if (id.includes('vue-router') || id.includes('pinia') || id.includes('/vue/')) return 'vendor-vue'
        },
      },
    },
  },
})
