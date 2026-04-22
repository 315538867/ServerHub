<template>
  <div class="svc-page">
    <template v-if="app?.container_name && app?.server_id">
      <UiCard padding="none">
        <div class="svc-head">
          <span class="svc-title">容器管理</span>
          <UiButton variant="secondary" size="sm" :loading="loading" @click="loadContainers">
            <template #icon><RefreshCw :size="14" /></template>
            刷新
          </UiButton>
        </div>
        <NDataTable
          :columns="columns"
          :data="containers"
          :loading="loading"
          :row-key="(row: ContainerItem) => row.id"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </template>
    <UiCard v-else padding="lg">
      <EmptyBlock description="该应用未关联 Docker 容器，请先在应用设置中配置 container_name" />
    </UiCard>

    <NDrawer v-model:show="logsVisible" :width="720" @after-leave="onLogsClosed">
      <NDrawerContent :title="`容器日志 — ${logsContainer}`" :native-scrollbar="false">
        <div ref="logsEl" class="logs-terminal" />
      </NDrawerContent>
    </NDrawer>

    <NModal v-model:show="inspectVisible" preset="card" title="容器详情" style="width: 720px" :bordered="false">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import { NDataTable, NDrawer, NDrawerContent, NModal, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { getContainers, containerAction, getContainerInspect, containerLogsWsUrl } from '@/api/docker'
import type { ContainerItem } from '@/api/docker'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const route = useRoute()
const auth = useAuthStore()
const appStore = useAppStore()
const message = useMessage()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const serverId = computed(() => app.value?.server_id ?? 0)

const containers = ref<ContainerItem[]>([])
const loading = ref(false)
const actionLoading = ref('')

function stateTone(state: string): any {
  return ({ running: 'success', paused: 'warning', exited: 'neutral' } as Record<string, string>)[state] ?? 'danger'
}

const columns = computed<DataTableColumns<ContainerItem>>(() => [
  { title: '名称', key: 'names', minWidth: 160, ellipsis: { tooltip: true } },
  { title: '镜像', key: 'image', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'status', width: 140,
    render: (row) => h(UiBadge, { tone: stateTone(row.state) }, () => row.status),
  },
  { title: '端口', key: 'ports', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'operations', width: 280, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      row.state !== 'running' ? h(UiButton, {
        variant: 'ghost', size: 'sm',
        loading: actionLoading.value === `${row.id}_start`,
        onClick: () => doAction(row, 'start'),
      }, () => '启动') : null,
      row.state === 'running' ? h(UiButton, {
        variant: 'ghost', size: 'sm',
        loading: actionLoading.value === `${row.id}_stop`,
        onClick: () => doAction(row, 'stop'),
      }, () => '停止') : null,
      h(UiButton, {
        variant: 'ghost', size: 'sm',
        loading: actionLoading.value === `${row.id}_restart`,
        onClick: () => doAction(row, 'restart'),
      }, () => '重启'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openLogs(row) }, () => '日志'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openInspect(row) }, () => '详情'),
    ]),
  },
])

const logsVisible = ref(false)
const logsContainer = ref('')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

const inspectVisible = ref(false)
const inspectJson = ref('')

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
    message.success('操作成功')
    await loadContainers()
  } catch { message.error('操作失败') }
  finally { actionLoading.value = '' }
}

function openLogs(row: ContainerItem) {
  logsContainer.value = row.names
  logsVisible.value = true
  nextTick(() => {
    if (!logsEl.value) return
    logsTerm?.dispose()
    logsTerm = new Terminal({
      theme: { background: '#0A0A0A', foreground: '#E4E4E7' },
      convertEol: true, fontSize: 12,
    })
    const fit = new FitAddon(); logsTerm.loadAddon(fit); logsTerm.open(logsEl.value); fit.fit()
    logsWs?.close()
    logsWs = new WebSocket(containerLogsWsUrl(serverId.value, row.id), ['bearer', auth.token ?? ''])
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
  } catch { message.error('获取详情失败') }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadContainers()
})
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose() })
</script>

<style scoped>
.svc-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.svc-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--ui-border);
}
.svc-title { font-size: var(--fs-sm); font-weight: var(--fw-semibold); color: var(--ui-fg); }
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); align-items: center; }
.logs-terminal {
  width: 100%; height: calc(100vh - 140px);
  background: #0A0A0A;
  border-radius: var(--radius-sm);
  overflow: hidden;
  padding: var(--space-3);
}
.inspect-json {
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: var(--space-4);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 70vh;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--ui-fg-2);
}
</style>
