<template>
  <div class="server-layout" :class="{ 'server-layout--fullscreen': isTerminal }">
    <div v-if="!isTerminal" class="server-layout-header">
      <div class="server-info">
        <h3>{{ server?.name }}</h3>
        <el-tag v-if="server" :type="statusType" size="small">{{ server.status }}</el-tag>
        <span v-if="server" class="server-host">{{ server.host }}:{{ server.port }}</span>
      </div>
    </div>
    <el-tabs v-model="activeTab" @tab-click="onTabClick" :class="{ 'tabs-compact': isTerminal }">
      <el-tab-pane v-for="tab in tabs" :key="tab.name" :label="tab.label" :name="tab.name" />
    </el-tabs>
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

const statusType = computed(() => {
  const s = server.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline') return 'danger'
  return 'info'
})

const tabs = [
  { name: 'overview', label: '概览' },
  { name: 'nginx', label: 'Nginx 网关' },
  { name: 'docker', label: 'Docker' },
  { name: 'system', label: '系统' },
  { name: 'files', label: '文件' },
  { name: 'terminal', label: '终端' },
]

const activeTab = computed({
  get: () => route.path.split('/').pop() || 'overview',
  set: () => {},
})

const isTerminal = computed(() => activeTab.value === 'terminal')

function onTabClick(tab: any) {
  router.push(`/servers/${serverId.value}/${tab.paneName}`)
}

onMounted(async () => {
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.server-layout { height: 100%; }
.server-layout--fullscreen { display: flex; flex-direction: column; }
.server-layout-header { margin-bottom: 8px; }
.server-info { display: flex; align-items: center; gap: 12px; }
.server-info h3 { margin: 0; font-size: 18px; }
.server-host { color: var(--el-text-color-secondary); font-size: 13px; }
.server-layout-content { margin-top: 8px; }
.content--fullscreen { flex: 1; overflow: hidden; margin-top: 0; }
.tabs-compact { margin-bottom: 0; }
</style>
