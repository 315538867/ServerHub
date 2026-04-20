<template>
  <div class="logs-page">
    <div class="page-toolbar">
      <t-radio-group v-model="activeSource" @change="switchSource">
        <t-radio-button v-if="app?.container_name" value="container">容器日志</t-radio-button>
        <t-radio-button v-if="app?.site_name" value="nginx_access">Nginx 访问</t-radio-button>
        <t-radio-button v-if="app?.site_name" value="nginx_error">Nginx 错误</t-radio-button>
      </t-radio-group>
      <t-button size="small" variant="outline" @click="reconnect">
        <template #icon><refresh-icon /></template>
        重连
      </t-button>
    </div>
    <div v-if="activeSource" ref="logsEl" class="logs-terminal" />
    <t-empty v-else description="该应用未关联容器或 Nginx 站点，无日志可查看" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { getContainers, containerLogsWsUrl } from '@/api/docker'
import { accessLogsWsUrl, errorLogsWsUrl } from '@/api/nginx'

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
  term = new Terminal({ theme: { background: '#1a2332' }, convertEol: true, fontSize: 13 })
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
.logs-page { padding: 4px 0; display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 12px; }
.logs-terminal { flex: 1; min-height: 400px; background: #1a2332; border-radius: 4px; overflow: hidden; }
</style>
