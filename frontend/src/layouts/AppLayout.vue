<template>
  <div class="al">
    <UiStateBanner
      :title="app?.name ?? '加载中…'"
      :status="bannerStatus"
      :status-label="statusLabel"
    >
      <template #subtitle>
        <span v-if="server" class="al__sub">
          <server-icon /> {{ server.name }}
          <span v-if="app?.image" class="al__sep">·</span>
          <code v-if="app?.image" class="al__code">{{ app.image }}</code>
        </span>
      </template>
      <template #meta v-if="app?.expose_mode && app.expose_mode !== 'none'">
        <span class="al__meta-pill">{{ exposeModeLabel }}</span>
      </template>
      <template #actions>
        <UiKbd>{{ tabs.length }} Tab</UiKbd>
      </template>
    </UiStateBanner>

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
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { ServerIcon } from 'tdesign-icons-vue-next'
import UiStateBanner from '@/components/ui/UiStateBanner.vue'
import UiTabs from '@/components/ui/UiTabs.vue'
import UiKbd from '@/components/ui/UiKbd.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()

const appId  = computed(() => Number(route.params.appId))
const app    = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

const activeTab = computed(() => {
  const segs = route.path.split('/').filter(Boolean)
  return segs[2] || 'overview'
})
const activeSub = computed(() => {
  const segs = route.path.split('/').filter(Boolean)
  return segs[3] || ''
})
const isNested = computed(() => activeTab.value === 'network' || activeTab.value === 'ops')
const isTerminal = computed(() => activeTab.value === 'ops' && activeSub.value === 'terminal')

const bannerStatus = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'online'
  if (s === 'offline') return 'offline'
  if (s === 'error') return 'error'
  return 'unknown'
}) as any

const statusLabel = computed(() => {
  const s = app.value?.status ?? ''
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string,string>)[s] ?? '未知'
})

const exposeModeLabel = computed(() => {
  const m = app.value?.expose_mode ?? ''
  return ({ public: '公网', internal: '内网', custom: '自定义' } as Record<string,string>)[m] ?? m
})

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
  padding: var(--ui-space-4) var(--ui-space-5) 0;
  background: var(--ui-bg-canvas);
  min-height: 0;
}

.al__sub {
  display: inline-flex; align-items: center;
  gap: var(--ui-space-2);
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
}
.al__sep { color: var(--ui-fg-4); }
.al__code {
  font-family: var(--ui-font-mono);
  font-size: var(--ui-fs-xs);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  padding: 1px 6px;
  color: var(--ui-fg-2);
}
.al__meta-pill {
  font-size: var(--ui-fs-2xs);
  font-weight: var(--ui-fw-medium);
  background: var(--ui-brand-soft);
  color: var(--ui-brand);
  padding: 2px 8px;
  border-radius: var(--ui-radius-pill);
  letter-spacing: .04em;
}

.al__tabs {
  margin: 0 0 var(--ui-space-3);
}

.al__content {
  flex: 1; min-height: 0;
  overflow-y: auto;
}
.al__content--nested { overflow: hidden; display: flex; flex-direction: column; }
.al__content--terminal { overflow: hidden; }

.al-fade-enter-active {
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard);
}
.al-fade-leave-active {
  transition: opacity var(--ui-dur-fast) var(--ui-ease-standard);
}
.al-fade-leave-to { opacity: 0; }
</style>
