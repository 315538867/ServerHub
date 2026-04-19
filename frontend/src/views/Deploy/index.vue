<template>
  <div class="apps-page">
    <div class="page-header">
      <h2>应用管理</h2>
      <div class="header-right">
        <el-select v-model="filterServerId" placeholder="全部服务器" clearable style="width:180px;margin-right:12px">
          <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建应用</el-button>
      </div>
    </div>

    <el-empty v-if="!loading && filteredApps.length === 0" description="暂无应用，点击右上角新建" style="margin-top:60px" />

    <el-row v-else :gutter="16" v-loading="loading" class="app-grid">
      <el-col
        v-for="app in filteredApps"
        :key="app.id"
        :xs="24" :sm="12" :lg="8" :xl="6"
        style="margin-bottom:16px"
      >
        <div class="app-card" :class="`app-card--${app.sync_status || 'idle'}`">
          <div class="app-card__header">
            <div class="app-card__title">
              <span class="app-card__name">{{ app.name }}</span>
              <el-tag size="small" :type="typeTagType(app.type)">{{ app.type }}</el-tag>
            </div>
            <el-tag size="small" :type="syncStatusTagType(app.sync_status)">
              <el-icon v-if="app.sync_status === 'syncing'" class="is-loading"><Loading /></el-icon>
              {{ syncStatusText(app.sync_status) }}
            </el-tag>
          </div>

          <div class="app-card__server">
            <el-icon><Monitor /></el-icon>
            {{ serverName(app.server_id) }}
          </div>

          <div class="app-card__version">
            <div class="version-block">
              <span class="version-label">期望</span>
              <span class="version-value" :class="{ 'version-value--drifted': isDrifted(app) }">
                {{ app.desired_version || '—' }}
              </span>
            </div>
            <div class="version-arrow" :class="{ 'version-arrow--drifted': isDrifted(app) }">
              <el-icon><Right /></el-icon>
            </div>
            <div class="version-block">
              <span class="version-label">实际</span>
              <span class="version-value">{{ app.actual_version || '—' }}</span>
            </div>
          </div>

          <div v-if="isDrifted(app)" class="app-card__drift-hint">
            <el-icon><WarningFilled /></el-icon>
            版本未同步，{{ app.auto_sync ? '将自动更新' : '需手动触发' }}
          </div>

          <div class="app-card__actions">
            <el-button type="primary" size="small" @click="openSetVersion(app)">设置版本</el-button>
            <el-button
              size="small"
              :type="isDrifted(app) ? 'warning' : ''"
              :loading="syncing === app.id"
              @click="handleSync(app)"
            >立即同步</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleCommand(cmd, app)">
              <el-button size="small" :icon="More" circle plain />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="detail">应用详情</el-dropdown-item>
                  <el-dropdown-item command="env">环境变量</el-dropdown-item>
                  <el-dropdown-item command="history">同步历史</el-dropdown-item>
                  <el-dropdown-item command="webhook">Webhook</el-dropdown-item>
                  <el-dropdown-item v-if="app.previous_version" command="rollback" divided>
                    回滚到 {{ app.previous_version }}
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided class="danger-item">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- ── Set Version Dialog ──────────────────────────────── -->
    <el-dialog v-model="versionDialogVisible" title="设置期望版本" width="420px" @closed="versionForm.desired_version = ''">
      <div class="version-dialog__current">
        当前实际版本：<span class="version-value">{{ versionTarget?.actual_version || '未部署' }}</span>
      </div>
      <el-form :model="versionForm" style="margin-top:16px">
        <el-form-item label="期望版本">
          <el-input v-model="versionForm.desired_version" placeholder="v1.0 / latest / 20240101" autofocus />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="versionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="versionSaving" @click="saveVersion">确定</el-button>
      </template>
    </el-dialog>

    <!-- ── Create App Dialog ───────────────────────────────── -->
    <el-dialog v-model="createVisible" title="新建应用" width="600px" @closed="resetCreateForm">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="90px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="createForm.name" placeholder="my-app" />
        </el-form-item>
        <el-form-item label="服务器" prop="server_id">
          <el-select v-model="createForm.server_id" placeholder="选择服务器" style="width:100%">
            <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="应用类型">
          <el-radio-group v-model="createForm.type">
            <el-radio value="docker-compose">Docker Compose</el-radio>
            <el-radio value="docker">Docker</el-radio>
            <el-radio value="native">Native</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="工作目录">
          <el-input v-model="createForm.work_dir" placeholder="/opt/myapp" />
        </el-form-item>
        <el-form-item v-if="createForm.type === 'docker-compose'" label="Compose 文件">
          <el-input v-model="createForm.compose_file" placeholder="docker-compose.yml" />
        </el-form-item>
        <el-form-item v-if="createForm.type === 'docker'" label="镜像名">
          <el-input v-model="createForm.image_name" placeholder="nginx（不含 tag）" />
        </el-form-item>
        <el-form-item v-if="createForm.type !== 'docker-compose'" label="部署脚本">
          <el-input
            v-model="createForm.start_cmd"
            type="textarea"
            :rows="3"
            :placeholder="createForm.type === 'docker'
              ? 'docker pull nginx:${VERSION} && docker stop app || true && docker run -d --name app nginx:${VERSION}'
              : './app --port 8080'"
          />
        </el-form-item>
        <el-divider content-position="left">版本控制</el-divider>
        <el-form-item label="期望版本">
          <el-input v-model="createForm.desired_version" placeholder="v1.0 / latest（留空仅保存配置）" />
        </el-form-item>
        <el-form-item label="自动同步">
          <el-switch v-model="createForm.auto_sync" />
          <span class="form-hint">版本变化时自动触发同步</span>
        </el-form-item>
        <el-form-item v-if="createForm.auto_sync" label="检查间隔">
          <el-input-number v-model="createForm.sync_interval" :min="30" :max="3600" :step="30" />
          <span class="form-hint">秒</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="createSaving" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- ── App Detail Drawer ───────────────────────────────── -->
    <el-drawer v-model="detailVisible" :title="detailApp?.name" size="55%" direction="rtl">
      <el-tabs v-if="detailApp" v-model="detailTab">

        <el-tab-pane label="版本管理" name="version">
          <el-form :model="detailVersionForm" label-width="90px" style="max-width:480px">
            <el-form-item label="期望版本">
              <el-input v-model="detailVersionForm.desired_version" placeholder="v1.0 / latest" />
            </el-form-item>
            <el-form-item label="实际版本">
              <el-input :model-value="detailApp.actual_version || '—'" readonly />
            </el-form-item>
            <el-form-item label="历史版本">
              <el-input :model-value="detailApp.previous_version || '—'" readonly />
            </el-form-item>
            <el-form-item label="自动同步">
              <el-switch v-model="detailVersionForm.auto_sync" />
            </el-form-item>
            <el-form-item v-if="detailVersionForm.auto_sync" label="检查间隔">
              <el-input-number v-model="detailVersionForm.sync_interval" :min="30" :max="3600" :step="30" />
              <span class="form-hint">秒</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="detailSaving" @click="saveDetailVersion">保存</el-button>
              <el-button
                v-if="detailApp.previous_version"
                type="warning"
                style="margin-left:12px"
                @click="handleRollbackFromDetail"
              >回滚到 {{ detailApp.previous_version }}</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="应用配置" name="config">
          <el-form :model="detailConfigForm" label-width="90px" style="max-width:480px">
            <el-form-item label="服务器">
              <el-select v-model="detailConfigForm.server_id" style="width:100%">
                <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="应用类型">
              <el-radio-group v-model="detailConfigForm.type">
                <el-radio value="docker-compose">Docker Compose</el-radio>
                <el-radio value="docker">Docker</el-radio>
                <el-radio value="native">Native</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="工作目录">
              <el-input v-model="detailConfigForm.work_dir" />
            </el-form-item>
            <el-form-item v-if="detailConfigForm.type === 'docker-compose'" label="Compose 文件">
              <el-input v-model="detailConfigForm.compose_file" />
            </el-form-item>
            <el-form-item v-if="detailConfigForm.type === 'docker'" label="镜像名">
              <el-input v-model="detailConfigForm.image_name" />
            </el-form-item>
            <el-form-item v-if="detailConfigForm.type !== 'docker-compose'" label="部署脚本">
              <el-input v-model="detailConfigForm.start_cmd" type="textarea" :rows="4" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="detailSaving" @click="saveDetailConfig">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="环境变量" name="env">
          <div class="env-toolbar">
            <el-button size="small" :icon="Plus" @click="addEnvRow">添加变量</el-button>
          </div>
          <el-table :data="envVars" size="small">
            <el-table-column label="Key" min-width="150">
              <template #default="{ row }">
                <el-input v-model="row.key" size="small" placeholder="VAR_NAME" />
              </template>
            </el-table-column>
            <el-table-column label="Value" min-width="180">
              <template #default="{ row }">
                <el-input v-model="row.value" size="small" :type="row.secret ? 'password' : 'text'" show-password placeholder="value" />
              </template>
            </el-table-column>
            <el-table-column label="Secret" width="70" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.secret" />
              </template>
            </el-table-column>
            <el-table-column width="50" align="center">
              <template #default="{ $index }">
                <el-button :icon="Delete" circle size="small" type="danger" plain @click="envVars.splice($index, 1)" />
              </template>
            </el-table-column>
          </el-table>
          <div style="margin-top:12px">
            <el-button type="primary" :loading="envSaving" @click="saveEnv">保存环境变量</el-button>
          </div>
        </el-tab-pane>

        <el-tab-pane label="同步历史" name="history">
          <el-table :data="historyLogs" size="small" border>
            <el-table-column label="时间" width="155">
              <template #default="{ row }">{{ dayjs(row.created_at).format('MM-DD HH:mm:ss') }}</template>
            </el-table-column>
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">
                  {{ row.status === 'success' ? '成功' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="70">
              <template #default="{ row }">{{ row.duration }}s</template>
            </el-table-column>
            <el-table-column label="" width="60">
              <template #default="{ row }">
                <el-button link size="small" @click="viewLogDetail(row)">日志</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="Webhook" name="webhook">
          <el-form label-width="110px" size="small" style="max-width:500px">
            <el-form-item label="Webhook URL">
              <el-input v-model="webhookUrl" readonly>
                <template #append>
                  <el-button @click="copyWebhook">复制</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="Secret Token">
              <el-input v-model="webhookSecret" readonly type="password" show-password />
            </el-form-item>
          </el-form>
          <el-alert type="info" :closable="false" style="margin-top:8px">
            Webhook 收到 POST 请求后将自动触发同步。支持 GitHub / GitLab 签名验证。
          </el-alert>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <!-- ── Sync Log Drawer (SSE) ───────────────────────────── -->
    <el-drawer v-model="logDrawerVisible" :title="`同步日志 — ${logAppName}`" size="55%" direction="rtl" @close="stopSync">
      <div class="log-toolbar">
        <el-tag :type="runStatus === 'success' ? 'success' : runStatus === 'failed' ? 'danger' : 'info'" size="small">
          {{ runStatus === 'running' ? '同步中…' : runStatus === 'success' ? '成功' : runStatus === 'failed' ? '失败' : '就绪' }}
        </el-tag>
        <el-button size="small" link @click="logLines = []">清空</el-button>
      </div>
      <pre class="log-output" ref="logEl">{{ logLines.join('\n') }}</pre>
    </el-drawer>

    <!-- ── Log Detail Dialog ───────────────────────────────── -->
    <el-dialog v-model="logDetailVisible" title="执行日志" width="720px">
      <pre class="log-output log-output--static">{{ selectedLog?.output }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, nextTick, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete, More, Right, Monitor, WarningFilled, Loading } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
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
function typeTagType(type: string) {
  return ({ 'docker-compose': 'primary', docker: 'success', native: 'warning' } as Record<string, string>)[type] ?? ''
}
function syncStatusTagType(s: Deploy['sync_status']) {
  return ({ synced: 'success', drifted: 'warning', syncing: '', error: 'danger', '': 'info' } as Record<string, string>)[s ?? '']
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
    ElMessage.success('期望版本已更新')
    versionDialogVisible.value = false
    await loadAll()
  } finally {
    versionSaving.value = false
  }
}

// ── Create App ────────────────────────────────────────────────
const createVisible = ref(false)
const createSaving = ref(false)
const createFormRef = ref<FormInstance>()
const defaultCreateForm = (): DeployForm => ({
  name: '', server_id: null, type: 'docker-compose',
  work_dir: '', compose_file: 'docker-compose.yml',
  start_cmd: '', image_name: '',
  desired_version: '', auto_sync: false, sync_interval: 60,
})
const createForm = reactive<DeployForm>(defaultCreateForm())
const createRules: FormRules = {
  name: [{ required: true, message: '请输入名称' }],
  server_id: [{ required: true, message: '请选择服务器' }],
}

function openCreate() {
  Object.assign(createForm, defaultCreateForm())
  createVisible.value = true
}
function resetCreateForm() { createFormRef.value?.clearValidate() }

async function handleCreate() {
  await createFormRef.value?.validate()
  createSaving.value = true
  try {
    await createDeploy(createForm)
    ElMessage.success('应用已创建')
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
    ElMessage.success('版本配置已保存')
    await loadAll()
    detailApp.value = apps.value.find(a => a.id === detailApp.value!.id) ?? detailApp.value
  } finally { detailSaving.value = false }
}

async function saveDetailConfig() {
  if (!detailApp.value) return
  detailSaving.value = true
  try {
    await updateDeploy(detailApp.value.id, toUpdateForm(detailApp.value, detailConfigForm))
    ElMessage.success('应用配置已保存')
    await loadAll()
  } finally { detailSaving.value = false }
}

// ── Env ───────────────────────────────────────────────────────
const envVars = ref<EnvVar[]>([])
const envSaving = ref(false)

async function loadEnv(id: number) { envVars.value = await getDeployEnv(id) }
function addEnvRow() { envVars.value.push({ key: '', value: '', secret: false }) }
async function saveEnv() {
  if (!detailApp.value) return
  envSaving.value = true
  try {
    await putDeployEnv(detailApp.value.id, envVars.value)
    ElMessage.success('环境变量已保存')
  } finally { envSaving.value = false }
}

// ── History ───────────────────────────────────────────────────
const historyLogs = ref<DeployLog[]>([])
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
  ElMessage.success('已复制')
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
async function handleDelete(app: Deploy) {
  await ElMessageBox.confirm(`确认删除应用「${app.name}」？此操作不可恢复。`, '删除确认', {
    type: 'warning', confirmButtonText: '确认删除', confirmButtonClass: 'el-button--danger',
  })
  await deleteDeploy(app.id)
  ElMessage.success('已删除')
  await loadAll()
}

// ── Command dispatcher ────────────────────────────────────────
async function handleCommand(cmd: string, app: Deploy) {
  switch (cmd) {
    case 'detail':   await openDetail(app, 'version'); break
    case 'env':      await openDetail(app, 'env'); break
    case 'history':  await openDetail(app, 'history'); break
    case 'webhook':  await openDetail(app, 'webhook'); break
    case 'rollback': await handleRollback(app); break
    case 'delete':   await handleDelete(app); break
  }
}

onMounted(loadAll)
</script>

<style scoped>
.apps-page { padding: 20px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.page-header h2 { margin: 0; font-size: 18px; }
.header-right { display: flex; align-items: center; }

.app-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-left: 4px solid var(--el-border-color);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: box-shadow 0.2s, border-left-color 0.3s;
  min-height: 180px;
}
.app-card:hover { box-shadow: var(--el-box-shadow-light); }
.app-card--synced  { border-left-color: var(--el-color-success); }
.app-card--drifted { border-left-color: var(--el-color-warning); }
.app-card--syncing { border-left-color: var(--el-color-primary); }
.app-card--error   { border-left-color: var(--el-color-danger); }
.app-card--idle    { border-left-color: var(--el-border-color); }

.app-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}
.app-card__title { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; min-width: 0; }
.app-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-card__server {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.app-card__version {
  display: flex;
  align-items: center;
  justify-content: space-around;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 10px 12px;
}
.version-block { display: flex; flex-direction: column; align-items: center; gap: 3px; }
.version-label { font-size: 11px; color: var(--el-text-color-secondary); }
.version-value {
  font-family: 'JetBrains Mono', 'Cascadia Code', Menlo, monospace;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}
.version-value--drifted { color: var(--el-color-warning); }
.version-arrow { color: var(--el-text-color-placeholder); font-size: 18px; display: flex; align-items: center; }
.version-arrow--drifted { color: var(--el-color-warning); }

.app-card__drift-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--el-color-warning);
}

.app-card__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: auto;
  padding-top: 2px;
}

.env-toolbar { margin-bottom: 10px; }
.form-hint { margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary); }
.log-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }

.log-output {
  background: #1a1a2e;
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
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  padding: 8px 12px;
  border-radius: 4px;
}
.version-dialog__current .version-value {
  font-family: 'JetBrains Mono', monospace;
  color: var(--el-text-color-primary);
  margin-left: 4px;
}

:deep(.danger-item) { color: var(--el-color-danger) !important; }
</style>
