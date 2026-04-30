<template>
  <div class="net-topo">
    <div class="topo-header">
      <span class="topo-title">请求链路拓扑</span>
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

      <!-- 域名 -->
      <div class="topo-node" :class="domainNodeClass">
        <div class="node-icon">🌐</div>
        <div class="node-title">{{ displayDomain || '未配置' }}</div>
        <div class="node-sub">{{ app?.expose_mode === 'site' ? '独立域名' : '路径前缀' }}</div>
      </div>

      <div class="topo-arrow"><span class="arrow-label">TLS/80</span></div>

      <!-- Nginx Edge -->
      <div class="topo-node" :class="edgeNodeClass">
        <div class="node-icon">⚙️</div>
        <div class="node-title">{{ edgeServerName }}</div>
        <div class="node-sub">Nginx Edge</div>
      </div>

      <!-- 动态路由 -->
      <template v-for="(rt, i) in displayRoutes" :key="rt.path">
        <div class="topo-arrow">
          <span class="arrow-label">{{ rt.path }}</span>
        </div>
        <div class="topo-node" :class="rt.nodeClass">
          <div class="node-icon">📦</div>
          <div class="node-title">{{ rt.label }}</div>
          <div class="node-sub">{{ rt.sub }}</div>
        </div>
      </template>

      <!-- 无路由时的占位 -->
      <template v-if="displayRoutes.length === 0">
        <div class="topo-arrow"><span class="arrow-label">—</span></div>
        <div class="topo-node node--missing">
          <div class="node-icon">📦</div>
          <div class="node-title">待配置</div>
          <div class="node-sub">尚无路由规则</div>
        </div>
      </template>
    </div>

    <!-- 检查项 -->
    <div class="topo-checks">
      <div
        v-for="c in checks"
        :key="c.label"
        class="check-item"
        :class="`check-item--${c.status}`"
      >
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
import type { AppIngress } from '@/api/application'

const props = defineProps<{
  appId: number
  ingresses?: AppIngress[]
}>()

const appStore = useAppStore()
const serverStore = useServerStore()
const app = computed(() => appStore.getById(props.appId))

const modeLabel = computed(() => {
  const m = app.value?.expose_mode
  if (m === 'site') return '独立站点模式'
  if (m === 'path') return '路径转发模式'
  return '未暴露'
})

// 从 ingress 数据派生域名
const displayDomain = computed(() => {
  if (props.ingresses?.length) {
    const applied = props.ingresses.find(ig => ig.status === 'applied')
    if (applied) return applied.domain
    const draft = props.ingresses.find(ig => ig.status === 'draft')
    if (draft) return `${draft.domain} (draft)`
  }
  return app.value?.domain || ''
})

// Edge server 名称
const edgeServerName = computed(() => {
  if (props.ingresses?.length) {
    const first = props.ingresses[0]
    return first.edge_server_name || `Edge #${first.edge_server_id}`
  }
  const s = serverStore.getById(app.value?.server_id ?? -1)
  return s ? s.name : '—'
})

interface DisplayRoute {
  path: string
  label: string
  sub: string
  nodeClass: string
}

const displayRoutes = computed<DisplayRoute[]>(() => {
  if (!props.ingresses?.length) return []
  const routes: DisplayRoute[] = []
  for (const ig of props.ingresses) {
    for (const r of ig.matching_routes || []) {
      const up = r.upstream
      let label = ''
      let sub = ''
      if (up.type === 'service' && up.service_id) {
        label = `Service #${up.service_id}`
        sub = up.override_host || up.override_port ? `${up.override_host || ''}:${up.override_port || ''}` : '自动解析'
      } else if (up.type === 'raw') {
        label = up.raw_url || '—'
        sub = '直连地址'
      }
      routes.push({
        path: r.path,
        label,
        sub,
        nodeClass: ig.status === 'applied' ? 'node--ok' : ig.status === 'draft' ? 'node--warn' : 'node--missing',
      })
    }
  }
  return routes
})

const domainNodeClass = computed(() =>
  displayDomain.value ? 'node--ok' : 'node--missing',
)
const edgeNodeClass = computed(() =>
  props.ingresses?.length ? 'node--ok' : 'node--warn',
)

interface Check {
  label: string
  value: string
  status: 'ok' | 'warn' | 'missing'
}

const checks = computed<Check[]>(() => {
  const a = app.value
  if (!a) return []
  const list: Check[] = []

  list.push({
    label: '暴露模式',
    value: modeLabel.value,
    status: a.expose_mode === 'none' ? 'warn' : 'ok',
  })

  if (a.access_url) {
    list.push({
      label: '访问入口',
      value: a.access_url,
      status: 'ok',
    })
  }

  list.push({
    label: 'Ingress 数',
    value: `${a.ingress_count || props.ingresses?.length || 0} 个`,
    status: (a.ingress_count || props.ingresses?.length) ? 'ok' : 'warn',
  })

  list.push({
    label: 'Service 数',
    value: `${a.service_count || 0} 个`,
    status: a.service_count ? 'ok' : 'warn',
  })

  list.push({
    label: '状态',
    value: a.status || 'unknown',
    status: a.status === 'running' ? 'ok' : a.status === 'error' ? 'missing' : 'warn',
  })

  return list
})
</script>

<style scoped>
.net-topo {
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-4) var(--space-5);
  margin: 0 0 var(--space-4);
}
.topo-header {
  display: flex;
  align-items: baseline;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}
.topo-title { font-size: var(--fs-md); font-weight: var(--fw-semibold); color: var(--ui-fg); }
.topo-mode  { font-size: var(--fs-xs); color: var(--ui-fg-3); }

.topo-flow {
  display: flex;
  align-items: stretch;
  gap: 0;
  overflow-x: auto;
  padding: var(--space-2) var(--space-1) var(--space-4);
}
.topo-flow::-webkit-scrollbar { height: 4px; }
.topo-flow::-webkit-scrollbar-thumb { background: var(--ui-border); border-radius: 2px; }

.topo-node {
  flex: 0 0 auto;
  min-width: 140px;
  max-width: 200px;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3);
  background: var(--ui-bg-1);
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
  position: relative;
}
.node--client {
  background: color-mix(in srgb, var(--ui-brand) 6%, transparent);
  border-color: color-mix(in srgb, var(--ui-brand) 30%, transparent);
}
.node--ok      { border-color: color-mix(in srgb, var(--ui-success) 50%, transparent); background: color-mix(in srgb, var(--ui-success) 6%, transparent); }
.node--warn    { border-color: color-mix(in srgb, var(--ui-warning) 50%, transparent); background: color-mix(in srgb, var(--ui-warning) 6%, transparent); }
.node--missing { border-color: color-mix(in srgb, var(--ui-danger) 40%, transparent); background: color-mix(in srgb, var(--ui-danger) 6%, transparent); border-style: dashed; }

.node-icon { font-size: 20px; line-height: 1; }
.node-title {
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  font-family: var(--font-mono);
  word-break: break-all;
  line-height: 1.3;
}
.node-sub {
  font-size: var(--fs-xs);
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
  min-width: 60px;
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
  background: var(--ui-bg-2);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  position: relative;
  z-index: 1;
  border: 1px solid var(--ui-border);
  font-family: var(--font-mono);
  max-width: 56px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topo-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--space-2) var(--space-4);
  padding-top: var(--space-4);
  border-top: 1px dashed var(--ui-border);
}
.check-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-xs);
  padding: var(--space-1) 0;
  min-width: 0;
}
.check-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--ui-fg-4);
  flex-shrink: 0;
}
.check-item--ok .check-dot      { background: var(--ui-success); }
.check-item--warn .check-dot    { background: var(--ui-warning); }
.check-item--missing .check-dot { background: var(--ui-danger); }
.check-label {
  color: var(--ui-fg-3);
  flex-shrink: 0;
  min-width: 64px;
}
.check-value {
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
</style>
