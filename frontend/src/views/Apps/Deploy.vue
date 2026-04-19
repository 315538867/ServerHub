<template>
  <div class="deploy-page">
    <template v-if="deploy">
      <el-descriptions :column="2" border class="desc-block">
        <el-descriptions-item label="名称">{{ deploy.name }}</el-descriptions-item>
        <el-descriptions-item label="类型"><el-tag size="small">{{ deploy.type }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="工作目录">{{ deploy.work_dir }}</el-descriptions-item>
        <el-descriptions-item label="镜像">{{ deploy.image_name || '—' }}</el-descriptions-item>
        <el-descriptions-item label="期望版本">{{ deploy.desired_version || '—' }}</el-descriptions-item>
        <el-descriptions-item label="实际版本">{{ deploy.actual_version || '—' }}</el-descriptions-item>
        <el-descriptions-item label="同步状态">
          <el-tag :type="syncTag(deploy.sync_status)" size="small">{{ deploy.sync_status || 'idle' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="最后运行">{{ deploy.last_run_at || '—' }}</el-descriptions-item>
      </el-descriptions>

      <div class="action-row">
        <el-button type="primary" :loading="running" @click="doRun('run')">立即同步</el-button>
        <el-button :disabled="!deploy.previous_version" :loading="running" @click="doRun('rollback')">回滚到上个版本</el-button>
      </div>

      <el-divider>部署历史</el-divider>
      <el-table :data="logs" v-loading="logsLoading" style="width:100%">
        <el-table-column label="时间" prop="created_at" width="180" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="90">
          <template #default="{ row }">{{ row.duration }}s</template>
        </el-table-column>
        <el-table-column label="输出" prop="output" min-width="300" show-overflow-tooltip />
      </el-table>
    </template>
    <el-empty v-else-if="!loading" description="该应用未关联部署配置，请先在应用设置中配置 deploy_id" />

    <el-drawer v-model="runDrawerVisible" title="部署输出" size="60%" direction="rtl" :close-on-click-modal="!running">
      <div class="run-status">
        <el-tag :type="runStatus === 'success' ? 'success' : runStatus === 'failed' ? 'danger' : 'warning'" size="small">
          {{ running ? '执行中…' : runStatus || '就绪' }}
        </el-tag>
        <el-button v-if="running" size="small" @click="stopRun">中止</el-button>
      </div>
      <pre ref="logEl" class="run-output">{{ logLines.join('\n') }}</pre>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { getDeploys, getDeployLogs } from '@/api/deploy'
import type { Deploy, DeployLog } from '@/types/api'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const deploy = ref<Deploy | null>(null)
const loading = ref(false)
const logs = ref<DeployLog[]>([])
const logsLoading = ref(false)

const running = ref(false)
const runDrawerVisible = ref(false)
const runStatus = ref('')
const logLines = ref<string[]>([])
const logEl = ref<HTMLPreElement>()
let abortCtrl: AbortController | null = null

function syncTag(s: string) {
  return ({ synced: 'success', drifted: 'warning', syncing: 'info', error: 'danger' } as Record<string, string>)[s] ?? 'info'
}

async function loadDeploy() {
  if (!app.value?.deploy_id) return
  loading.value = true
  try {
    const all = await getDeploys()
    deploy.value = all.find(d => d.id === app.value!.deploy_id) ?? null
  } finally { loading.value = false }
}

async function loadLogs() {
  if (!deploy.value) return
  logsLoading.value = true
  try { logs.value = await getDeployLogs(deploy.value.id, 30) }
  finally { logsLoading.value = false }
}

async function doRun(endpoint: 'run' | 'rollback') {
  if (!deploy.value) return
  running.value = true
  runStatus.value = ''
  logLines.value = []
  runDrawerVisible.value = true
  abortCtrl = new AbortController()
  try {
    const res = await fetch(`/panel/api/v1/deploys/${deploy.value.id}/${endpoint}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.token}` },
      signal: abortCtrl.signal,
    })
    if (!res.body) throw new Error('no response body')
    await streamSSE(res)
  } catch (e: unknown) {
    if ((e as Error).name !== 'AbortError') {
      logLines.value.push('[连接错误] ' + String(e))
      runStatus.value = 'failed'
    }
  } finally {
    running.value = false
    await loadLogs()
  }
}

async function streamSSE(res: Response) {
  if (!res.body) return
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''
    for (const part of parts) {
      const line = part.trim()
      if (!line.startsWith('data: ')) continue
      try {
        const evt = JSON.parse(line.slice(6)) as { type: string; line: string }
        if (evt.type === 'output' || evt.type === 'error') {
          logLines.value.push(evt.line)
          await nextTick()
          if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
        } else if (evt.type === 'done') {
          runStatus.value = evt.line
          ElMessage({ type: evt.line === 'success' ? 'success' : 'error', message: evt.line === 'success' ? '部署成功' : '部署失败' })
        }
      } catch { /* ignore */ }
    }
  }
}

function stopRun() { abortCtrl?.abort(); running.value = false }

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadDeploy()
  await loadLogs()
})
</script>

<style scoped>
.deploy-page { padding: 4px 0; }
.desc-block { margin-bottom: 20px; }
.action-row { display: flex; gap: 8px; margin-bottom: 16px; }
.run-status { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.run-output { background: #1a1a2e; color: #e0e0e0; border-radius: 4px; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; height: calc(100vh - 140px); white-space: pre-wrap; word-break: break-all; margin: 0; }
</style>
