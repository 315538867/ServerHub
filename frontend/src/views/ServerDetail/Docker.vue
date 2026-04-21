<template>
  <div class="dk-page">
    <UiCard padding="none">
      <UiTabs :items="tabItems" :model-value="activeTab" @change="val => activeTab = String(val)" />
      <div class="dk-tab-body">
        <div v-if="activeTab === 'containers'">
          <div class="dk-toolbar">
            <div />
            <UiButton variant="secondary" size="sm" :loading="loading" @click="refresh">
              <template #icon><RefreshCw :size="14" /></template>
              刷新
            </UiButton>
          </div>
          <NDataTable
            :columns="containerColumns"
            :data="containers"
            :loading="loading"
            :row-key="(row: ContainerItem) => row.id"
            size="small"
            :bordered="false"
          />
        </div>
        <div v-else-if="activeTab === 'images'">
          <div class="dk-toolbar">
            <div />
            <UiButton variant="primary" size="sm" @click="openPull">
              <template #icon><Download :size="14" /></template>
              拉取镜像
            </UiButton>
          </div>
          <NDataTable
            :columns="imageColumns"
            :data="images"
            :loading="imgLoading"
            :row-key="(row: ImageItem) => row.id"
            size="small"
            :bordered="false"
          />
        </div>
      </div>
    </UiCard>

    <NDrawer v-model:show="logsVisible" :width="720" @after-leave="onLogsClosed">
      <NDrawerContent :title="`容器日志 — ${logsContainer}`" :native-scrollbar="false">
        <div ref="logsEl" class="logs-terminal" />
      </NDrawerContent>
    </NDrawer>

    <NModal v-model:show="inspectVisible" preset="card" title="容器详情" style="width: 720px" :bordered="false">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </NModal>

    <NModal
      v-model:show="pullVisible"
      preset="card"
      title="拉取镜像"
      style="width: 580px"
      :bordered="false"
      :mask-closable="!pulling"
      @after-leave="onPullClosed"
    >
      <NInput v-model:value="pullImage" placeholder="例如：ubuntu:22.04 或 nginx:latest" :disabled="pulling" @keydown.enter="startPull" />
      <pre v-if="pullOutput" ref="pullOutputEl" class="pull-output">{{ pullOutput }}</pre>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" :disabled="pulling" @click="pullVisible = false">关闭</UiButton>
          <UiButton variant="primary" size="sm" :loading="pulling" @click="startPull">拉取</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  NDataTable, NModal, NDrawer, NDrawerContent, NInput, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Download } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import {
  getContainers, containerAction, getContainerInspect,
  getImages, deleteImage, containerLogsWsUrl, pullImageWsUrl,
} from '@/api/docker'
import type { ContainerItem, ImageItem } from '@/api/docker'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiTabs from '@/components/ui/UiTabs.vue'

const route = useRoute()
const auth = useAuthStore()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))
const activeTab = ref('containers')

const tabItems = [
  { value: 'containers', label: '容器' },
  { value: 'images', label: '镜像' },
]

const containers = ref<ContainerItem[]>([])
const loading = ref(false)
const actionLoading = ref('')
const images = ref<ImageItem[]>([])
const imgLoading = ref(false)

const logsVisible = ref(false)
const logsContainer = ref('')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

const inspectVisible = ref(false)
const inspectJson = ref('')

const pullVisible = ref(false)
const pullImage = ref('')
const pullOutput = ref('')
const pulling = ref(false)
const pullOutputEl = ref<HTMLPreElement>()
let pullWs: WebSocket | null = null

function stateTone(state: string): 'success' | 'warning' | 'neutral' | 'danger' {
  if (state === 'running') return 'success'
  if (state === 'paused') return 'warning'
  if (state === 'exited') return 'neutral'
  return 'danger'
}
function stateText(state: string) {
  return ({ running: '运行中', paused: '已暂停', exited: '已停止', restarting: '重启中', dead: '异常', created: '已创建', removing: '删除中' } as Record<string, string>)[state] ?? state
}

const containerColumns = computed<DataTableColumns<ContainerItem>>(() => [
  { title: '名称', key: 'names', minWidth: 140, ellipsis: { tooltip: true } },
  { title: '镜像', key: 'image', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'state', width: 110,
    render: (row) => h(UiBadge, { tone: stateTone(row.state) }, () => stateText(row.state)),
  },
  { title: '端口', key: 'ports', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 300, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      row.state !== 'running' ? h(UiButton, { variant: 'ghost', size: 'sm', loading: actionLoading.value === row.id + '_start', onClick: () => doAction(row, 'start') }, () => '启动') : null,
      row.state === 'running' ? h(UiButton, { variant: 'ghost', size: 'sm', loading: actionLoading.value === row.id + '_stop', onClick: () => doAction(row, 'stop') }, () => '停止') : null,
      h(UiButton, { variant: 'ghost', size: 'sm', loading: actionLoading.value === row.id + '_restart', onClick: () => doAction(row, 'restart') }, () => '重启'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openLogs(row) }, () => '日志'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openInspect(row) }, () => '详情'),
      h(NPopconfirm, {
        onPositiveClick: () => doAction(row, 'remove'),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm', loading: actionLoading.value === row.id + '_remove' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除该容器？',
      }),
    ]),
  },
])

const imageColumns = computed<DataTableColumns<ImageItem>>(() => [
  { title: '仓库', key: 'repository', minWidth: 180, ellipsis: { tooltip: true } },
  { title: 'Tag', key: 'tag', width: 120 },
  { title: '大小', key: 'size', width: 100 },
  { title: '创建时间', key: 'created_at', minWidth: 140, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 90, fixed: 'right' as const,
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => rmImage(row),
      positiveText: '删除', negativeText: '取消',
    }, {
      trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
        () => h('span', { class: 'text-danger' }, '删除')),
      default: () => '确认删除该镜像？',
    }),
  },
])

async function refresh() {
  if (activeTab.value === 'containers') await loadContainers()
  else await loadImages()
}

watch(activeTab, async (tab) => {
  if (tab === 'images' && images.value.length === 0) await loadImages()
})

async function loadContainers() {
  loading.value = true
  try { containers.value = await getContainers(serverId.value) } finally { loading.value = false }
}
async function loadImages() {
  imgLoading.value = true
  try { images.value = await getImages(serverId.value) } finally { imgLoading.value = false }
}

async function doAction(row: ContainerItem, action: 'start' | 'stop' | 'restart' | 'remove') {
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
  logsContainer.value = row.names; logsVisible.value = true
  nextTick(() => {
    if (!logsEl.value) return
    logsTerm?.dispose()
    logsTerm = new Terminal({ theme: { background: '#0A0A0A', foreground: '#E4E4E7' }, convertEol: true, fontSize: 12 })
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
  } catch { message.error('获取详情失败') }
}

function openPull() { pullImage.value = ''; pullOutput.value = ''; pullVisible.value = true }

function startPull() {
  const image = pullImage.value.trim()
  if (!image) return
  pulling.value = true; pullOutput.value = ''
  pullWs?.close()
  pullWs = new WebSocket(pullImageWsUrl(serverId.value, image, auth.token))
  pullWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') {
        pullOutput.value += msg.data + '\n'
        nextTick(() => { if (pullOutputEl.value) pullOutputEl.value.scrollTop = pullOutputEl.value.scrollHeight })
      } else if (msg.type === 'done') { pulling.value = false; message.success('拉取完成'); loadImages() }
      else if (msg.type === 'error') { pulling.value = false; pullOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  pullWs.onerror = () => { pulling.value = false }
}

function onPullClosed() { pullWs?.close(); pullWs = null; pulling.value = false }

async function rmImage(row: ImageItem) {
  try { await deleteImage(serverId.value, row.id); message.success('镜像已删除'); await loadImages() }
  catch { message.error('删除失败') }
}

onMounted(() => loadContainers())
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose(); pullWs?.close() })
</script>

<style scoped>
.dk-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.dk-tab-body { padding: var(--space-4); }
.dk-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-3);
}

.logs-terminal {
  width: 100%;
  height: calc(100vh - 160px);
  background: #0A0A0A;
  border-radius: var(--radius-sm);
  overflow: hidden;
  padding: var(--space-2);
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

.pull-output {
  background: #0A0A0A;
  color: #E4E4E7;
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 320px;
  margin: var(--space-3) 0 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
