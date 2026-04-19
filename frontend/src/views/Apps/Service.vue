<template>
  <div class="service-page">
    <template v-if="app?.container_name && app?.server_id">
      <div class="page-toolbar">
        <el-button :icon="Refresh" :loading="loading" @click="loadContainers">刷新</el-button>
      </div>
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
          </template>
        </el-table-column>
      </el-table>
    </template>
    <el-empty v-else description="该应用未关联 Docker 容器，请先在应用设置中配置 container_name" />

    <el-drawer v-model="logsVisible" :title="`容器日志 — ${logsContainer}`" size="60%" direction="rtl" @closed="onLogsClosed">
      <div ref="logsEl" class="logs-terminal" />
    </el-drawer>

    <el-dialog v-model="inspectVisible" title="容器详情" width="720px" top="5vh">
      <pre class="inspect-json">{{ inspectJson }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
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

const logsVisible = ref(false)
const logsContainer = ref('')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

const inspectVisible = ref(false)
const inspectJson = ref('')

function stateTag(state: string) {
  return ({ running: 'success', paused: 'warning', exited: 'info' } as Record<string, string>)[state] ?? 'danger'
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
    ElMessage.success('操作成功')
    await loadContainers()
  } catch { ElMessage.error('操作失败') }
  finally { actionLoading.value = '' }
}

function openLogs(row: ContainerItem) {
  logsContainer.value = row.names
  logsVisible.value = true
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

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadContainers()
})
onBeforeUnmount(() => { logsWs?.close(); logsTerm?.dispose() })
</script>

<style scoped>
.service-page { padding: 4px 0; }
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
.logs-terminal { width: 100%; height: calc(100vh - 120px); background: #1a1a2e; border-radius: 4px; overflow: hidden; }
.inspect-json { background: #f5f7fa; border-radius: 4px; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 70vh; margin: 0; white-space: pre-wrap; word-break: break-all; }
</style>
