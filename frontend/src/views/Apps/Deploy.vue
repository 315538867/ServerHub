<template>
  <div class="page-container">
    <template v-if="deploy">
      <!-- 部署信息 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">部署信息</span>
          <t-space size="small">
            <t-button theme="primary" size="small" :loading="running" @click="doRun('run')">立即同步</t-button>
            <t-button size="small" :disabled="!deploy.previous_version" :loading="running" @click="doRun('rollback')">回滚到上个版本</t-button>
          </t-space>
        </div>
        <div class="desc-wrap">
          <t-descriptions :column="2">
            <t-descriptions-item label="名称">{{ deploy.name }}</t-descriptions-item>
            <t-descriptions-item label="类型"><t-tag size="small" variant="light">{{ deploy.type }}</t-tag></t-descriptions-item>
            <t-descriptions-item label="工作目录">{{ deploy.work_dir }}</t-descriptions-item>
            <t-descriptions-item label="镜像">{{ deploy.image_name || '—' }}</t-descriptions-item>
            <t-descriptions-item label="期望版本">{{ deploy.desired_version || '—' }}</t-descriptions-item>
            <t-descriptions-item label="实际版本">{{ deploy.actual_version || '—' }}</t-descriptions-item>
            <t-descriptions-item label="同步状态">
              <t-tag :theme="syncTheme(deploy.sync_status)" variant="light" size="small">{{ deploy.sync_status || 'idle' }}</t-tag>
            </t-descriptions-item>
            <t-descriptions-item label="最后运行">{{ deploy.last_run_at || '—' }}</t-descriptions-item>
          </t-descriptions>
        </div>
      </div>

      <!-- 部署历史 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">部署历史</span>
        </div>
        <div class="table-wrap">
          <t-table :data="logs" :columns="logColumns" :loading="logsLoading" row-key="id" stripe size="small">
            <template #status="{ row }">
              <t-tag :theme="row.status === 'success' ? 'success' : 'danger'" variant="light" size="small">{{ row.status }}</t-tag>
            </template>
            <template #duration="{ row }">{{ row.duration }}s</template>
          </t-table>
        </div>
      </div>
    </template>
    <div v-else-if="!loading" class="section-block empty-block">
      <t-empty description="该应用未关联部署配置，请先在应用设置中配置 deploy_id" />
    </div>

    <t-drawer v-model:visible="runDrawerVisible" header="部署输出" size="60%">
      <div class="run-status">
        <t-tag
          :theme="runStatus === 'success' ? 'success' : runStatus === 'failed' ? 'danger' : 'warning'"
          variant="light"
          size="small"
        >{{ running ? '执行中…' : runStatus || '就绪' }}</t-tag>
        <t-button v-if="running" size="small" variant="outline" @click="stopRun">中止</t-button>
      </div>
      <pre ref="logEl" class="run-output">{{ logLines.join('\n') }}</pre>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
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

const logColumns = [
  { colKey: 'created_at', title: '时间', width: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'duration', title: '耗时', width: 90 },
  { colKey: 'output', title: '输出', minWidth: 300, ellipsis: true },
]

const running = ref(false)
const runDrawerVisible = ref(false)
const runStatus = ref('')
const logLines = ref<string[]>([])
const logEl = ref<HTMLPreElement>()
let abortCtrl: AbortController | null = null

function syncTheme(s: string) {
  return ({ synced: 'success', drifted: 'warning', syncing: 'default', error: 'danger' } as Record<string, string>)[s] ?? 'default'
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
          MessagePlugin[evt.line === 'success' ? 'success' : 'error'](evt.line === 'success' ? '部署成功' : '部署失败')
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
.desc-wrap {
  padding: 16px 20px 20px;
}
:deep(.t-descriptions__label) {
  color: var(--sh-text-secondary);
  font-size: 13px;
  width: 90px;
}
:deep(.t-descriptions__content) {
  font-size: 13px;
}
.table-wrap {
  padding: 0 20px 16px;
}
:deep(.t-table td) {
  font-size: 13px;
}
.empty-block {
  padding: 40px 20px;
  display: flex;
  justify-content: center;
}
.run-status { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.run-output { background: #1a2332; color: #e0e0e0; border-radius: 4px; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; height: calc(100vh - 140px); white-space: pre-wrap; word-break: break-all; margin: 0; }
</style>
