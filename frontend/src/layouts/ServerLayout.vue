<template>
  <div class="server-layout" :class="{ 'server-layout--fullscreen': isTerminal }">
    <div v-if="!isTerminal" class="server-layout-header">
      <div class="server-info">
        <h3 class="server-name">{{ server?.name }}</h3>
        <t-tag v-if="server" :theme="statusTheme" variant="light" size="small">{{ statusLabel }}</t-tag>
        <span v-if="server" class="server-host">{{ server.host }}:{{ server.port }}</span>
      </div>
    </div>
    <t-tabs :value="activeTab" @change="onTabChange" :class="{ 'tabs-compact': isTerminal }">
      <t-tab-panel v-for="tab in tabs" :key="tab.value" :value="tab.value" :label="tab.label" />
    </t-tabs>
    <div class="server-layout-content" :class="{ 'content--fullscreen': isTerminal }">
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
const server = computed(() => serverStore.getById(serverId.value))

const statusTheme = computed(() => {
  const s = server.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline') return 'danger'
  return 'default'
})
const statusLabel = computed(() => {
  const s = server.value?.status ?? ''
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s] ?? s
})

const tabs = [
  { value: 'overview', label: '概览' },
  { value: 'nginx', label: 'Nginx 网关' },
  { value: 'docker', label: 'Docker' },
  { value: 'system', label: '系统' },
  { value: 'files', label: '文件' },
  { value: 'terminal', label: '终端' },
]

const activeTab = computed(() => route.path.split('/').pop() || 'overview')
const isTerminal = computed(() => activeTab.value === 'terminal')

function onTabChange(val: string | number) {
  router.push(`/servers/${serverId.value}/${val}`)
}

onMounted(async () => {
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.server-layout { height: 100%; }
.server-layout--fullscreen { display: flex; flex-direction: column; }
.server-layout-header { margin-bottom: 12px; }
.server-info { display: flex; align-items: center; gap: 10px; }
.server-name { margin: 0; font-size: 18px; font-weight: 600; color: var(--td-text-color-primary); }
.server-host { color: var(--td-text-color-secondary); font-size: 13px; }
.server-layout-content { margin-top: 12px; }
.content--fullscreen { flex: 1; overflow: hidden; margin-top: 0; }
.tabs-compact { margin-bottom: 0; }
</style>
