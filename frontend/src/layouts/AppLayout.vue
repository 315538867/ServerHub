<template>
  <div class="app-layout">
    <div class="app-layout-header">
      <div class="app-info">
        <h3 class="app-name">{{ app?.name }}</h3>
        <t-tag v-if="app" :theme="statusTheme" variant="light" size="small">{{ statusLabel }}</t-tag>
        <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="server-link">
          <t-tag theme="default" variant="outline" size="small">{{ server.name }}</t-tag>
        </router-link>
      </div>
    </div>
    <t-tabs :value="activeTab" @change="onTabChange">
      <t-tab-panel v-for="tab in tabs" :key="tab.value" :value="tab.value" :label="tab.label" />
    </t-tabs>
    <div class="app-layout-content">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

const statusTheme = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'default'
})
const statusLabel = computed(() => {
  const s = app.value?.status ?? ''
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? s
})

const tabs = [
  { value: 'overview', label: '概览' },
  { value: 'domain', label: '域名' },
  { value: 'service', label: '服务' },
  { value: 'deploy', label: '部署' },
  { value: 'logs', label: '日志' },
  { value: 'database', label: '数据库' },
  { value: 'env', label: '环境变量' },
]

const activeTab = computed(() => route.path.split('/').pop() || 'overview')

function onTabChange(val: string | number) {
  router.push(`/apps/${appId.value}/${val}`)
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.app-layout { height: 100%; }
.app-layout-header { margin-bottom: 12px; }
.app-info { display: flex; align-items: center; gap: 10px; }
.app-name { margin: 0; font-size: 18px; font-weight: 600; color: var(--td-text-color-primary); }
.server-link { text-decoration: none; }
.app-layout-content { margin-top: 12px; }
</style>
