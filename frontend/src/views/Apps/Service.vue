<template>
  <div class="page-container">
    <template v-if="app?.container_name && app?.server_id">
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">容器管理</span>
          <t-button variant="outline" size="small" :loading="loading" @click="loadContainers">
            <template #icon><refresh-icon /></template>
            刷新
          </t-button>
        </div>
        <div class="table-wrap">
          <t-table :data="containers" :columns="containerColumns" :loading="loading" row-key="id" stripe size="small">
            <template #status="{ row }">
              <t-tag :theme="stateTheme(row.state)" variant="light" size="small">{{ row.status }}</t-tag>
            </template>
            <template #operations="{ row }">
              <t-space size="small">
                <t-button v-if="row.state !== 'running'" theme="success" size="small" variant="text" :loading="actionLoading === row.id + '_start'" @click="doAction(row, 'start')">启动</t-button>
                <t-button v-if="row.state === 'running'" theme="warning" size="small" variant="text" :loading="actionLoading === row.id + '_stop'" @click="doAction(row, 'stop')">停止</t-button>
                <t-button size="small" variant="text" :loading="actionLoading === row.id + '_restart'" @click="doAction(row, 'restart')">重启</t-button>
                <t-button size="small" variant="text" @click="openLogs(row)">日志</t-button>
                <t-button size="small" variant="text" @click="openInspect(row)">详情</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
      </div>
    </template>
    <div v-else class="section-block empty-block">
      <t-empty description="该应用未关联 Docker 容器，请先在应用设置中配置 container_name" />
    </div>

    <t-drawer v-model:visible="logsVisible" :header="`容器日志 — ${logsContainer}`" size="60%" @closed="onLogsClosed">
      <div ref="logsEl" class="logs-terminal" />
    </t-drawer>

    <t-dialog v-model:visible="inspectVisible" header="容器详情" width="720px" :footer="false">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { getContainers, containerAction, getContainerInspect, containerLogsWsUrl } from '@/api/docker'
import type { ContainerItem } from '@/api/docker'

const route = useRoute()
const auth = useAuthStore()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const serverId = computed(() => app.value?.server_id ?? 0)

const containers = ref<ContainerItem[]>([])
const loading = ref(false)
const actionLoading = ref('')

const containerColumns = [
  { colKey: 'names', title: '名称', minWidth: 140, ellipsis: true },
  { colKey: 'image', title: '镜像', minWidth: 160, ellipsis: true },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'ports', title: '端口', minWidth: 160, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 260, fixed: 'right' as const },
]

const logsVisible = ref(false)
const logsContainer = ref('')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

const inspectVisible = ref(false)
const inspectJson = ref('')

function stateTheme(state: string) {
  return ({ running: 'success', paused: 'warning', exited: 'default' } as Record<string, string>)[state] ?? 'danger'
}

async function loadContainers() {
  if (!serverId.value) return
  loading.value = true
  try { containers.value = await getContainers(serverId.value) }
  finally { loading.value = false }
}

async function doAction(row: ContainerItem, action: 'start' | 'stop' | 'restart') {
  const key = `${row.id}_${action}`
  actionLoading.value = key
  try {
    await containerAction(serverId.value, row.id, action)
    MessagePlugin.success('操作成功')
    await loadContainers()
  } catch { MessagePlugin.error('操作失败') }
  finally { actionLoading.value = '' }
}

function openLogs(row: ContainerItem) {
  logsContainer.value = row.names
  logsVisible.value = true
  nextTick(() => {
    if (!logsEl.value) return
    logsTerm?.dispose()
    logsTerm = new Terminal({ theme: { background: '#1a2332' }, convertEol: true, fontSize: 13 })
    const fit = new FitAddon(); logsTerm.loadAddon(fit); logsTerm.open(logsEl.value); fit.fit()
    logsWs?.close()
    logsWs = new WebSocket(containerLogsWsUrl(serverId.value, row.id, auth.token))
    logsWs.onmessage = (e) => {
      try { const msg = JSON.parse(e.data); if (msg.type === 'output') logsTerm?.writeln(msg.data) } catch { /* ignore */ }
    }
  })
}

function onLogsClosed() { logsWs?.close(); logsWs = null; logsTerm?.dispose(); logsTerm = null }

async function openInspect(row: ContainerItem) {
  try {
    const data = await getContainerInspect(serverId.value, row.id)
    const arr = Array.isArray(data) ? data : [data]
    if (arr[0]?.Config?.Env) {
      arr[0].Config.Env = (arr[0].Config.Env as string[]).map((e: string) =>
        /(?:PASSWORD|SECRET|KEY|TOKEN)=/i.test(e) ? e.replace(/=.*/, '=***') : e)
    }
    inspectJson.value = JSON.stringify(arr[0] ?? data, null, 2)
    inspectVisible.value = true
  } catch { MessagePlugin.error('获取详情失败') }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadContainers()
})
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose() })
</script>

<style scoped>
.table-wrap {
  padding: 0 var(--sh-space-lg) var(--sh-space-md);
}
.empty-block {
  padding: var(--sh-space-xl) var(--sh-space-lg);
  display: flex;
  justify-content: center;
}
:deep(.t-table td) {
  font-size: 13px;
}
.logs-terminal { width: 100%; height: calc(100vh - 120px); background: #1a2332; border-radius: 4px; overflow: hidden; }
.inspect-json { background: var(--td-bg-color-secondarycontainer); border-radius: 4px; padding: var(--sh-space-md); font-size: 12px; line-height: 1.6; overflow: auto; max-height: 70vh; margin: 0; white-space: pre-wrap; word-break: break-all; }
</style>
