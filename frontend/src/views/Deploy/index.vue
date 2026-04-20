<template>
  <div class="apps-page">
    <div class="page-header">
      <h2 class="page-title">应用管理</h2>
      <div class="header-right">
        <t-select v-model="filterServerId" placeholder="全部服务器" clearable style="width:180px;margin-right:12px">
          <t-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
        </t-select>
        <t-button theme="primary" @click="openCreate">
          <template #icon><add-icon /></template>
          新建应用
        </t-button>
      </div>
    </div>

    <t-empty v-if="!loading && filteredApps.length === 0" description="暂无应用，点击右上角新建" style="margin-top:60px" />

    <div v-else class="app-grid" v-loading="loading">
      <div v-for="app in filteredApps" :key="app.id" class="app-card" :class="`app-card--${app.sync_status || 'idle'}`">
        <div class="app-card__header">
          <div class="app-card__title">
            <span class="app-card__name">{{ app.name }}</span>
            <t-tag size="small" :theme="typeTagTheme(app.type)" variant="light">{{ app.type }}</t-tag>
          </div>
          <t-tag size="small" :theme="syncStatusTagTheme(app.sync_status)" variant="light">
            <t-loading v-if="app.sync_status === 'syncing'" size="small" style="display:inline-flex;margin-right:4px" />
            {{ syncStatusText(app.sync_status) }}
          </t-tag>
        </div>

        <div class="app-card__server">
          <server-icon style="color:#8a94a6;font-size:14px" />
          {{ serverName(app.server_id) }}
        </div>

        <div class="app-card__version">
          <div class="version-block">
            <span class="version-label">期望</span>
            <span class="version-value" :class="{ 'version-value--drifted': isDrifted(app) }">
              {{ app.desired_version || '—' }}
            </span>
          </div>
          <div class="version-arrow" :class="{ 'version-arrow--drifted': isDrifted(app) }">→</div>
          <div class="version-block">
            <span class="version-label">实际</span>
            <span class="version-value">{{ app.actual_version || '—' }}</span>
          </div>
        </div>

        <div v-if="isDrifted(app)" class="app-card__drift-hint">
          版本未同步，{{ app.auto_sync ? '将自动更新' : '需手动触发' }}
        </div>

        <div class="app-card__actions">
          <t-button theme="primary" size="small" @click="openSetVersion(app)">设置版本</t-button>
          <t-button
            size="small"
            :theme="isDrifted(app) ? 'warning' : 'default'"
            :loading="syncing === app.id"
            @click="handleSync(app)"
          >立即同步</t-button>
          <t-dropdown :options="dropdownOptions(app)" trigger="click" @click="(item: any) => handleCommand(item.value, app)">
            <t-button size="small" variant="outline" shape="circle">
              <template #icon><ellipsis-icon /></template>
            </t-button>
          </t-dropdown>
        </div>
      </div>
    </div>

    <!-- Set Version Dialog -->
    <t-dialog
      v-model:visible="versionDialogVisible"
      header="设置期望版本"
      width="420px"
      :confirm-btn="{ content: '确定', loading: versionSaving }"
      @confirm="saveVersion"
      @closed="versionForm.desired_version = ''"
    >
      <div class="version-dialog__current">
        当前实际版本：<span class="version-value">{{ versionTarget?.actual_version || '未部署' }}</span>
      </div>
      <t-form :data="versionForm" style="margin-top:16px">
        <t-form-item label="期望版本">
          <t-input v-model="versionForm.desired_version" placeholder="v1.0 / latest / 20240101" autofocus />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- Create App Dialog -->
    <t-dialog
      v-model:visible="createVisible"
      header="新建应用"
      width="600px"
      :confirm-btn="{ content: '创建', loading: createSaving }"
      @confirm="handleCreate"
      @closed="resetCreateForm"
    >
      <t-form ref="createFormRef" :data="createForm" :rules="createRules" label-width="90px" colon>
        <t-form-item label="名称" name="name">
          <t-input v-model="createForm.name" placeholder="my-app" />
        </t-form-item>
        <t-form-item label="服务器" name="server_id">
          <t-select v-model="createForm.server_id" placeholder="选择服务器" style="width:100%">
            <t-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </t-select>
        </t-form-item>
        <t-form-item label="应用类型">
          <t-radio-group v-model="createForm.type">
            <t-radio value="docker-compose">Docker Compose</t-radio>
            <t-radio value="docker">Docker</t-radio>
            <t-radio value="native">Native</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="工作目录">
          <t-input v-model="createForm.work_dir" placeholder="/opt/myapp" />
        </t-form-item>
        <t-form-item v-if="createForm.type === 'docker-compose'" label="Compose 文件">
          <t-input v-model="createForm.compose_file" placeholder="docker-compose.yml" />
        </t-form-item>
        <t-form-item v-if="createForm.type === 'docker'" label="镜像名">
          <t-input v-model="createForm.image_name" placeholder="nginx（不含 tag）" />
        </t-form-item>
        <t-form-item v-if="createForm.type !== 'docker-compose'" label="部署脚本">
          <t-textarea v-model="createForm.start_cmd" :autosize="{ minRows: 3 }" placeholder="./app --port 8080" />
        </t-form-item>
        <div class="form-section-label">版本控制</div>
        <t-form-item label="期望版本">
          <t-input v-model="createForm.desired_version" placeholder="v1.0 / latest（留空仅保存配置）" />
        </t-form-item>
        <t-form-item label="自动同步">
          <t-switch v-model="createForm.auto_sync" />
          <span class="form-hint">版本变化时自动触发同步</span>
        </t-form-item>
        <t-form-item v-if="createForm.auto_sync" label="检查间隔">
          <t-input-number v-model="createForm.sync_interval" :min="30" :max="3600" :step="30" />
          <span class="form-hint">秒</span>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- App Detail Drawer -->
    <t-drawer v-model:visible="detailVisible" :header="detailApp?.name" size="55%">
      <t-tabs v-if="detailApp" :value="detailTab" @change="val => (detailTab = val as string)">

        <t-tab-panel value="version" label="版本管理">
          <div class="tab-content">
            <t-form :data="detailVersionForm" label-width="90px" colon style="max-width:480px">
              <t-form-item label="期望版本">
                <t-input v-model="detailVersionForm.desired_version" placeholder="v1.0 / latest" />
              </t-form-item>
              <t-form-item label="实际版本">
                <t-input :value="detailApp.actual_version || '—'" readonly />
              </t-form-item>
              <t-form-item label="历史版本">
                <t-input :value="detailApp.previous_version || '—'" readonly />
              </t-form-item>
              <t-form-item label="自动同步">
                <t-switch v-model="detailVersionForm.auto_sync" />
              </t-form-item>
              <t-form-item v-if="detailVersionForm.auto_sync" label="检查间隔">
                <t-input-number v-model="detailVersionForm.sync_interval" :min="30" :max="3600" :step="30" />
                <span class="form-hint">秒</span>
              </t-form-item>
              <t-form-item>
                <t-space>
                  <t-button theme="primary" :loading="detailSaving" @click="saveDetailVersion">保存</t-button>
                  <t-button v-if="detailApp.previous_version" theme="warning" @click="handleRollbackFromDetail">
                    回滚到 {{ detailApp.previous_version }}
                  </t-button>
                </t-space>
              </t-form-item>
            </t-form>
          </div>
        </t-tab-panel>

        <t-tab-panel value="config" label="应用配置">
          <div class="tab-content">
            <t-form :data="detailConfigForm" label-width="90px" colon style="max-width:480px">
              <t-form-item label="服务器">
                <t-select v-model="detailConfigForm.server_id" style="width:100%">
                  <t-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
                </t-select>
              </t-form-item>
              <t-form-item label="应用类型">
                <t-radio-group v-model="detailConfigForm.type">
                  <t-radio value="docker-compose">Docker Compose</t-radio>
                  <t-radio value="docker">Docker</t-radio>
                  <t-radio value="native">Native</t-radio>
                </t-radio-group>
              </t-form-item>
              <t-form-item label="工作目录">
                <t-input v-model="detailConfigForm.work_dir" />
              </t-form-item>
              <t-form-item v-if="detailConfigForm.type === 'docker-compose'" label="Compose 文件">
                <t-input v-model="detailConfigForm.compose_file" />
              </t-form-item>
              <t-form-item v-if="detailConfigForm.type === 'docker'" label="镜像名">
                <t-input v-model="detailConfigForm.image_name" />
              </t-form-item>
              <t-form-item v-if="detailConfigForm.type !== 'docker-compose'" label="部署脚本">
                <t-textarea v-model="detailConfigForm.start_cmd" :autosize="{ minRows: 4 }" />
              </t-form-item>
              <t-form-item>
                <t-button theme="primary" :loading="detailSaving" @click="saveDetailConfig">保存配置</t-button>
              </t-form-item>
            </t-form>
          </div>
        </t-tab-panel>

        <t-tab-panel value="env" label="环境变量">
          <div class="tab-content">
            <div class="env-toolbar">
              <t-button size="small" theme="primary" @click="addEnvRow">
                <template #icon><add-icon /></template>
                添加变量
              </t-button>
            </div>
            <t-table :data="envVars" :columns="envColumns" size="small" row-key="key">
              <template #key="{ row }">
                <t-input v-model="row.key" size="small" placeholder="VAR_NAME" />
              </template>
              <template #value="{ row }">
                <t-input v-model="row.value" size="small" :type="row.secret ? 'password' : 'text'" placeholder="value" />
              </template>
              <template #secret="{ row }">
                <t-checkbox v-model="row.secret" />
              </template>
              <template #operations="{ rowIndex }">
                <t-button theme="danger" size="small" variant="text" @click="envVars.splice(rowIndex, 1)">
                  <template #icon><delete-icon /></template>
                </t-button>
              </template>
            </t-table>
            <div style="margin-top:12px">
              <t-button theme="primary" :loading="envSaving" @click="saveEnv">保存环境变量</t-button>
            </div>
          </div>
        </t-tab-panel>

        <t-tab-panel value="history" label="同步历史">
          <div class="tab-content">
            <t-table :data="historyLogs" :columns="historyColumns" size="small" stripe row-key="id">
              <template #created_at="{ row }">{{ dayjs(row.created_at).format('MM-DD HH:mm:ss') }}</template>
              <template #status="{ row }">
                <t-tag size="small" :theme="row.status === 'success' ? 'success' : 'danger'" variant="light">
                  {{ row.status === 'success' ? '成功' : '失败' }}
                </t-tag>
              </template>
              <template #duration="{ row }">{{ row.duration }}s</template>
              <template #operations="{ row }">
                <t-button size="small" variant="text" @click="viewLogDetail(row)">日志</t-button>
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="webhook" label="Webhook">
          <div class="tab-content">
            <t-form label-width="110px" colon style="max-width:500px">
              <t-form-item label="Webhook URL">
                <div class="input-with-btn">
                  <t-input v-model="webhookUrl" readonly />
                  <t-button @click="copyWebhook">复制</t-button>
                </div>
              </t-form-item>
              <t-form-item label="Secret Token">
                <t-input v-model="webhookSecret" readonly type="password" />
              </t-form-item>
            </t-form>
            <t-alert theme="info" message="Webhook 收到 POST 请求后将自动触发同步。支持 GitHub / GitLab 签名验证。" style="margin-top:8px" />
          </div>
        </t-tab-panel>
      </t-tabs>
    </t-drawer>

    <!-- Sync Log Drawer (SSE) -->
    <t-drawer v-model:visible="logDrawerVisible" :header="`同步日志 — ${logAppName}`" size="55%" @close="stopSync">
      <div class="log-toolbar">
        <t-tag :theme="runStatus === 'success' ? 'success' : runStatus === 'failed' ? 'danger' : 'default'" variant="light" size="small">
          {{ runStatus === 'running' ? '同步中…' : runStatus === 'success' ? '成功' : runStatus === 'failed' ? '失败' : '就绪' }}
        </t-tag>
        <t-button size="small" variant="text" @click="logLines = []">清空</t-button>
      </div>
      <pre class="log-output" ref="logEl">{{ logLines.join('\n') }}</pre>
    </t-drawer>

    <!-- Log Detail Dialog -->
    <t-dialog v-model:visible="logDetailVisible" header="执行日志" width="720px" :footer="false">
      <pre class="log-output log-output--static">{{ selectedLog?.output }}</pre>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, nextTick, onMounted, watch } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { AddIcon, DeleteIcon, EllipsisIcon, ServerIcon } from 'tdesign-icons-vue-next'
import dayjs from 'dayjs'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  getDeploys, createDeploy, updateDeploy, deleteDeploy,
  getDeployLogs, getDeployEnv, putDeployEnv, getWebhookInfo,
} from '@/api/deploy'
import type { EnvVar } from '@/api/deploy'
import type { Server, Deploy, DeployForm, DeployLog } from '@/types/api'

const authStore = useAuthStore()
const apps = ref<Deploy[]>([])
const servers = ref<Server[]>([])
const loading = ref(false)
const filterServerId = ref<number | null>(null)

const filteredApps = computed(() =>
  filterServerId.value ? apps.value.filter(a => a.server_id === filterServerId.value) : apps.value
)

function serverName(id: number) {
  return servers.value.find(s => s.id === id)?.name ?? `#${id}`
}
function isDrifted(app: Deploy) {
  return !!(app.desired_version && app.desired_version !== app.actual_version)
}
function typeTagTheme(type: string) {
  return ({ 'docker-compose': 'primary', docker: 'success', native: 'warning' } as Record<string, string>)[type] ?? 'default'
}
function syncStatusTagTheme(s: Deploy['sync_status']) {
  return ({ synced: 'success', drifted: 'warning', syncing: 'primary', error: 'danger', '': 'default' } as Record<string, string>)[s ?? ''] ?? 'default'
}
function syncStatusText(s: Deploy['sync_status']) {
  return ({ synced: '已同步', drifted: '待更新', syncing: '同步中', error: '错误', '': '空闲' } as Record<string, string>)[s ?? '']
}

function toUpdateForm(app: Deploy, override: Partial<DeployForm> = {}): DeployForm {
  return {
    name: app.name, server_id: app.server_id, type: app.type,
    work_dir: app.work_dir, compose_file: app.compose_file,
    start_cmd: app.start_cmd, image_name: app.image_name,
    desired_version: app.desired_version,
    auto_sync: app.auto_sync, sync_interval: app.sync_interval,
    ...override,
  }
}

function dropdownOptions(app: Deploy) {
  const items: Array<{ content: string; value: string; divider?: boolean; theme?: string }> = [
    { content: '应用详情', value: 'detail' },
    { content: '环境变量', value: 'env' },
    { content: '同步历史', value: 'history' },
    { content: 'Webhook', value: 'webhook' },
  ]
  if (app.previous_version) {
    items.push({ content: `回滚到 ${app.previous_version}`, value: 'rollback', divider: true })
  }
  items.push({ content: '删除', value: 'delete', divider: true, theme: 'error' })
  return items
}

async function loadAll() {
  loading.value = true
  try {
    [apps.value, servers.value] = await Promise.all([getDeploys(), getServers()])
  } finally {
    loading.value = false
  }
}

// ── Set Version ───────────────────────────────────────────────
const versionDialogVisible = ref(false)
const versionTarget = ref<Deploy | null>(null)
const versionSaving = ref(false)
const versionForm = reactive({ desired_version: '' })

function openSetVersion(app: Deploy) {
  versionTarget.value = app
  versionForm.desired_version = app.desired_version
  versionDialogVisible.value = true
}

async function saveVersion() {
  if (!versionTarget.value) return
  versionSaving.value = true
  try {
    await updateDeploy(versionTarget.value.id, toUpdateForm(versionTarget.value, { desired_version: versionForm.desired_version }))
    MessagePlugin.success('期望版本已更新')
    versionDialogVisible.value = false
    await loadAll()
  } finally {
    versionSaving.value = false
  }
}

// ── Create App ────────────────────────────────────────────────
const createVisible = ref(false)
const createSaving = ref(false)
const createFormRef = ref()
const defaultCreateForm = (): DeployForm => ({
  name: '', server_id: null, type: 'docker-compose',
  work_dir: '', compose_file: 'docker-compose.yml',
  start_cmd: '', image_name: '',
  desired_version: '', auto_sync: false, sync_interval: 60,
})
const createForm = reactive<DeployForm>(defaultCreateForm())
const createRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  server_id: [{ required: true, message: '请选择服务器', trigger: 'change' }],
}

function openCreate() {
  Object.assign(createForm, defaultCreateForm())
  createVisible.value = true
}
function resetCreateForm() { createFormRef.value?.clearValidate() }

async function handleCreate() {
  const result = await createFormRef.value?.validate()
  if (result !== true) return
  createSaving.value = true
  try {
    await createDeploy(createForm)
    MessagePlugin.success('应用已创建')
    createVisible.value = false
    await loadAll()
  } finally { createSaving.value = false }
}

// ── Sync (SSE) ────────────────────────────────────────────────
const syncing = ref<number | null>(null)
const logDrawerVisible = ref(false)
const logLines = ref<string[]>([])
const runStatus = ref<'' | 'running' | 'success' | 'failed'>('')
const logAppName = ref('')
const logEl = ref<HTMLPreElement>()
let abortCtrl: AbortController | null = null

async function runWithSSE(app: Deploy, endpoint: 'run' | 'rollback') {
  logAppName.value = app.name
  syncing.value = app.id
  logLines.value = []
  runStatus.value = 'running'
  logDrawerVisible.value = true

  abortCtrl = new AbortController()
  try {
    const res = await fetch(`/panel/api/v1/deploys/${app.id}/${endpoint}`, {
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
    syncing.value = null
    await loadAll()
  }
}

function handleSync(app: Deploy) { return runWithSSE(app, 'run') }

function stopSync() { abortCtrl?.abort(); syncing.value = null }

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
          runStatus.value = evt.line as 'success' | 'failed'
        }
      } catch {}
    }
  }
}

// ── Detail Drawer ─────────────────────────────────────────────
const detailVisible = ref(false)
const detailApp = ref<Deploy | null>(null)
const detailTab = ref('version')
const detailSaving = ref(false)
const detailVersionForm = reactive({ desired_version: '', auto_sync: false, sync_interval: 60 })
const detailConfigForm = reactive({
  server_id: null as number | null, type: 'docker-compose' as Deploy['type'],
  work_dir: '', compose_file: '', start_cmd: '', image_name: '',
})

async function openDetail(app: Deploy, tab = 'version') {
  detailApp.value = app
  detailTab.value = tab
  Object.assign(detailVersionForm, { desired_version: app.desired_version, auto_sync: app.auto_sync, sync_interval: app.sync_interval })
  Object.assign(detailConfigForm, { server_id: app.server_id, type: app.type, work_dir: app.work_dir, compose_file: app.compose_file, start_cmd: app.start_cmd, image_name: app.image_name })
  envVars.value = []
  historyLogs.value = []
  webhookUrl.value = ''
  webhookSecret.value = ''
  if (tab === 'env') await loadEnv(app.id)
  if (tab === 'history') await loadHistory(app.id)
  if (tab === 'webhook') await loadWebhook(app.id)
  detailVisible.value = true
}

watch(detailTab, async (tab) => {
  if (!detailApp.value) return
  if (tab === 'env' && envVars.value.length === 0) await loadEnv(detailApp.value.id)
  if (tab === 'history') await loadHistory(detailApp.value.id)
  if (tab === 'webhook' && !webhookUrl.value) await loadWebhook(detailApp.value.id)
})

async function saveDetailVersion() {
  if (!detailApp.value) return
  detailSaving.value = true
  try {
    await updateDeploy(detailApp.value.id, toUpdateForm(detailApp.value, detailVersionForm))
    MessagePlugin.success('版本配置已保存')
    await loadAll()
    detailApp.value = apps.value.find(a => a.id === detailApp.value!.id) ?? detailApp.value
  } finally { detailSaving.value = false }
}

async function saveDetailConfig() {
  if (!detailApp.value) return
  detailSaving.value = true
  try {
    await updateDeploy(detailApp.value.id, toUpdateForm(detailApp.value, detailConfigForm))
    MessagePlugin.success('应用配置已保存')
    await loadAll()
  } finally { detailSaving.value = false }
}

// ── Env ───────────────────────────────────────────────────────
const envVars = ref<EnvVar[]>([])
const envSaving = ref(false)
const envColumns = [
  { colKey: 'key', title: 'Key', minWidth: 150 },
  { colKey: 'value', title: 'Value', minWidth: 180 },
  { colKey: 'secret', title: 'Secret', width: 70, align: 'center' as const },
  { colKey: 'operations', title: '', width: 50, align: 'center' as const },
]

async function loadEnv(id: number) { envVars.value = await getDeployEnv(id) }
function addEnvRow() { envVars.value.push({ key: '', value: '', secret: false }) }
async function saveEnv() {
  if (!detailApp.value) return
  envSaving.value = true
  try {
    await putDeployEnv(detailApp.value.id, envVars.value)
    MessagePlugin.success('环境变量已保存')
  } finally { envSaving.value = false }
}

// ── History ───────────────────────────────────────────────────
const historyLogs = ref<DeployLog[]>([])
const historyColumns = [
  { colKey: 'created_at', title: '时间', width: 155 },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'duration', title: '耗时', width: 70 },
  { colKey: 'operations', title: '', width: 60 },
]
const logDetailVisible = ref(false)
const selectedLog = ref<DeployLog | null>(null)

async function loadHistory(id: number) { historyLogs.value = await getDeployLogs(id) }
function viewLogDetail(log: DeployLog) { selectedLog.value = log; logDetailVisible.value = true }

// ── Webhook ───────────────────────────────────────────────────
const webhookUrl = ref('')
const webhookSecret = ref('')

async function loadWebhook(id: number) {
  const info = await getWebhookInfo(id)
  webhookUrl.value = info.url
  webhookSecret.value = info.secret
}
function copyWebhook() {
  navigator.clipboard.writeText(webhookUrl.value)
  MessagePlugin.success('已复制')
}

// ── Rollback ──────────────────────────────────────────────────
function handleRollback(app: Deploy) { return runWithSSE(app, 'rollback') }

async function handleRollbackFromDetail() {
  if (!detailApp.value) return
  const app = detailApp.value
  detailVisible.value = false
  await handleRollback(app)
}

// ── Delete ────────────────────────────────────────────────────
function handleDelete(app: Deploy) {
  const dialog = DialogPlugin.confirm({
    header: '删除确认',
    body: `确认删除应用「${app.name}」？此操作不可恢复。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    onConfirm: async () => {
      dialog.hide()
      await deleteDeploy(app.id)
      MessagePlugin.success('已删除')
      await loadAll()
    },
  })
}

// ── Command dispatcher ────────────────────────────────────────
async function handleCommand(cmd: string, app: Deploy) {
  switch (cmd) {
    case 'detail':   await openDetail(app, 'version'); break
    case 'env':      await openDetail(app, 'env'); break
    case 'history':  await openDetail(app, 'history'); break
    case 'webhook':  await openDetail(app, 'webhook'); break
    case 'rollback': await handleRollback(app); break
    case 'delete':   handleDelete(app); break
  }
}

onMounted(loadAll)
</script>

<style scoped>
.apps-page { padding: 20px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.page-title { margin: 0; font-size: 18px; font-weight: 600; color: var(--td-text-color-primary); }
.header-right { display: flex; align-items: center; }

.app-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }

.app-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-border);
  border-left: 4px solid var(--td-component-border);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: box-shadow 0.2s, border-left-color 0.3s;
  min-height: 180px;
}
.app-card:hover { box-shadow: var(--td-shadow-1); }
.app-card--synced  { border-left-color: #00a870; }
.app-card--drifted { border-left-color: #ed7b2f; }
.app-card--syncing { border-left-color: #0052d9; }
.app-card--error   { border-left-color: #e34d59; }
.app-card--idle    { border-left-color: var(--td-component-border); }

.app-card__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
.app-card__title { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; min-width: 0; }
.app-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-card__server {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.app-card__version {
  display: flex;
  align-items: center;
  justify-content: space-around;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
  padding: 10px 12px;
}
.version-block { display: flex; flex-direction: column; align-items: center; gap: 3px; }
.version-label { font-size: 11px; color: var(--td-text-color-secondary); }
.version-value {
  font-family: 'JetBrains Mono', 'Cascadia Code', Menlo, monospace;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}
.version-value--drifted { color: #ed7b2f; }
.version-arrow { color: var(--td-text-color-placeholder); font-size: 18px; display: flex; align-items: center; }
.version-arrow--drifted { color: #ed7b2f; }

.app-card__drift-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #ed7b2f;
}

.app-card__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: auto;
  padding-top: 2px;
}

.env-toolbar { margin-bottom: 10px; }
.form-section-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-secondary);
  padding: 8px 0 4px;
  border-bottom: 1px solid var(--td-component-border);
  margin-bottom: 12px;
}
.form-hint { margin-left: 8px; font-size: 12px; color: var(--td-text-color-secondary); }
.tab-content { padding: 16px 0; }
.log-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.input-with-btn { display: flex; gap: 8px; align-items: center; width: 100%; }
.input-with-btn .t-input { flex: 1; }

.log-output {
  background: #1a2332;
  color: #e0e0e0;
  font-family: 'Cascadia Code', 'JetBrains Mono', Menlo, monospace;
  font-size: 13px;
  line-height: 1.6;
  padding: 12px;
  border-radius: 6px;
  overflow-y: auto;
  height: calc(100vh - 200px);
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
.log-output--static { height: 400px; }

.version-dialog__current {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
  padding: 8px 12px;
  border-radius: 4px;
}
.version-dialog__current .version-value {
  font-family: 'JetBrains Mono', monospace;
  color: var(--td-text-color-primary);
  margin-left: 4px;
}
</style>
