<template>
  <div class="al-wrap">
    <!-- 信息头 + Tab 导航 -->
    <div class="al-header">
      <div class="al-info">
        <div class="al-name-row">
          <span class="status-dot" :class="app?.status" />
          <span class="al-name">{{ app?.name }}</span>
          <t-tag :theme="statusTheme" variant="light" size="small">{{ statusLabel }}</t-tag>
        </div>
        <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="al-server-link">
          <t-tag theme="default" variant="outline" size="small">{{ server.name }}</t-tag>
        </router-link>
      </div>
      <t-tabs class="al-tabs" :value="activeTab" @change="onTabChange">
        <t-tab-panel v-for="tab in tabs" :key="tab.value" :value="tab.value" :label="tab.label" />
      </t-tabs>
    </div>

    <!-- 内容区 -->
    <div class="al-content">
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

const appId  = computed(() => Number(route.params.appId))
const app    = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)
const activeTab = computed(() => route.path.split('/').pop() || 'overview')

const statusTheme = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'default'
})
const statusLabel = computed(() => {
  const s = app.value?.status ?? ''
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string,string>)[s] ?? s
})

const tabs = [
  { value: 'overview', label: '概览' },
  { value: 'domain',   label: '域名' },
  { value: 'service',  label: '服务' },
  { value: 'deploy',   label: '部署' },
  { value: 'logs',     label: '日志' },
  { value: 'database', label: '数据库' },
  { value: 'env',      label: '环境变量' },
]

function onTabChange(val: string | number) {
  router.push(`/apps/${appId.value}/${val}`)
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.al-wrap { height: 100%; display: flex; flex-direction: column; }

.al-header {
  background: var(--sh-card-bg);
  border-bottom: 1px solid var(--sh-border);
  flex-shrink: 0;
}

.al-info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 24px 0;
  flex-wrap: wrap;
}
.al-name-row { display: flex; align-items: center; gap: 8px; }
.al-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.al-server-link { text-decoration: none; }

.al-tabs { margin-top: 4px; padding: 0 16px; }
.al-tabs :deep(.t-tabs__nav) { border-bottom: none; }

.al-content { flex: 1; overflow-y: auto; padding: 20px 24px; }
</style>
