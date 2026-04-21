<template>
  <div class="deploy-page">
    <div class="deploy-head">
      <div class="deploy-head__title">部署管理</div>
      <div class="deploy-head__actions">
        <NSelect
          v-model:value="filterServerId"
          placeholder="全部服务器"
          clearable
          size="small"
          style="width:200px"
          :options="serverFilterOptions"
        />
        <UiButton variant="primary" size="sm" @click="openCreate">
          <template #icon><Plus :size="14" /></template>
          新建部署
        </UiButton>
      </div>
    </div>

    <UiCard v-if="!loading && filteredApps.length === 0" padding="lg">
      <EmptyBlock description="暂无部署应用，点击右上角新建" />
    </UiCard>

    <div v-else class="app-grid">
      <div
        v-for="app in filteredApps"
        :key="app.id"
        class="app-card"
        :class="`app-card--${app.sync_status || 'idle'}`"
      >
        <div class="app-card__header">
          <div class="app-card__name-row">
            <span class="app-card__name">{{ app.name }}</span>
            <UiBadge :tone="typeTone(app.type)">{{ app.type }}</UiBadge>
          </div>
          <div class="app-card__header-right">
            <UiBadge :tone="syncStatusTone(app.sync_status)">
              {{ syncStatusText(app.sync_status) }}
            </UiBadge>
            <NDropdown
              trigger="click"
              :options="dropdownOptions(app)"
              @select="(key: string) => handleCommand(key, app)"
            >
              <UiIconButton variant="ghost" size="sm">
                <MoreHorizontal :size="14" />
              </UiIconButton>
            </NDropdown>
          </div>
        </div>

        <div class="app-card__body">
          <div class="app-card__server">
            <Server :size="13" />
            <span>{{ serverName(app.server_id) }}</span>
          </div>
          <div class="app-card__version">
            <div class="version-block">
              <span class="version-label">期望</span>
              <span class="version-value" :class="{ 'is-drift': isDrifted(app) }">
                {{ app.desired_version || '—' }}
              </span>
            </div>
            <span class="version-arrow" :class="{ 'is-drift': isDrifted(app) }">→</span>
            <div class="version-block">
              <span class="version-label">实际</span>
              <span class="version-value">{{ app.actual_version || '—' }}</span>
            </div>
          </div>
          <div v-if="isDrifted(app)" class="app-card__drift-hint">
            版本未同步，{{ app.auto_sync ? '将自动更新' : '需手动触发' }}
          </div>
        </div>

        <div class="app-card__footer">
          <UiButton variant="secondary" size="sm" @click="openSetVersion(app)">设置版本</UiButton>
          <UiButton
            :variant="isDrifted(app) ? 'warning' : 'primary'"
            size="sm"
            :loading="syncing === app.id"
            @click="handleSync(app)"
          >立即同步</UiButton>
        </div>
      </div>
    </div>

    <!-- 设置版本弹窗 -->
    <NModal v-model:show="versionDialogVisible" preset="card" title="设置期望版本" style="width: 420px" :bordered="false">
      <div class="ver-current">
        当前实际版本：<span class="version-value">{{ versionTarget?.actual_version || '未部署' }}</span>
      </div>
      <NForm :model="versionForm" label-placement="left" label-width="80" style="margin-top: var(--space-3)">
        <NFormItem label="期望版本">
          <NInput v-model:value="versionForm.desired_version" placeholder="v1.0 / latest / 20240101" autofocus />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="versionDialogVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="versionSaving" @click="saveVersion">确定</UiButton>
        </div>
      </template>
    </NModal>

    <!-- 新建应用弹窗 -->
    <NModal v-model:show="createVisible" preset="card" title="新建部署" style="width: 600px" :bordered="false" @after-leave="resetCreateForm">
      <NForm ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="90">
        <NFormItem label="名称" path="name">
          <NInput v-model:value="createForm.name" placeholder="my-app" />
        </NFormItem>
        <NFormItem label="服务器" path="server_id">
          <NSelect v-model:value="createForm.server_id" placeholder="选择服务器" :options="serverFullOptions" />
        </NFormItem>
        <NFormItem label="应用类型">
          <NRadioGroup v-model:value="createForm.type">
            <NRadio value="docker-compose">Docker Compose</NRadio>
            <NRadio value="docker">Docker</NRadio>
            <NRadio value="native">Native</NRadio>
            <NRadio value="static">Static</NRadio>
          </NRadioGroup>
        </NFormItem>
        <NFormItem label="工作目录">
          <NInput v-model:value="createForm.work_dir" placeholder="/opt/myapp" />
        </NFormItem>
        <NFormItem v-if="createForm.type === 'docker-compose'" label="Compose 文件">
          <NInput v-model:value="createForm.compose_file" placeholder="docker-compose.yml" />
        </NFormItem>
        <NFormItem v-if="createForm.type === 'docker'" label="镜像名">
          <NInput v-model:value="createForm.image_name" placeholder="nginx（不含 tag）" />
        </NFormItem>
        <NFormItem v-if="createForm.type === 'native' || createForm.type === 'docker'" label="部署脚本">
          <NInput v-model:value="createForm.start_cmd" type="textarea" :autosize="{ minRows: 3 }" placeholder="./app --port 8080" />
        </NFormItem>
        <NAlert v-if="createForm.type === 'static'" type="info" :show-icon="false" style="margin-bottom: var(--space-3)">
          静态站点：创建后到详情页上传 dist.zip / dist.tar.gz，部署时自动归档到 <code>releases/&lt;时间戳&gt;/</code> 并切换 <code>current</code> 软链。Nginx 把 root 指向 <code>{{ createForm.work_dir || '&lt;work_dir&gt;' }}/current</code> 即可。
        </NAlert>
        <div class="form-section-label">版本控制</div>
        <NFormItem label="期望版本">
          <NInput v-model:value="createForm.desired_version" placeholder="v1.0 / latest（留空仅保存配置）" />
        </NFormItem>
        <NFormItem label="自动同步">
          <NSwitch v-model:value="createForm.auto_sync" />
          <span class="form-hint">版本变化时自动触发同步</span>
        </NFormItem>
        <NFormItem v-if="createForm.auto_sync" label="检查间隔">
          <NInputNumber v-model:value="createForm.sync_interval" :min="30" :max="3600" :step="30" />
          <span class="form-hint">秒</span>
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="createVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="createSaving" @click="handleCreate">创建</UiButton>
        </div>
      </template>
    </NModal>

    <!-- 应用详情抽屉 -->
    <NDrawer v-model:show="detailVisible" :width="640" placement="right">
      <NDrawerContent v-if="detailApp" :title="detailApp.name" closable>
        <NTabs v-model:value="detailTab" type="line" size="small" animated>
          <NTabPane name="version" tab="版本管理">
            <NForm :model="detailVersionForm" label-placement="left" label-width="90" class="drawer-form">
              <NFormItem label="期望版本">
                <NInput v-model:value="detailVersionForm.desired_version" placeholder="v1.0 / latest" />
              </NFormItem>
              <NFormItem label="实际版本">
                <NInput :value="detailApp.actual_version || '—'" readonly />
              </NFormItem>
              <NFormItem label="自动同步">
                <NSwitch v-model:value="detailVersionForm.auto_sync" />
              </NFormItem>
              <NFormItem v-if="detailVersionForm.auto_sync" label="检查间隔">
                <NInputNumber v-model:value="detailVersionForm.sync_interval" :min="30" :max="3600" :step="30" />
                <span class="form-hint">秒</span>
              </NFormItem>
              <NFormItem :show-label="false">
                <div class="row-actions">
                  <UiButton variant="primary" size="sm" :loading="detailSaving" @click="saveDetailVersion">保存</UiButton>
                </div>
              </NFormItem>
            </NForm>

            <div class="ver-history" v-if="detailApp">
              <div class="ver-history-head">
                <span class="ver-history-title">历史版本（最多 {{ 7 }} 条）</span>
                <UiButton size="sm" variant="ghost" :loading="versionsLoading" @click="loadDetailVersions">刷新</UiButton>
              </div>
              <NDataTable
                :columns="versionColumns"
                :data="detailVersions"
                :loading="versionsLoading"
                :row-key="(r: DeployVersion) => r.id"
                size="small"
                :bordered="false"
                :max-height="260"
              />
            </div>
          </NTabPane>

          <NTabPane v-if="detailApp?.type === 'static'" name="upload" tab="上传产物">
            <div class="upload-tip">
              支持 <code>.zip</code> / <code>.tar.gz</code> / <code>.tgz</code>。上传后点击"立即同步"完成归档切换。
            </div>
            <div class="upload-row">
              <input ref="uploadInputRef" type="file" accept=".zip,.tar.gz,.tgz" class="upload-file" @change="onUploadPick" />
              <UiButton
                variant="primary"
                size="sm"
                :loading="uploading"
                :disabled="!pendingFile"
                @click="doUpload"
              >开始上传</UiButton>
            </div>
            <NProgress
              v-if="uploading || uploadDone"
              :percentage="uploadPct"
              :status="uploadDone ? 'success' : 'default'"
              style="margin-top: var(--space-3)"
            />
            <div v-if="uploadLog.length" class="upload-log">
              <div v-for="(l, i) in uploadLog" :key="i">{{ l }}</div>
            </div>
          </NTabPane>

          <NTabPane name="config" tab="应用配置">
            <NForm :model="detailConfigForm" label-placement="left" label-width="90" class="drawer-form">
              <NFormItem label="服务器">
                <NSelect v-model:value="detailConfigForm.server_id" :options="serverFullOptions" />
              </NFormItem>
              <NFormItem label="应用类型">
                <NRadioGroup v-model:value="detailConfigForm.type">
                  <NRadio value="docker-compose">Docker Compose</NRadio>
                  <NRadio value="docker">Docker</NRadio>
                  <NRadio value="native">Native</NRadio>
                  <NRadio value="static">Static</NRadio>
                </NRadioGroup>
              </NFormItem>
              <NFormItem label="工作目录">
                <NInput v-model:value="detailConfigForm.work_dir" />
              </NFormItem>
              <NFormItem v-if="detailConfigForm.type === 'docker-compose'" label="Compose 文件">
                <NInput v-model:value="detailConfigForm.compose_file" />
              </NFormItem>
              <NFormItem v-if="detailConfigForm.type === 'docker'" label="镜像名">
                <NInput v-model:value="detailConfigForm.image_name" />
              </NFormItem>
              <NFormItem v-if="detailConfigForm.type === 'native' || detailConfigForm.type === 'docker'" label="部署脚本">
                <NInput v-model:value="detailConfigForm.start_cmd" type="textarea" :autosize="{ minRows: 4 }" />
              </NFormItem>
              <NAlert v-if="detailConfigForm.type === 'static'" type="info" :show-icon="false" style="margin-bottom: var(--space-3)">
                静态站点：上传 dist.zip / dist.tar.gz 后点击"立即同步"即可完成归档与软链切换。
              </NAlert>
              <NFormItem :show-label="false">
                <UiButton variant="primary" size="sm" :loading="detailSaving" @click="saveDetailConfig">保存配置</UiButton>
              </NFormItem>
            </NForm>
          </NTabPane>

          <NTabPane name="env" tab="环境变量">
            <div class="env-toolbar">
              <UiButton variant="primary" size="sm" @click="addEnvRow">
                <template #icon><Plus :size="14" /></template>
                添加变量
              </UiButton>
            </div>
            <div class="env-list">
              <div v-for="(row, idx) in envVars" :key="idx" class="env-row">
                <NInput v-model:value="row.key" size="small" placeholder="VAR_NAME" style="width:30%" />
                <NInput
                  v-model:value="row.value"
                  size="small"
                  :type="row.secret ? 'password' : 'text'"
                  show-password-on="click"
                  placeholder="value"
                  style="flex:1"
                />
                <NCheckbox v-model:checked="row.secret">Secret</NCheckbox>
                <UiIconButton variant="ghost" size="sm" @click="envVars.splice(idx, 1)">
                  <Trash2 :size="13" />
                </UiIconButton>
              </div>
              <EmptyBlock v-if="envVars.length === 0" description="暂无环境变量" />
            </div>
            <div style="margin-top: var(--space-3)">
              <UiButton variant="primary" size="sm" :loading="envSaving" @click="saveEnv">保存环境变量</UiButton>
            </div>
          </NTabPane>

          <NTabPane name="history" tab="同步历史">
            <NDataTable
              :columns="historyColumns"
              :data="historyLogs"
              :row-key="(r: DeployLog) => r.id"
              size="small"
              :bordered="false"
            />
          </NTabPane>

          <NTabPane name="webhook" tab="Webhook">
            <NForm label-placement="left" label-width="110" class="drawer-form">
              <NFormItem label="Webhook URL">
                <div class="input-with-btn">
                  <NInput v-model:value="webhookUrl" readonly />
                  <UiButton variant="secondary" size="sm" @click="copyWebhook">复制</UiButton>
                </div>
              </NFormItem>
              <NFormItem label="Secret Token">
                <NInput v-model:value="webhookSecret" readonly type="password" show-password-on="click" />
              </NFormItem>
            </NForm>
            <NAlert type="info" :show-icon="true" style="margin-top: var(--space-3)">
              Webhook 收到 POST 请求后将自动触发同步。支持 GitHub / GitLab 签名验证。
            </NAlert>
          </NTabPane>
        </NTabs>
      </NDrawerContent>
    </NDrawer>

    <!-- 同步日志抽屉 -->
    <NDrawer v-model:show="logDrawerVisible" :width="640" placement="right" @after-leave="stopSync">
      <NDrawerContent :title="`同步日志 — ${logAppName}`" closable>
        <div class="log-toolbar">
          <UiBadge :tone="runStatus === 'success' ? 'success' : runStatus === 'failed' ? 'danger' : 'brand'">
            {{ runStatus === 'running' ? '同步中…' : runStatus === 'success' ? '成功' : runStatus === 'failed' ? '失败' : '就绪' }}
          </UiBadge>
          <UiButton variant="ghost" size="sm" @click="logLines = []">清空</UiButton>
        </div>
        <pre class="log-output" ref="logEl">{{ logLines.join('\n') }}</pre>
      </NDrawerContent>
    </NDrawer>

    <!-- 日志详情弹窗 -->
    <NModal v-model:show="logDetailVisible" preset="card" title="执行日志" style="width: 720px" :bordered="false">
      <pre class="log-output log-output--static">{{ selectedLog?.output }}</pre>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, nextTick, onMounted, onBeforeUnmount, watch, h } from 'vue'
import {
  NSelect, NDataTable, NModal, NDrawer, NDrawerContent, NTabs, NTabPane,
  NForm, NFormItem, NInput, NInputNumber, NRadioGroup, NRadio, NSwitch,
  NCheckbox, NDropdown, NAlert, NProgress, useMessage, useDialog,
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { Plus, Trash2, MoreHorizontal, Server } from 'lucide-vue-next'
import dayjs from 'dayjs'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  getDeploys, createDeploy, updateDeploy, deleteDeploy,
  getDeployLogs, getDeployEnv, putDeployEnv, getWebhookInfo,
  getDeployVersions,
} from '@/api/deploy'
import type { EnvVar } from '@/api/deploy'
import type { Server as ServerType, Deploy, DeployForm, DeployLog, DeployVersion } from '@/types/api'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiIconButton from '@/components/ui/UiIconButton.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const authStore = useAuthStore()
const message = useMessage()
const dialog = useDialog()

const apps = ref<Deploy[]>([])
const servers = ref<ServerType[]>([])
const loading = ref(false)
const filterServerId = ref<number | null>(null)

const filteredApps = computed(() =>
  filterServerId.value ? apps.value.filter(a => a.server_id === filterServerId.value) : apps.value
)
const serverFilterOptions = computed(() =>
  servers.value.map(s => ({ label: s.name, value: s.id })))
const serverFullOptions = computed(() =>
  servers.value.map(s => ({ label: `${s.name} (${s.host})`, value: s.id })))

function serverName(id: number) {
  return servers.value.find(s => s.id === id)?.name ?? `#${id}`
}
function isDrifted(app: Deploy) {
  return !!(app.desired_version && app.desired_version !== app.actual_version)
}
type Tone = 'brand' | 'success' | 'warning' | 'danger' | 'neutral'
function typeTone(type: string): Tone {
  return ({ 'docker-compose': 'brand', docker: 'success', native: 'warning', static: 'neutral' } as Record<string, Tone>)[type] ?? 'neutral'
}
function syncStatusTone(s: Deploy['sync_status']): Tone {
  return ({ synced: 'success', drifted: 'warning', syncing: 'brand', error: 'danger' } as Record<string, Tone>)[s ?? ''] ?? 'neutral'
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

function dropdownOptions(_app: Deploy) {
  const items: any[] = [
    { label: '应用详情', key: 'detail' },
    { label: '环境变量', key: 'env' },
    { label: '同步历史', key: 'history' },
    { label: 'Webhook', key: 'webhook' },
    { type: 'divider', key: 'd2' },
    { label: '删除', key: 'delete', props: { style: 'color: var(--ui-danger-fg)' } },
  ]
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
    message.success('期望版本已更新')
    versionDialogVisible.value = false
    await loadAll()
  } finally {
    versionSaving.value = false
  }
}

// ── Create App ────────────────────────────────────────────────
const createVisible = ref(false)
const createSaving = ref(false)
const createFormRef = ref<FormInst | null>(null)
const defaultCreateForm = (): DeployForm => ({
  name: '', server_id: null, type: 'docker-compose',
  work_dir: '', compose_file: 'docker-compose.yml',
  start_cmd: '', image_name: '',
  desired_version: '', auto_sync: false, sync_interval: 60,
})
const createForm = reactive<DeployForm>(defaultCreateForm())
const createRules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  server_id: [{ required: true, message: '请选择服务器', type: 'number', trigger: 'change' }],
}

function openCreate() {
  Object.assign(createForm, defaultCreateForm())
  createVisible.value = true
}
function resetCreateForm() { createFormRef.value?.restoreValidation() }

async function handleCreate() {
  try { await createFormRef.value?.validate() } catch { return }
  createSaving.value = true
  try {
    await createDeploy(createForm)
    message.success('应用已创建')
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
  detailVersions.value = []
  if (tab === 'version') await loadDetailVersions()
  if (tab === 'env') await loadEnv(app.id)
  if (tab === 'history') await loadHistory(app.id)
  if (tab === 'webhook') await loadWebhook(app.id)
  detailVisible.value = true
}

watch(detailTab, async (tab) => {
  if (!detailApp.value) return
  if (tab === 'version' && detailVersions.value.length === 0) await loadDetailVersions()
  if (tab === 'env' && envVars.value.length === 0) await loadEnv(detailApp.value.id)
  if (tab === 'history') await loadHistory(detailApp.value.id)
  if (tab === 'webhook' && !webhookUrl.value) await loadWebhook(detailApp.value.id)
})

async function saveDetailVersion() {
  if (!detailApp.value) return
  detailSaving.value = true
  try {
    await updateDeploy(detailApp.value.id, toUpdateForm(detailApp.value, detailVersionForm))
    message.success('版本配置已保存')
    await loadAll()
    detailApp.value = apps.value.find(a => a.id === detailApp.value!.id) ?? detailApp.value
  } finally { detailSaving.value = false }
}

async function saveDetailConfig() {
  if (!detailApp.value) return
  detailSaving.value = true
  try {
    await updateDeploy(detailApp.value.id, toUpdateForm(detailApp.value, detailConfigForm))
    message.success('应用配置已保存')
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
    message.success('环境变量已保存')
  } finally { envSaving.value = false }
}

// ── History ───────────────────────────────────────────────────
const historyLogs = ref<DeployLog[]>([])
const historyColumns: DataTableColumns<DeployLog> = [
  { title: '时间', key: 'created_at', width: 160, render: (row) => dayjs(row.created_at).format('MM-DD HH:mm:ss') },
  {
    title: '状态', key: 'status', width: 80,
    render: (row) => h(UiBadge, { tone: row.status === 'success' ? 'success' : 'danger' as Tone },
      () => row.status === 'success' ? '成功' : '失败'),
  },
  { title: '耗时', key: 'duration', width: 80, render: (row) => `${row.duration}s` },
  {
    title: '', key: 'ops', width: 70,
    render: (row) => h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => viewLogDetail(row) }, () => '日志'),
  },
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
  message.success('已复制')
}

// ── Rollback ──────────────────────────────────────────────────
function handleRollback(app: Deploy) { return runWithSSE(app, 'rollback') }

// ── Version History ───────────────────────────────────────────
const detailVersions = ref<DeployVersion[]>([])
const versionsLoading = ref(false)

async function loadDetailVersions() {
  if (!detailApp.value) return
  versionsLoading.value = true
  try { detailVersions.value = await getDeployVersions(detailApp.value.id) }
  finally { versionsLoading.value = false }
}

function rollbackToVersion(v: DeployVersion) {
  if (!detailApp.value) return
  const app = detailApp.value
  dialog.warning({
    title: '回滚确认',
    content: `确认将「${app.name}」回滚到版本 ${v.version || '#' + v.id}？将以快照配置重新部署。`,
    positiveText: '确认回滚',
    negativeText: '取消',
    onPositiveClick: async () => {
      detailVisible.value = false
      await runWithSSEPath(app, `versions/${v.id}/rollback`)
      await loadDetailVersions()
    },
  })
}

async function runWithSSEPath(app: Deploy, path: string) {
  logAppName.value = app.name
  syncing.value = app.id
  logLines.value = []
  runStatus.value = 'running'
  logDrawerVisible.value = true
  abortCtrl = new AbortController()
  try {
    const res = await fetch(`/panel/api/v1/deploys/${app.id}/${path}`, {
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

const versionColumns: DataTableColumns<DeployVersion> = [
  { title: '时间', key: 'created_at', width: 140, render: (r) => dayjs(r.created_at).format('MM-DD HH:mm:ss') },
  { title: '版本', key: 'version', width: 120, render: (r) => r.version || '—' },
  {
    title: '触发', key: 'trigger_source', width: 90,
    render: (r) => ({ manual: '手动', webhook: 'Webhook', schedule: '定时', api: 'API', rollback: '回滚' } as Record<string, string>)[r.trigger_source] ?? r.trigger_source,
  },
  {
    title: '状态', key: 'status', width: 70,
    render: (r) => h(UiBadge, { tone: r.status === 'success' ? 'success' : 'danger' as Tone },
      () => r.status === 'success' ? '成功' : '失败'),
  },
  {
    title: '', key: 'ops', width: 80,
    render: (r) => h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => rollbackToVersion(r) }, () => '回滚'),
  },
]

// ── Static Upload ─────────────────────────────────────────────
const uploadInputRef = ref<HTMLInputElement | null>(null)
const pendingFile = ref<File | null>(null)
const uploading = ref(false)
const uploadDone = ref(false)
const uploadPct = ref(0)
const uploadLog = ref<string[]>([])

function onUploadPick(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  pendingFile.value = f
  uploadDone.value = false
  uploadPct.value = 0
  uploadLog.value = []
}

async function doUpload() {
  if (!detailApp.value || !pendingFile.value) return
  const file = pendingFile.value
  uploading.value = true
  uploadDone.value = false
  uploadPct.value = 0
  uploadLog.value = [`上传 ${file.name} ...`]
  try {
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch(`/panel/api/v1/deploys/${detailApp.value.id}/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.token}` },
      body: fd,
    })
    if (!res.body) throw new Error('no body')
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
          const evt = JSON.parse(line.slice(6))
          if (evt.type === 'progress' && evt.total) {
            uploadPct.value = Math.round((evt.bytes / evt.total) * 100)
          } else if (evt.type === 'done') {
            uploadDone.value = true
            uploadPct.value = 100
            uploadLog.value.push(`已上传到 ${evt.path}`)
          } else if (evt.type === 'error') {
            uploadLog.value.push('[错误] ' + evt.msg)
          }
        } catch {}
      }
    }
    if (uploadDone.value) message.success('上传成功，点击"立即同步"部署')
  } catch (e) {
    uploadLog.value.push('[异常] ' + String(e))
    message.error('上传失败')
  } finally {
    uploading.value = false
    pendingFile.value = null
    if (uploadInputRef.value) uploadInputRef.value.value = ''
  }
}

// ── Delete ────────────────────────────────────────────────────
function handleDelete(app: Deploy) {
  dialog.warning({
    title: '删除确认',
    content: `确认删除应用「${app.name}」？此操作不可恢复。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await deleteDeploy(app.id)
      message.success('已删除')
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
onBeforeUnmount(() => {
  // Abort any in-flight SSE reader so the component tears down cleanly.
  abortCtrl?.abort()
})
</script>

<style scoped>
.deploy-page {
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.deploy-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.deploy-head__title {
  font-size: var(--fs-lg);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}
.deploy-head__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: var(--space-4);
}

.app-card {
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-left: 3px solid var(--ui-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: box-shadow var(--dur-fast) var(--ease), border-left-color var(--dur-fast) var(--ease);
}
.app-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}
.app-card--synced  { border-left-color: var(--ui-success); }
.app-card--drifted { border-left-color: var(--ui-warning); }
.app-card--syncing { border-left-color: var(--ui-brand); }
.app-card--error   { border-left-color: var(--ui-danger); }

.app-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3) var(--space-4) var(--space-2);
  border-bottom: 1px solid var(--ui-border);
}
.app-card__name-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  flex: 1;
}
.app-card__name {
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.app-card__header-right {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  flex-shrink: 0;
}

.app-card__body {
  padding: var(--space-3) var(--space-4);
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.app-card__server {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.app-card__version {
  display: flex;
  align-items: center;
  justify-content: space-around;
  background: var(--ui-bg-1);
  border-radius: var(--radius-sm);
  padding: var(--space-2);
}
.version-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.version-label {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.version-value {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  font-weight: var(--fw-medium);
  color: var(--ui-fg);
}
.version-value.is-drift { color: var(--ui-warning-fg); }
.version-arrow {
  color: var(--ui-fg-4);
  font-size: var(--fs-md);
}
.version-arrow.is-drift { color: var(--ui-warning-fg); }
.app-card__drift-hint {
  font-size: var(--fs-xs);
  color: var(--ui-warning-fg);
}

.app-card__footer {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-1);
}

.ver-current {
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  background: var(--ui-bg-1);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
}
.ver-current .version-value {
  margin-left: var(--space-1);
}

.modal-foot {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.form-section-label {
  font-size: var(--fs-xs);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: var(--space-2) 0 var(--space-1);
  border-bottom: 1px solid var(--ui-border);
  margin-bottom: var(--space-3);
}
.form-hint {
  margin-left: var(--space-2);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}

.drawer-form { max-width: 520px; }
.row-actions { display: flex; gap: var(--space-2); }

.env-toolbar { margin-bottom: var(--space-3); }
.env-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.env-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.input-with-btn {
  display: flex;
  gap: var(--space-2);
  align-items: center;
  width: 100%;
}

.log-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-2);
}

.log-output {
  background: #0A0A0A;
  color: #E4E4E7;
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  line-height: 1.6;
  padding: var(--space-3);
  border-radius: var(--radius-sm);
  overflow-y: auto;
  height: calc(100vh - 240px);
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
.log-output--static { height: 420px; }

.upload-tip {
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  margin-bottom: var(--space-3);
}
.upload-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.upload-file {
  flex: 1;
  font-size: var(--fs-sm);
}
.upload-log {
  margin-top: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--ui-bg-1);
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg);
  max-height: 160px;
  overflow: auto;
}
</style>
