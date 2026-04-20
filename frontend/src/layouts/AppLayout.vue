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
    <div class="al-content" :class="{ 'al-content--terminal': isTerminal, 'al-content--nested': isNested }">
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

// 5-Tab 父级识别：/apps/:id/<tab>/<sub?> → 取第 4 段
const activeTab = computed(() => {
  const segs = route.path.split('/').filter(Boolean)
  // ['apps', ':id', 'tab', 'sub?']
  return segs[2] || 'overview'
})
const activeSub = computed(() => {
  const segs = route.path.split('/').filter(Boolean)
  return segs[3] || ''
})
const isNested = computed(() => activeTab.value === 'network' || activeTab.value === 'ops')
const isTerminal = computed(() => activeTab.value === 'ops' && activeSub.value === 'terminal')

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

// 5-Tab 扁平化：总览 / 部署 / 网络 / 运维 / 数据
const tabs = computed(() => {
  const a = app.value
  const hasNetwork = (a?.expose_mode && a.expose_mode !== 'none') || !!a?.site_name
  return [
    { value: 'overview', label: '总览' },
    { value: 'deploy',   label: '部署' },
    ...(hasNetwork ? [{ value: 'network', label: '网络' }] : []),
    { value: 'ops',      label: '运维' },
    ...(a?.db_conn_id ? [{ value: 'data', label: '数据' }] : []),
  ]
})

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
  gap: var(--sh-space-md);
  padding: var(--sh-space-md) var(--sh-space-lg) 0;
  flex-wrap: wrap;
}
.al-name-row { display: flex; align-items: center; gap: var(--sh-space-sm); }
.al-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.al-server-link { text-decoration: none; }

.al-tabs { margin-top: var(--sh-space-xs); padding: 0 var(--sh-space-lg); }
.al-tabs :deep(.t-tabs__nav) { border-bottom: none; }

.al-content { flex: 1; overflow-y: auto; min-height: 0; }
.al-content--nested { overflow: hidden; display: flex; flex-direction: column; }
.al-content--terminal { overflow: hidden; }
</style>
