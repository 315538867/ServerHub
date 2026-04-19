<template>
  <div class="app-layout">
    <div class="app-layout-header">
      <div class="app-info">
        <h3>{{ app?.name }}</h3>
        <el-tag v-if="app" :type="statusType" size="small">{{ app.status }}</el-tag>
        <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="server-link">
          <el-tag size="small" type="info">{{ server.name }}</el-tag>
        </router-link>
      </div>
    </div>
    <el-tabs v-model="activeTab" @tab-click="onTabClick">
      <el-tab-pane v-for="tab in tabs" :key="tab.name" :label="tab.label" :name="tab.name" />
    </el-tabs>
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

const statusType = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'info'
})

const tabs = [
  { name: 'overview', label: '概览' },
  { name: 'domain', label: '域名' },
  { name: 'service', label: '服务' },
  { name: 'deploy', label: '部署' },
  { name: 'logs', label: '日志' },
  { name: 'database', label: '数据库' },
  { name: 'env', label: '环境变量' },
]

const activeTab = computed({
  get: () => route.path.split('/').pop() || 'overview',
  set: () => {},
})

function onTabClick(tab: any) {
  router.push(`/apps/${appId.value}/${tab.paneName}`)
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.app-layout-header { margin-bottom: 8px; }
.app-info { display: flex; align-items: center; gap: 12px; }
.app-info h3 { margin: 0; font-size: 18px; }
.server-link { text-decoration: none; }
.server-link:hover .el-tag { opacity: 0.8; }
.app-layout-content { margin-top: 8px; }
</style>
