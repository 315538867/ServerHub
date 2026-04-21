<template>
  <div class="sl" :class="{ 'sl--fullscreen': isTerminal }">
    <template v-if="!isTerminal">
      <UiStateBanner
        :title="server?.name ?? '加载中…'"
        :status="bannerStatus"
        :status-label="statusLabel"
      >
        <template #subtitle>
          <span class="sl__sub">
            <code class="sl__code">{{ server?.host }}:{{ server?.port }}</code>
          </span>
        </template>
      </UiStateBanner>

      <UiTabs
        class="sl__tabs"
        :model-value="activeTab"
        :items="tabs"
        @update:model-value="onTabChange"
      />
    </template>

    <div class="sl__content" :class="{ 'sl__content--fullscreen': isTerminal }">
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
import UiStateBanner from '@/components/ui/UiStateBanner.vue'
import UiTabs from '@/components/ui/UiTabs.vue'

const route = useRoute()
const router = useRouter()
const serverStore = useServerStore()

const serverId = computed(() => Number(route.params.serverId))
const server   = computed(() => serverStore.getById(serverId.value))
const activeTab  = computed(() => route.path.split('/').pop() || 'overview')
const isTerminal = computed(() => activeTab.value === 'terminal')

const bannerStatus = computed(() => {
  const s = server.value?.status
  if (s === 'online') return 'online'
  if (s === 'offline') return 'offline'
  return 'unknown'
}) as any
const statusLabel = computed(() => {
  const s = server.value?.status ?? ''
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string,string>)[s] ?? '未知'
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
.sl {
  height: 100%;
  display: flex; flex-direction: column;
  padding: var(--ui-space-4) var(--ui-space-5) 0;
  background: var(--ui-bg-canvas);
  min-height: 0;
}
.sl--fullscreen { padding: 0; overflow: hidden; }

.sl__sub { display: inline-flex; align-items: center; gap: var(--ui-space-2); }
.sl__code {
  font-family: var(--ui-font-mono);
  font-size: var(--ui-fs-xs);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  padding: 1px 6px;
  color: var(--ui-fg-2);
}

.sl__tabs { margin: 0 0 var(--ui-space-3); }

.sl__content { flex: 1; min-height: 0; overflow-y: auto; }
.sl__content--fullscreen { overflow: hidden; padding: 0; }

.sl-fade-enter-active {
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard);
}
.sl-fade-leave-active { transition: opacity var(--ui-dur-fast); }
.sl-fade-leave-to { opacity: 0; }
</style>
