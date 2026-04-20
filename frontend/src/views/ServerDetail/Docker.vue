<template>
  <div class="page-container">
    <div class="section-block">
      <!-- 标签页 -->
      <t-tabs :value="activeTab" @change="activeTab = $event as string">
        <t-tab-panel value="containers" label="容器">
          <div class="tab-toolbar">
            <div class="toolbar-left">
              <t-button size="small" variant="outline" :loading="loading" @click="refresh">
                <template #icon><refresh-icon /></template>
                刷新
              </t-button>
            </div>
          </div>
          <t-table
            :data="containers"
            :columns="containerColumns"
            :loading="loading"
            row-key="id"
            bordered
            size="small"
          >
            <template #state="{ row }">
              <t-tag :theme="stateTheme(row.state)" variant="light" size="small">{{ row.status }}</t-tag>
            </template>
            <template #operations="{ row }">
              <t-space size="small">
                <t-button v-if="row.state !== 'running'" theme="success" size="small" variant="text" :loading="actionLoading === row.id + '_start'" @click="doAction(row, 'start')">启动</t-button>
                <t-button v-if="row.state === 'running'" theme="warning" size="small" variant="text" :loading="actionLoading === row.id + '_stop'" @click="doAction(row, 'stop')">停止</t-button>
                <t-button size="small" variant="text" :loading="actionLoading === row.id + '_restart'" @click="doAction(row, 'restart')">重启</t-button>
                <t-button size="small" variant="text" @click="openLogs(row)">日志</t-button>
                <t-button size="small" variant="text" @click="openInspect(row)">详情</t-button>
                <t-popconfirm content="确认删除该容器？" @confirm="doAction(row, 'remove')">
                  <t-button theme="danger" size="small" variant="text" :loading="actionLoading === row.id + '_remove'">删除</t-button>
                </t-popconfirm>
              </t-space>
            </template>
          </t-table>
        </t-tab-panel>

        <t-tab-panel value="images" label="镜像">
          <div class="tab-toolbar">
            <div class="toolbar-left" />
            <t-button theme="primary" size="small" @click="openPull">
              <template #icon><download-icon /></template>
              拉取镜像
            </t-button>
          </div>
          <t-table
            :data="images"
            :columns="imageColumns"
            :loading="imgLoading"
            row-key="id"
            bordered
            size="small"
          >
            <template #operations="{ row }">
              <t-popconfirm content="确认删除该镜像？" @confirm="rmImage(row)">
                <t-button theme="danger" size="small" variant="text">删除</t-button>
              </t-popconfirm>
            </template>
          </t-table>
        </t-tab-panel>
      </t-tabs>
    </div>

    <!-- 日志抽屉 -->
    <t-drawer v-model:visible="logsVisible" :header="`容器日志 — ${logsContainer}`" size="60%" @close="onLogsClosed">
      <div ref="logsEl" class="logs-terminal" />
    </t-drawer>

    <!-- 详情对话框 -->
    <t-dialog v-model:visible="inspectVisible" header="容器详情" width="720px" :footer="false">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </t-dialog>

    <!-- 拉取镜像对话框 -->
    <t-dialog
      v-model:visible="pullVisible"
      header="拉取镜像"
      width="580px"
      :close-on-overlay-click="!pulling"
      :confirm-btn="{ content: '拉取', loading: pulling }"
      :cancel-btn="{ disabled: pulling }"
      @confirm="startPull"
      @closed="onPullClosed"
    >
      <t-input v-model="pullImage" placeholder="例如：ubuntu:22.04 或 nginx:latest" :disabled="pulling" @keydown.enter="startPull" />
      <pre v-if="pullOutput" ref="pullOutputEl" class="pull-output">{{ pullOutput }}</pre>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon, DownloadIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import {
  getContainers, containerAction, getContainerInspect,
  getImages, deleteImage, containerLogsWsUrl, pullImageWsUrl,
} from '@/api/docker'
import type { ContainerItem, ImageItem } from '@/api/docker'

const route = useRoute()
const auth = useAuthStore()
const serverId = computed(() => Number(route.params.serverId))
const activeTab = ref('containers')
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

const containerColumns = [
  { colKey: 'names', title: '名称', minWidth: 140, ellipsis: true },
  { colKey: 'image', title: '镜像', minWidth: 160, ellipsis: true },
  { colKey: 'state', title: '状态', width: 110 },
  { colKey: 'ports', title: '端口', minWidth: 160, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 280, fixed: 'right' as const },
]
const imageColumns = [
  { colKey: 'repository', title: '仓库', minWidth: 180, ellipsis: true },
  { colKey: 'tag', title: 'Tag', width: 120 },
  { colKey: 'size', title: '大小', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 140, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 80, fixed: 'right' as const },
]

function stateTheme(state: string) {
  return ({ running: 'success', paused: 'warning', exited: 'default' } as Record<string, string>)[state] ?? 'danger'
}

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
    MessagePlugin.success('操作成功')
    await loadContainers()
  } catch { MessagePlugin.error('操作失败') }
  finally { actionLoading.value = '' }
}

function openLogs(row: ContainerItem) {
  logsContainer.value = row.names; logsVisible.value = true
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
      } else if (msg.type === 'done') { pulling.value = false; MessagePlugin.success('拉取完成'); loadImages() }
      else if (msg.type === 'error') { pulling.value = false; pullOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  pullWs.onerror = () => { pulling.value = false }
}

function onPullClosed() { pullWs?.close(); pullWs = null; pulling.value = false }

async function rmImage(row: ImageItem) {
  try { await deleteImage(serverId.value, row.id); MessagePlugin.success('镜像已删除'); await loadImages() }
  catch { MessagePlugin.error('删除失败') }
}

onMounted(() => loadContainers())
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose(); pullWs?.close() })
</script>

<style scoped>
.logs-terminal {
  width: 100%;
  height: calc(100vh - 120px);
  background: #1a2332;
  border-radius: 4px;
  overflow: hidden;
}

.inspect-json {
  background: var(--sh-gray-bg);
  border-radius: 4px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 70vh;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: "Cascadia Code", "JetBrains Mono", Menlo, monospace;
}

.pull-output {
  background: #1a2332;
  color: #e0e0e0;
  border-radius: 4px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 320px;
  margin: 12px 0 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: "Cascadia Code", "JetBrains Mono", Menlo, monospace;
}

:deep(.t-table) {
  font-size: 13px;
}
:deep(.t-tab-panel) {
  padding: 0;
}
</style>
