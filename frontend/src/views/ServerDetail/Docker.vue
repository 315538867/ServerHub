<template>
  <div>
    <div class="page-toolbar">
      <el-button :icon="Refresh" :loading="loading" @click="refresh">刷新</el-button>
    </div>
    <el-tabs v-model="activeTab">
      <el-tab-pane label="容器" name="containers">
        <el-table :data="containers" v-loading="loading" style="width:100%">
          <el-table-column label="名称" prop="names" min-width="140" show-overflow-tooltip />
          <el-table-column label="镜像" prop="image" min-width="160" show-overflow-tooltip />
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="stateTag(row.state)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="端口" prop="ports" min-width="160" show-overflow-tooltip />
          <el-table-column label="操作" width="260" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.state !== 'running'" size="small" type="success" :loading="actionLoading === row.id + '_start'" @click="doAction(row, 'start')">启动</el-button>
              <el-button v-if="row.state === 'running'" size="small" type="warning" :loading="actionLoading === row.id + '_stop'" @click="doAction(row, 'stop')">停止</el-button>
              <el-button size="small" :loading="actionLoading === row.id + '_restart'" @click="doAction(row, 'restart')">重启</el-button>
              <el-button size="small" @click="openLogs(row)">日志</el-button>
              <el-button size="small" @click="openInspect(row)">详情</el-button>
              <el-popconfirm title="确认删除该容器？" @confirm="doAction(row, 'remove')">
                <template #reference>
                  <el-button size="small" type="danger" :loading="actionLoading === row.id + '_remove'">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="镜像" name="images">
        <div class="tab-toolbar">
          <el-button type="primary" :icon="Download" @click="openPull">拉取镜像</el-button>
        </div>
        <el-table :data="images" v-loading="imgLoading" style="width:100%">
          <el-table-column label="仓库" prop="repository" min-width="180" show-overflow-tooltip />
          <el-table-column label="Tag" prop="tag" width="120" />
          <el-table-column label="大小" prop="size" width="100" />
          <el-table-column label="创建时间" prop="created_at" min-width="140" show-overflow-tooltip />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-popconfirm title="确认删除该镜像？" @confirm="rmImage(row)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-drawer v-model="logsVisible" :title="`容器日志 — ${logsContainer}`" size="60%" direction="rtl" @closed="onLogsClosed">
      <div ref="logsEl" class="logs-terminal" />
    </el-drawer>

    <el-dialog v-model="inspectVisible" title="容器详情" width="720px" top="5vh">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </el-dialog>

    <el-dialog v-model="pullVisible" title="拉取镜像" width="580px" :close-on-click-modal="!pulling" @closed="onPullClosed">
      <el-input v-model="pullImage" placeholder="例如：ubuntu:22.04 或 nginx:latest" :disabled="pulling" @keyup.enter="startPull" />
      <pre v-if="pullOutput" ref="pullOutputEl" class="pull-output">{{ pullOutput }}</pre>
      <template #footer>
        <el-button @click="pullVisible = false" :disabled="pulling">取消</el-button>
        <el-button type="primary" :loading="pulling" @click="startPull">拉取</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh, Download } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
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

function stateTag(state: string) {
  return ({ running: 'success', paused: 'warning', exited: 'info' } as Record<string, string>)[state] ?? 'danger'
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
  try { containers.value = await getContainers(serverId.value) }
  finally { loading.value = false }
}

async function loadImages() {
  imgLoading.value = true
  try { images.value = await getImages(serverId.value) }
  finally { imgLoading.value = false }
}

async function doAction(row: ContainerItem, action: 'start' | 'stop' | 'restart' | 'remove') {
  const key = `${row.id}_${action}`
  actionLoading.value = key
  try {
    await containerAction(serverId.value, row.id, action)
    ElMessage.success('操作成功')
    await loadContainers()
  } catch { ElMessage.error('操作失败') }
  finally { actionLoading.value = '' }
}

function openLogs(row: ContainerItem) {
  logsContainer.value = row.names; logsVisible.value = true
  nextTick(() => {
    if (!logsEl.value) return
    logsTerm?.dispose()
    logsTerm = new Terminal({ theme: { background: '#1a1a2e' }, convertEol: true, fontSize: 13 })
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
  } catch { ElMessage.error('获取详情失败') }
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
      } else if (msg.type === 'done') { pulling.value = false; ElMessage.success('拉取完成'); loadImages() }
      else if (msg.type === 'error') { pulling.value = false; pullOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  pullWs.onerror = () => { pulling.value = false }
}

function onPullClosed() { pullWs?.close(); pullWs = null; pulling.value = false }

async function rmImage(row: ImageItem) {
  try { await deleteImage(serverId.value, row.id); ElMessage.success('镜像已删除'); await loadImages() }
  catch { ElMessage.error('删除失败') }
}

onMounted(() => loadContainers())
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose(); pullWs?.close() })
</script>

<style scoped>
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
.tab-toolbar { margin-bottom: 12px; }
.logs-terminal { width: 100%; height: calc(100vh - 120px); background: #1a1a2e; border-radius: 4px; overflow: hidden; }
.inspect-json { background: #f5f7fa; border-radius: 4px; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 70vh; margin: 0; white-space: pre-wrap; word-break: break-all; }
.pull-output { background: #1a1a2e; color: #e0e0e0; border-radius: 4px; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 320px; margin: 12px 0 0; white-space: pre-wrap; word-break: break-all; }
</style>
