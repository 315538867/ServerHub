<template>
  <div class="al">
    <div class="al__bar">
      <div class="al__bar-main">
        <div class="al__title-row">
          <StatusDot :status="app?.status ?? 'unknown'" :size="10" :ring="true" :pulse="app?.status === 'running'" />
          <h1 class="al__title">{{ app?.name ?? '加载中…' }}</h1>
          <UiBadge v-if="statusLabel" :tone="statusTone">{{ statusLabel }}</UiBadge>
          <UiBadge v-if="exposeModeLabel" tone="info" variant="soft">{{ exposeModeLabel }}</UiBadge>
        </div>
        <div class="al__meta">
          <span v-if="server" class="al__sub">
            <Server :size="13" /> {{ server.name }}
          </span>
          <span v-if="app?.access_url" class="al__sep">·</span>
          <code v-if="app?.access_url" class="al__code al__access-url">{{ app.access_url }}</code>
        </div>
      </div>
      <div id="app-bar-actions" class="al__bar-actions">
        <UiButton
          v-if="app?.access_url"
          size="sm"
          variant="primary"
          @click="openAccessUrl"
        >
          <template #icon><Globe :size="14" /></template>
          访问
        </UiButton>
        <UiButton
          v-if="app?.container_name"
          size="sm"
          variant="secondary"
          @click="router.push(`/apps/${appId}/ops/terminal`)"
        >
          <template #icon><Terminal :size="14" /></template>
          终端
        </UiButton>
        <UiButton
          v-if="app?.container_name"
          size="sm"
          variant="secondary"
          @click="router.push(`/apps/${appId}/ops/logs`)"
        >
          <template #icon><FileText :size="14" /></template>
          日志
        </UiButton>
        <UiButton
          v-if="app?.expose_mode !== 'none'"
          size="sm"
          variant="secondary"
          @click="router.push(`/apps/${appId}/network/ingresses`)"
        >
          <template #icon><Route :size="14" /></template>
          路由
        </UiButton>
      </div>
    </div>

    <UiTabs
      class="al__tabs"
      :model-value="activeTab"
      :items="tabsOptions"
      @update:model-value="onTabChange"
    />

    <div class="al__content" :class="{ 'al__content--terminal': isTerminal, 'al__content--nested': isNested }">
      <router-view v-slot="{ Component, route: r }">
        <transition name="al-fade" mode="out-in">
          <component :is="Component" :key="r.fullPath" />
        </transition>
      </router-view>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Server, Globe, Terminal, FileText, Route } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import StatusDot from '@/components/ui/StatusDot.vue'
import UiTabs from '@/components/ui/UiTabs.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()

const appId  = computed(() => Number(route.params.appId))
const app    = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

function openAccessUrl() {
  if (app.value?.access_url) window.open(app.value.access_url, '_blank', 'noopener')
}

const activeTab = computed(() => route.path.split('/').filter(Boolean)[2] || 'overview')
const activeSub = computed(() => route.path.split('/').filter(Boolean)[3] || '')
const isNested = computed(() => activeTab.value === 'network' || activeTab.value === 'ops')
const isTerminal = computed(() => activeTab.value === 'ops' && activeSub.value === 'terminal')

// R3 起 app.status 由后端 derive.AppStatus 派生,枚举改为 running|syncing|error|unknown。
const statusLabel = computed(() => {
  const s = app.value?.status ?? ''
  return ({ running: '运行中', syncing: '同步中', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? ''
})
const statusTone = computed<'success' | 'neutral' | 'danger' | 'warning'>(() => {
  const s = app.value?.status ?? ''
  if (s === 'running') return 'success'
  if (s === 'syncing') return 'warning'
  if (s === 'error') return 'danger'
  return 'neutral'
})
const exposeModeLabel = computed(() => {
  const m = app.value?.expose_mode ?? ''
  if (!m || m === 'none') return ''
  return ({ site: '独立站点', path: '路径转发' } as Record<string, string>)[m] ?? m
})

const tabs = computed(() => {
  const a = app.value
  const hasNetwork = (a?.expose_mode && a.expose_mode !== 'none') || !!a?.site_name
  return [
    { value: 'overview', label: '总览' },
    { value: 'services', label: 'Services' },
    ...(hasNetwork ? [{ value: 'network', label: '网络' }] : []),
    { value: 'ops',      label: '运维' },
    ...(a?.db_conn_id ? [{ value: 'data', label: '数据' }] : []),
  ]
})
const tabsOptions = computed(() => tabs.value.map(t => ({ value: t.value, label: t.label })))

function onTabChange(val: string | number) {
  router.push(`/apps/${appId.value}/${val}`)
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.al {
  height: 100%;
  display: flex; flex-direction: column;
  background: var(--ui-bg);
  min-height: 0;
}

.al__bar {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-5) var(--space-8) var(--space-3);
}
.al__bar-main {
  display: flex; flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}
.al__bar-actions {
  display: flex; align-items: center;
  gap: var(--space-2);
  flex-shrink: 0;
}
.al__title-row {
  display: flex; align-items: center; gap: var(--space-2);
}
.al__title {
  font-size: var(--fs-xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.01em;
  margin: 0;
}
.al__meta {
  display: flex; align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
}
.al__sub { display: inline-flex; align-items: center; gap: 6px; }
.al__sep { color: var(--ui-fg-4); }
.al__code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
  color: var(--ui-fg-2);
}

.al__tabs { margin: 0; padding: 0 var(--space-8); }

.al__content {
  flex: 1; min-height: 0;
  overflow-y: auto;
}
.al__content--nested { overflow: hidden; display: flex; flex-direction: column; }
.al__content--terminal { overflow: hidden; }

.al-fade-enter-active {
  transition: opacity var(--dur-base) var(--ease), transform var(--dur-base) var(--ease);
}
.al-fade-enter-from { opacity: 0; transform: translateY(4px); }
.al-fade-leave-active { transition: opacity var(--dur-fast) var(--ease); }
.al-fade-leave-to { opacity: 0; }
</style>
