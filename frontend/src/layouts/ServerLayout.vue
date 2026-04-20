<template>
  <div class="sl-wrap" :class="{ 'sl-wrap--fullscreen': isTerminal }">
    <!-- 信息头（终端模式下隐藏） -->
    <div v-if="!isTerminal" class="sl-header">
      <div class="sl-info">
        <div class="sl-name-row">
          <span class="status-dot" :class="server?.status" />
          <span class="sl-name">{{ server?.name }}</span>
          <t-tag :theme="statusTheme" variant="light" size="small">{{ statusLabel }}</t-tag>
        </div>
        <span class="sl-host">{{ server?.host }}:{{ server?.port }}</span>
      </div>
      <!-- Tab 导航 -->
      <t-tabs class="sl-tabs" :value="activeTab" @change="onTabChange">
        <t-tab-panel v-for="tab in tabs" :key="tab.value" :value="tab.value" :label="tab.label" />
      </t-tabs>
    </div>

    <!-- 内容区 -->
    <div class="sl-content" :class="{ 'sl-content--fullscreen': isTerminal }">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useServerStore } from '@/stores/server'

const route = useRoute()
const router = useRouter()
const serverStore = useServerStore()

const serverId = computed(() => Number(route.params.serverId))
const server   = computed(() => serverStore.getById(serverId.value))
const activeTab  = computed(() => route.path.split('/').pop() || 'overview')
const isTerminal = computed(() => activeTab.value === 'terminal')

const statusTheme = computed(() => {
  const s = server.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline') return 'danger'
  return 'default'
})
const statusLabel = computed(() => {
  const s = server.value?.status ?? ''
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string,string>)[s] ?? s
})

const tabs = [
  { value: 'overview',  label: '概览' },
  { value: 'nginx',     label: 'Nginx 网关' },
  { value: 'docker',    label: 'Docker' },
  { value: 'system',    label: '系统' },
  { value: 'files',     label: '文件' },
  { value: 'terminal',  label: '终端' },
]

function onTabChange(val: string | number) {
  router.push(`/servers/${serverId.value}/${val}`)
}

onMounted(async () => {
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.sl-wrap { height: 100%; display: flex; flex-direction: column; }
.sl-wrap--fullscreen { overflow: hidden; }

.sl-header {
  background: var(--sh-card-bg);
  border-bottom: 1px solid var(--sh-border);
  flex-shrink: 0;
}

.sl-info {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 24px 0;
}
.sl-name-row { display: flex; align-items: center; gap: 8px; }
.sl-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.sl-host { font-size: 13px; color: var(--sh-text-secondary); }

.sl-tabs {
  margin-top: 4px;
  padding: 0 16px;
}
.sl-tabs :deep(.t-tabs__nav) { border-bottom: none; }

.sl-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}
.sl-content--fullscreen {
  overflow: hidden;
  padding: 0;
}
</style>
