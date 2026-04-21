<template>
  <div class="net-topo">
    <div class="topo-header">
      <span class="topo-title">网络拓扑</span>
      <span class="topo-mode">{{ modeLabel }}</span>
    </div>

    <div class="topo-flow">
      <!-- 用户 -->
      <div class="topo-node node--client">
        <div class="node-icon">👤</div>
        <div class="node-title">用户</div>
        <div class="node-sub">浏览器 / 客户端</div>
      </div>

      <div class="topo-arrow"><span class="arrow-label">HTTPS</span></div>

      <!-- 域名（独立站点模式才显示） -->
      <template v-if="app?.expose_mode === 'site'">
        <div class="topo-node" :class="domainNodeClass">
          <div class="node-icon">🌐</div>
          <div class="node-title">{{ app?.domain || '未配置' }}</div>
          <div class="node-sub">DNS A/AAAA 解析</div>
        </div>
        <div class="topo-arrow"><span class="arrow-label">TCP 443/80</span></div>
      </template>

      <!-- Nginx -->
      <div class="topo-node" :class="nginxNodeClass">
        <div class="node-icon">⚙️</div>
        <div class="node-title">Nginx</div>
        <div class="node-sub">{{ app?.site_name || '未关联站点' }}</div>
      </div>

      <div class="topo-arrow"><span class="arrow-label">proxy_pass</span></div>

      <!-- 容器 -->
      <div class="topo-node" :class="containerNodeClass">
        <div class="node-icon">🐳</div>
        <div class="node-title">{{ app?.container_name || '未绑定容器' }}</div>
        <div class="node-sub">{{ serverName }}</div>
      </div>
    </div>

    <!-- 检查项 -->
    <div class="topo-checks">
      <div v-for="c in checks" :key="c.label" class="check-item" :class="`check-item--${c.status}`">
        <span class="check-dot" />
        <span class="check-label">{{ c.label }}</span>
        <span class="check-value">{{ c.value }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'

const props = defineProps<{ appId: number }>()

const appStore = useAppStore()
const serverStore = useServerStore()
const app = computed(() => appStore.getById(props.appId))

const modeLabel = computed(() => {
  const m = app.value?.expose_mode
  if (m === 'site') return '独立站点（独立 server 块 + 域名）'
  if (m === 'path') return '路径转发（反代到已有 Nginx 站点）'
  return '未通过 Nginx 暴露'
})

const serverName = computed(() => {
  const s = serverStore.getById(app.value?.server_id ?? -1)
  return s ? `${s.name} · ${s.host}` : '—'
})

const domainNodeClass = computed(() => app.value?.domain ? 'node--ok' : 'node--missing')
const nginxNodeClass = computed(() => app.value?.site_name ? 'node--ok' : 'node--warn')
const containerNodeClass = computed(() => app.value?.container_name ? 'node--ok' : 'node--missing')

interface Check { label: string; value: string; status: 'ok' | 'warn' | 'missing' }

const checks = computed<Check[]>(() => {
  const a = app.value
  if (!a) return []
  const list: Check[] = []

  // 暴露模式
  list.push({
    label: '暴露模式',
    value: modeLabel.value,
    status: a.expose_mode === 'none' ? 'warn' : 'ok',
  })

  // 域名
  if (a.expose_mode === 'site') {
    list.push({
      label: '域名',
      value: a.domain || '未配置',
      status: a.domain ? 'ok' : 'missing',
    })
  }

  // 站点
  list.push({
    label: 'Nginx 站点',
    value: a.site_name || '未关联',
    status: a.site_name ? 'ok' : 'warn',
  })

  // 容器
  list.push({
    label: '后端容器',
    value: a.container_name || '未绑定',
    status: a.container_name ? 'ok' : 'missing',
  })

  // 服务器
  list.push({
    label: '所属服务器',
    value: serverName.value,
    status: a.server_id ? 'ok' : 'missing',
  })

  return list
})
</script>

<style scoped>
.net-topo {
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-radius: 10px;
  padding: var(--ui-space-4) var(--ui-space-6) var(--ui-space-4);
  margin: 0 0 var(--ui-space-4);
}
.topo-header {
  display: flex;
  align-items: baseline;
  gap: var(--ui-space-4);
  margin-bottom: var(--ui-space-4);
}
.topo-title { font-size: 14px; font-weight: 600; color: var(--ui-fg); }
.topo-mode  { font-size: 12px; color: var(--ui-fg-3); }

/* 流程 */
.topo-flow {
  display: flex;
  align-items: stretch;
  gap: 0;
  overflow-x: auto;
  padding: var(--ui-space-2) var(--ui-space-1) var(--ui-space-4);
}
.topo-flow::-webkit-scrollbar { height: 4px; }
.topo-flow::-webkit-scrollbar-thumb { background: var(--ui-border); border-radius: 2px; }

.topo-node {
  flex: 0 0 auto;
  min-width: 150px;
  max-width: 220px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  padding: var(--ui-space-2) var(--ui-space-4);
  background: color-mix(in srgb, var(--ui-fg) 2%, transparent);
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: var(--ui-space-1);
  transition: border-color .15s, background .15s;
  position: relative;
}
.node--client {
  background: color-mix(in srgb, var(--ui-brand) 4%, transparent);
  border-color: color-mix(in srgb, var(--ui-brand) 30%, transparent);
}
.node--ok      { border-color: color-mix(in srgb, #67c23a 50%, transparent); background: color-mix(in srgb, #67c23a 4%, transparent); }
.node--warn    { border-color: color-mix(in srgb, #e6a23c 50%, transparent); background: color-mix(in srgb, #e6a23c 4%, transparent); }
.node--missing { border-color: color-mix(in srgb, #e34d59 40%, transparent); background: color-mix(in srgb, #e34d59 4%, transparent); border-style: dashed; }

.node-icon { font-size: 22px; line-height: 1; }
.node-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ui-fg);
  font-family: var(--ui-font-mono, ui-monospace, SFMono-Regular, monospace);
  word-break: break-all;
  line-height: 1.3;
}
.node-sub {
  font-size: 11px;
  color: var(--ui-fg-3);
  word-break: break-all;
  line-height: 1.3;
}

.topo-arrow {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 70px;
  position: relative;
  color: var(--ui-fg-3);
}
.topo-arrow::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 4px; right: 4px;
  height: 1px;
  background: var(--ui-border);
}
.topo-arrow::after {
  content: '';
  position: absolute;
  top: 50%;
  right: 2px;
  width: 0; height: 0;
  border-top: 4px solid transparent;
  border-bottom: 4px solid transparent;
  border-left: 6px solid var(--ui-border);
  transform: translateY(-50%);
}
.arrow-label {
  font-size: 10px;
  background: var(--ui-bg-surface);
  padding: 1px 6px;
  border-radius: 3px;
  position: relative;
  z-index: 1;
  border: 1px solid var(--ui-border);
}

/* 检查项 */
.topo-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--ui-space-2) var(--ui-space-4);
  padding-top: var(--ui-space-4);
  border-top: 1px dashed var(--ui-border);
}
.check-item {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  font-size: 12px;
  padding: var(--ui-space-1) 0;
  min-width: 0;
}
.check-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: #999;
  flex-shrink: 0;
}
.check-item--ok .check-dot      { background: #67c23a; }
.check-item--warn .check-dot    { background: #e6a23c; }
.check-item--missing .check-dot { background: #e34d59; }
.check-label {
  color: var(--ui-fg-3);
  flex-shrink: 0;
  min-width: 70px;
}
.check-value {
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
