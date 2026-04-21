<template>
  <div class="logs-page">
    <UiCard padding="none">
      <div class="logs-toolbar">
        <NRadioGroup v-model:value="activeSource" size="small" @update:value="switchSource">
          <NRadioButton v-if="app?.container_name" value="container">容器日志</NRadioButton>
          <NRadioButton v-if="app?.site_name" value="nginx_access">Nginx 访问</NRadioButton>
          <NRadioButton v-if="app?.site_name" value="nginx_error">Nginx 错误</NRadioButton>
        </NRadioGroup>
        <UiButton variant="secondary" size="sm" :disabled="!activeSource" @click="reconnect">
          <template #icon><RefreshCw :size="14" /></template>
          重连
        </UiButton>
      </div>
    </UiCard>

    <UiCard padding="none" class="logs-card">
      <div v-if="activeSource" ref="logsEl" class="logs-terminal" />
      <EmptyBlock v-else description="该应用未关联容器或 Nginx 站点，无日志可查看" />
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NRadioGroup, NRadioButton } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { getContainers, containerLogsWsUrl } from '@/api/docker'
import { accessLogsWsUrl, errorLogsWsUrl } from '@/api/nginx'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const route = useRoute()
const auth = useAuthStore()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const serverId = computed(() => app.value?.server_id ?? 0)

const logsEl = ref<HTMLDivElement>()
let term: Terminal | null = null
let ws: WebSocket | null = null

const activeSource = ref('')

function defaultSource() {
  if (app.value?.container_name) return 'container'
  if (app.value?.site_name) return 'nginx_access'
  return ''
}

async function startStream() {
  if (!logsEl.value || !serverId.value || !activeSource.value) return
  term?.dispose()
  term = new Terminal({
    theme: { background: '#0A0A0A', foreground: '#E4E4E7' },
    convertEol: true, fontSize: 12,
    fontFamily: 'ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, monospace',
  })
  const fit = new FitAddon(); term.loadAddon(fit); term.open(logsEl.value); fit.fit()
  ws?.close()

  if (activeSource.value === 'container' && app.value?.container_name) {
    const containers = await getContainers(serverId.value)
    const c = containers.find(c => c.names.includes(app.value!.container_name))
    if (!c) { term.writeln('\x1b[31m[找不到容器]\x1b[0m'); return }
    ws = new WebSocket(containerLogsWsUrl(serverId.value, c.id, auth.token))
  } else if (activeSource.value === 'nginx_access') {
    ws = new WebSocket(accessLogsWsUrl(serverId.value, auth.token))
  } else if (activeSource.value === 'nginx_error') {
    ws = new WebSocket(errorLogsWsUrl(serverId.value, auth.token))
  }

  if (!ws) return
  ws.onmessage = (e) => {
    try { const msg = JSON.parse(e.data); if (msg.type === 'output') term?.writeln(msg.data) } catch { /* ignore */ }
  }
  ws.onerror = () => term?.writeln('\x1b[31m[连接错误]\x1b[0m')
}

async function switchSource() {
  cleanup()
  await nextTick()
  await startStream()
}

function reconnect() { cleanup(); nextTick(() => startStream()) }

function cleanup() { ws?.close(); ws = null; term?.dispose(); term = null }

watch(() => app.value?.container_name, () => {
  if (!activeSource.value) activeSource.value = defaultSource()
})

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  activeSource.value = defaultSource()
  await nextTick()
  await startStream()
})
onBeforeUnmount(() => cleanup())
</script>

<style scoped>
.logs-page {
  padding: var(--space-6);
  display: flex; flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  min-height: 0;
}
.logs-toolbar {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  flex-wrap: wrap;
}
.logs-card {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.logs-terminal {
  width: 100%;
  height: 100%;
  min-height: 360px;
  background: #0A0A0A;
  padding: var(--space-3);
}
</style>
