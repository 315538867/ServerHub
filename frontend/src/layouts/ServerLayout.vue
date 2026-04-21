<template>
  <div class="sl">
    <div class="sl__bar">
      <div class="sl__title-row">
        <StatusDot :status="server?.status ?? 'unknown'" :size="10" :ring="true" :pulse="server?.status === 'online'" />
        <h1 class="sl__title">{{ server?.name ?? '加载中…' }}</h1>
        <UiBadge v-if="statusLabel" :tone="statusTone">{{ statusLabel }}</UiBadge>
      </div>
      <div class="sl__meta">
        <code class="sl__code">{{ server?.host }}:{{ server?.port }}</code>
      </div>
    </div>

    <UiTabs
      class="sl__tabs"
      :model-value="activeTab"
      :items="tabs"
      @update:model-value="onTabChange"
    />

    <div class="sl__content" :class="{ 'sl__content--terminal': isTerminal }">
      <router-view v-slot="{ Component, route: r }">
        <transition name="sl-fade" mode="out-in">
          <component :is="Component" :key="r.fullPath" />
        </transition>
      </router-view>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useServerStore } from '@/stores/server'
import StatusDot from '@/components/ui/StatusDot.vue'
import UiTabs from '@/components/ui/UiTabs.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const route = useRoute()
const router = useRouter()
const serverStore = useServerStore()

const serverId = computed(() => Number(route.params.serverId))
const server   = computed(() => serverStore.getById(serverId.value))
const activeTab  = computed(() => route.path.split('/').pop() || 'overview')
const isTerminal = computed(() => activeTab.value === 'terminal')

const statusLabel = computed(() => {
  const s = server.value?.status ?? ''
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s] ?? ''
})
const statusTone = computed<'success' | 'neutral' | 'danger'>(() => {
  const s = server.value?.status ?? ''
  if (s === 'online') return 'success'
  if (s === 'offline') return 'neutral'
  return 'neutral'
})

const tabs = [
  { value: 'overview',  label: '概览' },
  { value: 'nginx',     label: 'Nginx 网关' },
  { value: 'docker',    label: 'Docker' },
  { value: 'system',    label: '系统' },
  { value: 'logs-search', label: '日志搜索' },
  { value: 'files',     label: '文件' },
  { value: 'terminal',  label: '终端' },
  { value: 'discover',  label: '发现' },
]

function onTabChange(val: string | number) {
  router.push(`/servers/${serverId.value}/${val}`)
}

onMounted(async () => {
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.sl {
  height: 100%;
  display: flex; flex-direction: column;
  background: var(--ui-bg);
  min-height: 0;
}

.sl__bar {
  display: flex; flex-direction: column;
  gap: var(--space-1);
  padding: var(--space-5) var(--space-8) var(--space-3);
}
.sl__title-row {
  display: flex; align-items: center; gap: var(--space-2);
}
.sl__title {
  font-size: var(--fs-xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.01em;
  margin: 0;
}
.sl__meta {
  display: flex; align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
}
.sl__code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
  color: var(--ui-fg-2);
}

.sl__tabs { margin: 0; padding: 0 var(--space-8); }

.sl__content { flex: 1; min-height: 0; overflow-y: auto; }
.sl__content--terminal {
  overflow: hidden;
  padding: var(--space-4) var(--space-8) var(--space-6);
}

.sl-fade-enter-active {
  transition: opacity var(--dur-base) var(--ease), transform var(--dur-base) var(--ease);
}
.sl-fade-enter-from { opacity: 0; transform: translateY(4px); }
.sl-fade-leave-active { transition: opacity var(--dur-fast) var(--ease); }
.sl-fade-leave-to { opacity: 0; }
</style>
