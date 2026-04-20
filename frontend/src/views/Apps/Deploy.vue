<template>
  <div class="page-container">

    <!-- 加载中 -->
    <div v-if="loading" class="section-block empty-block">
      <t-loading />
    </div>

    <!-- ===== 向导：未关联部署配置 ===== -->
    <template v-else-if="!app?.deploy_id">
      <div class="wizard-card">
        <div class="wizard-header">
          <div class="wizard-title">选择部署方式</div>
          <div class="wizard-subtitle">为该应用创建一个部署配置，配置完成后即可一键部署</div>
        </div>

        <div class="type-cards">
          <div class="type-card" :class="{ active: wizardType === 'docker-compose' }" @click="wizardType = 'docker-compose'">
            <div class="type-card-icon">⚙️</div>
            <div class="type-card-title">Docker Compose</div>
            <div class="type-card-desc">使用 docker-compose.yml 编排多个容器，支持 pull + up -d</div>
          </div>
          <div class="type-card" :class="{ active: wizardType === 'docker' }" @click="wizardType = 'docker'">
            <div class="type-card-icon">🐳</div>
            <div class="type-card-title">Docker 单容器</div>
            <div class="type-card-desc">拉取指定镜像，执行 docker run 命令启动单个容器</div>
          </div>
          <div class="type-card" :class="{ active: wizardType === 'native' }" @click="wizardType = 'native'">
            <div class="type-card-icon">📦</div>
            <div class="type-card-title">文件部署</div>
            <div class="type-card-desc">上传可执行文件（jar / binary / zip）到服务器并运行</div>
          </div>
        </div>

        <template v-if="wizardType">
          <div class="wizard-divider">基础配置</div>
          <div class="form-grid wizard-form-grid">
            <div class="form-field">
              <label class="form-label">工作目录 <span class="form-required">*</span></label>
              <t-input v-model="wizardForm.work_dir" :placeholder="app ? `/srv/apps/${app.name}` : '/srv/apps/myapp'" />
              <span class="form-hint">在远程服务器上的项目根目录</span>
            </div>
            <template v-if="wizardType === 'docker-compose'">
              <div class="form-field">
                <label class="form-label">Compose 文件</label>
                <t-input v-model="wizardForm.compose_file" placeholder="docker-compose.yml" />
              </div>
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <t-input v-model="wizardForm.image_name" placeholder="nginx（版本追踪用，可选）" />
              </div>
              <div class="form-field">
                <label class="form-label">期望版本</label>
                <t-input v-model="wizardForm.desired_version" placeholder="latest（可选）" />
              </div>
            </template>
            <template v-if="wizardType === 'docker'">
              <div class="form-field">
                <label class="form-label">镜像名称 <span class="form-required">*</span></label>
                <t-input v-model="wizardForm.image_name" placeholder="nginx" />
              </div>
              <div class="form-field">
                <label class="form-label">期望版本 <span class="form-required">*</span></label>
                <t-input v-model="wizardForm.desired_version" placeholder="latest" />
              </div>
              <div class="form-field form-field--full">
                <label class="form-label">启动命令 <span class="form-required">*</span></label>
                <t-input v-model="wizardForm.start_cmd" placeholder="docker run -d --name myapp -p 8080:8080 nginx:latest" />
                <span class="form-hint">完整的 docker run 命令</span>
              </div>
            </template>
            <template v-if="wizardType === 'native'">
              <div class="form-field form-field--full">
                <label class="form-label">启动命令 <span class="form-required">*</span></label>
                <t-input v-model="wizardForm.start_cmd" placeholder="./app --port 8080" />
                <span class="form-hint">上传文件后在工作目录执行此命令</span>
              </div>
            </template>
          </div>
          <div class="wizard-footer">
            <t-button theme="primary" size="large" :loading="creating" @click="createAndLink">创建并关联部署配置</t-button>
          </div>
        </template>
      </div>
    </template>

    <!-- ===== 主视图：已关联部署配置 ===== -->
    <template v-else-if="deploy">

      <!-- S1: 配置信息 -->
      <div class="section-block">
        <div class="section-title">
          <span>配置信息</span>
          <t-space v-if="!editMode" size="small">
            <t-button size="small" variant="outline" @click="startEdit">编辑</t-button>
          </t-space>
          <t-space v-else size="small">
            <t-button size="small" theme="primary" :loading="saving" @click="saveEdit">保存</t-button>
            <t-button size="small" variant="outline" @click="cancelEdit">取消</t-button>
          </t-space>
        </div>
        <div class="config-body">
          <!-- 查看模式 -->
          <t-descriptions v-if="!editMode" :column="3">
            <t-descriptions-item label="部署方式">
              <t-tag size="small" variant="light" theme="default">{{ typeLabel(deploy.type) }}</t-tag>
            </t-descriptions-item>
            <t-descriptions-item label="同步状态">
              <t-tag :theme="syncTheme(deploy.sync_status)" variant="light" size="small">{{ syncLabel(deploy.sync_status) }}</t-tag>
            </t-descriptions-item>
            <t-descriptions-item label="最后运行">{{ deploy.last_run_at ? fmtTime(deploy.last_run_at) : '—' }}</t-descriptions-item>
            <t-descriptions-item label="工作目录">{{ deploy.work_dir || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type === 'docker-compose'" label="Compose 文件">{{ deploy.compose_file || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type !== 'native'" label="镜像名称">{{ deploy.image_name || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type !== 'docker-compose'" label="启动命令">{{ deploy.start_cmd || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type !== 'native'" label="期望版本">{{ deploy.desired_version || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type !== 'native'" label="实际版本">{{ deploy.actual_version || '—' }}</t-descriptions-item>
            <t-descriptions-item v-if="deploy.type !== 'native'" label="上一版本">{{ deploy.previous_version || '—' }}</t-descriptions-item>
          </t-descriptions>
          <!-- 编辑模式 -->
          <div v-else class="form-grid">
            <div class="form-field">
              <label class="form-label">工作目录</label>
              <t-input v-model="editForm.work_dir" />
            </div>
            <template v-if="editForm.type === 'docker-compose'">
              <div class="form-field">
                <label class="form-label">Compose 文件</label>
                <t-input v-model="editForm.compose_file" />
              </div>
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <t-input v-model="editForm.image_name" />
              </div>
            </template>
            <template v-if="editForm.type === 'docker'">
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <t-input v-model="editForm.image_name" />
              </div>
              <div class="form-field form-field--full">
                <label class="form-label">启动命令</label>
                <t-input v-model="editForm.start_cmd" />
              </div>
            </template>
            <template v-if="editForm.type === 'native'">
              <div class="form-field form-field--full">
                <label class="form-label">启动命令</label>
                <t-input v-model="editForm.start_cmd" />
              </div>
            </template>
            <div class="form-field">
              <label class="form-label">期望版本</label>
              <t-input v-model="editForm.desired_version" />
            </div>
          </div>
        </div>
      </div>

      <!-- S2: 操作台 -->
      <div class="section-block">
        <div class="section-title">
          <span>操作台</span>
          <t-space v-if="!running" size="small">
            <t-button theme="primary" size="small" @click="doRun('run')">立即部署</t-button>
            <t-button
              size="small"
              :disabled="!deploy.previous_version"
              :title="deploy.previous_version ? `回滚到 ${deploy.previous_version}` : '无历史版本'"
              @click="doRun('rollback')"
            >回滚{{ deploy.previous_version ? ` (→ ${deploy.previous_version})` : '' }}</t-button>
          </t-space>
          <t-button v-else size="small" theme="danger" variant="outline" @click="stopRun">中止</t-button>
        </div>

        <!-- 文件上传区 (native type) -->
        <div v-if="deploy.type === 'native'" class="upload-area">
          <div
            class="upload-zone"
            :class="{ 'upload-zone--active': !!uploadFile, 'upload-zone--drag': isDragging }"
            @dragover.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="onFileDrop"
            @click="fileInputRef?.click()"
          >
            <input ref="fileInputRef" type="file" class="file-input-hidden" @change="onFileChange" />
            <template v-if="!uploadFile">
              <div class="upload-zone-icon">📁</div>
              <div class="upload-zone-text">拖拽文件到此处，或点击选择文件</div>
              <div class="upload-zone-hint">支持任意可执行文件（jar、binary、zip 等）</div>
            </template>
            <template v-else>
              <div class="upload-zone-icon">📄</div>
              <div class="upload-zone-text">{{ uploadFile.name }}</div>
              <div class="upload-zone-hint">{{ fmtBytes(uploadFile.size) }}</div>
            </template>
          </div>

          <div v-if="uploading || uploadPhase === 'done'" class="upload-progress-area">
            <div class="upload-progress-header">
              <span>{{ uploadPhase === 'uploading' ? '正在上传文件到服务器...' : uploadPhase === 'transferring' ? '正在传输到远程主机...' : '传输完成 ✓' }}</span>
              <span v-if="uploadTotal > 0" class="upload-size-text">{{ fmtBytes(uploadProgress) }} / {{ fmtBytes(uploadTotal) }}</span>
            </div>
            <t-progress v-if="uploadTotal > 0" :percentage="Math.min(Math.round(uploadProgress / uploadTotal * 100), 100)" />
          </div>

          <div class="upload-actions">
            <t-button theme="primary" size="small" :loading="uploading" :disabled="!uploadFile" @click="doUpload">上传到服务器</t-button>
            <t-button v-if="uploadFile && !uploading" size="small" variant="outline" @click="clearUpload">清除选择</t-button>
          </div>

          <t-divider>运行控制</t-divider>
        </div>

        <!-- SSE 终端输出 -->
        <div class="terminal-wrap">
          <div v-if="runStatus" class="terminal-status-bar">
            <t-tag
              :theme="runStatus === 'success' ? 'success' : 'danger'"
              variant="light"
              size="small"
            >{{ runStatus === 'success' ? '部署成功' : '部署失败' }}</t-tag>
          </div>
          <div v-else-if="running" class="terminal-status-bar">
            <t-tag theme="warning" variant="light" size="small">执行中…</t-tag>
          </div>
          <pre v-if="outputLines.length > 0 || running" ref="termRef" class="deploy-terminal">{{ outputLines.join('\n') }}</pre>
          <div v-else class="terminal-placeholder">点击「立即部署」执行部署，输出将实时显示在此处</div>
        </div>
      </div>

      <!-- S3: 环境变量 -->
      <div class="section-block">
        <div class="section-title">
          <span>环境变量</span>
          <t-space size="small">
            <t-button size="small" variant="outline" :loading="envLoading" @click="loadEnv">刷新</t-button>
            <t-button size="small" variant="outline" @click="addEnvRow">添加变量</t-button>
            <t-button size="small" theme="primary" :loading="envSaving" @click="saveEnv">保存全部</t-button>
          </t-space>
        </div>
        <div class="env-body">
          <div v-if="!envLoading && envVars.length === 0" class="env-empty">暂无环境变量，点击「添加变量」新增</div>
          <div v-for="(v, i) in envVars" :key="i" class="env-row">
            <t-input v-model="v.key" placeholder="变量名（如 PORT）" class="env-key" size="small" />
            <div class="env-value-wrap">
              <t-input
                v-model="v.value"
                :type="v.secret && !v._revealed ? 'password' : 'text'"
                placeholder="变量值"
                class="env-value"
                size="small"
              />
            </div>
            <t-checkbox v-model="v.secret" size="small" @change="onSecretToggle(v)">Secret</t-checkbox>
            <t-button v-if="v.secret" size="small" variant="text" @click="v._revealed = !v._revealed">
              {{ v._revealed ? '隐藏' : '显示' }}
            </t-button>
            <t-button size="small" variant="text" theme="danger" @click="removeEnvRow(i)">删除</t-button>
          </div>
        </div>
      </div>

      <!-- S4: Webhook -->
      <div class="section-block">
        <div class="section-title">Webhook</div>
        <div class="webhook-body">
          <template v-if="webhookInfo">
            <div class="webhook-row">
              <span class="webhook-label">Webhook URL</span>
              <div class="webhook-value-wrap">
                <code class="webhook-url">{{ webhookInfo.url }}</code>
                <t-button size="small" variant="text" @click="copyText(webhookInfo.url, '链接已复制')">复制</t-button>
              </div>
            </div>
            <div class="webhook-row">
              <span class="webhook-label">Secret Token</span>
              <div class="webhook-value-wrap">
                <code class="webhook-url">{{ showSecret ? webhookInfo.secret : '••••••••••••••••••••••••' }}</code>
                <t-button size="small" variant="text" @click="showSecret = !showSecret">{{ showSecret ? '隐藏' : '显示' }}</t-button>
                <t-button size="small" variant="text" @click="copyText(webhookInfo.secret, 'Secret 已复制')">复制</t-button>
              </div>
            </div>
            <t-alert
              theme="info"
              message="支持 GitHub（X-Hub-Signature-256 HMAC 签名）和 GitLab（X-Gitlab-Token 原始 token 对比），推送时自动触发部署"
              class="webhook-alert"
            />
          </template>
          <div v-else class="env-empty">加载中...</div>
        </div>
      </div>

      <!-- S5: 部署历史 -->
      <div class="section-block">
        <div class="section-title">
          <span>部署历史</span>
          <t-button size="small" variant="outline" :loading="logsLoading" @click="loadLogs">刷新</t-button>
        </div>
        <div class="table-wrap">
          <t-table
            :data="logs"
            :columns="logColumns"
            :loading="logsLoading"
            row-key="id"
            stripe
            size="small"
          >
            <template #status="{ row }">
              <t-tag :theme="row.status === 'success' ? 'success' : 'danger'" variant="light" size="small">
                {{ row.status === 'success' ? '成功' : '失败' }}
              </t-tag>
            </template>
            <template #duration="{ row }">{{ row.duration }}s</template>
            <template #expandedRow="{ row }">
              <pre class="log-detail">{{ row.output }}</pre>
            </template>
          </t-table>
        </div>
      </div>

    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { getDeploy, createDeploy, updateDeploy, getDeployLogs, getDeployEnv, putDeployEnv, getWebhookInfo } from '@/api/deploy'
import { updateApp } from '@/api/application'
import type { Deploy, DeployLog, DeployForm } from '@/types/api'
import type { EnvVar } from '@/api/deploy'

// ── Local types ────────────────────────────────────────────────────────────

type LocalEnvVar = EnvVar & { _revealed: boolean }

// ── Route & store ─────────────────────────────────────────────────────────

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

// ── Core data ──────────────────────────────────────────────────────────────

const deploy = ref<Deploy | null>(null)
const loading = ref(false)

async function loadDeploy() {
  const deployId = app.value?.deploy_id
  if (!deployId) { deploy.value = null; return }
  loading.value = true
  try {
    deploy.value = await getDeploy(deployId)
    await Promise.all([loadLogs(), loadEnv(), loadWebhook()])
  } finally {
    loading.value = false
  }
}

watch(() => app.value?.deploy_id, (newId) => {
  if (newId) loadDeploy()
  else deploy.value = null
})

// ── Wizard ─────────────────────────────────────────────────────────────────

const wizardType = ref<'docker-compose' | 'docker' | 'native' | ''>('')
const wizardForm = reactive({
  work_dir: '',
  compose_file: 'docker-compose.yml',
  image_name: '',
  desired_version: '',
  start_cmd: '',
})
const creating = ref(false)

watch(wizardType, () => {
  wizardForm.compose_file = 'docker-compose.yml'
  wizardForm.image_name = ''
  wizardForm.desired_version = ''
  wizardForm.start_cmd = ''
  if (app.value && !wizardForm.work_dir) {
    wizardForm.work_dir = app.value.base_dir || `/srv/apps/${app.value.name}`
  }
})

async function createAndLink() {
  if (!app.value || !wizardType.value) return
  if (!wizardForm.work_dir) { MessagePlugin.warning('请填写工作目录'); return }
  if (wizardType.value === 'docker' && (!wizardForm.image_name || !wizardForm.start_cmd)) {
    MessagePlugin.warning('Docker 单容器需要填写镜像名称和启动命令'); return
  }
  if (wizardType.value === 'native' && !wizardForm.start_cmd) {
    MessagePlugin.warning('文件部署需要填写启动命令'); return
  }
  creating.value = true
  try {
    const payload: DeployForm = {
      name: `${app.value.name}-deploy`,
      server_id: app.value.server_id,
      type: wizardType.value as 'docker-compose' | 'docker' | 'native',
      work_dir: wizardForm.work_dir,
      compose_file: wizardForm.compose_file || 'docker-compose.yml',
      start_cmd: wizardForm.start_cmd,
      image_name: wizardForm.image_name,
      desired_version: wizardForm.desired_version,
    }
    const newDeploy = await createDeploy(payload)
    await updateApp(appId.value, {
      name: app.value.name,
      description: app.value.description,
      server_id: app.value.server_id,
      site_name: app.value.site_name,
      domain: app.value.domain,
      container_name: app.value.container_name,
      base_dir: app.value.base_dir,
      expose_mode: app.value.expose_mode,
      deploy_id: newDeploy.id,
      db_conn_id: app.value.db_conn_id,
    })
    await appStore.fetch()
    MessagePlugin.success('部署配置已创建并关联')
  } catch (e: any) {
    MessagePlugin.error(e?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

// ── Config edit ────────────────────────────────────────────────────────────

const editMode = ref(false)
const saving = ref(false)
const editForm = reactive<Partial<DeployForm>>({})

function startEdit() {
  if (!deploy.value) return
  Object.assign(editForm, {
    type: deploy.value.type,
    work_dir: deploy.value.work_dir,
    compose_file: deploy.value.compose_file,
    start_cmd: deploy.value.start_cmd,
    image_name: deploy.value.image_name,
    desired_version: deploy.value.desired_version,
  })
  editMode.value = true
}

function cancelEdit() { editMode.value = false }

async function saveEdit() {
  if (!deploy.value) return
  saving.value = true
  try {
    await updateDeploy(deploy.value.id, editForm)
    deploy.value = await getDeploy(deploy.value.id)
    editMode.value = false
    MessagePlugin.success('配置已保存')
  } catch (e: any) {
    MessagePlugin.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// ── Run / Rollback (SSE) ───────────────────────────────────────────────────

const running = ref(false)
const runStatus = ref('')
const outputLines = ref<string[]>([])
const termRef = ref<HTMLPreElement>()
let runAbort: AbortController | null = null

async function doRun(endpoint: 'run' | 'rollback') {
  if (!deploy.value) return
  running.value = true
  runStatus.value = ''
  outputLines.value = []
  runAbort = new AbortController()
  try {
    const res = await fetch(`/panel/api/v1/deploys/${deploy.value.id}/${endpoint}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.token}` },
      signal: runAbort.signal,
    })
    await consumeSSE(res, (evt) => {
      if (evt.type === 'output' || evt.type === 'error') {
        outputLines.value.push(evt.line)
        nextTick(() => { if (termRef.value) termRef.value.scrollTop = termRef.value.scrollHeight })
      } else if (evt.type === 'done') {
        runStatus.value = evt.line
        MessagePlugin[evt.line === 'success' ? 'success' : 'error'](evt.line === 'success' ? '部署成功' : '部署失败')
      }
    })
  } catch (e: any) {
    if (e.name !== 'AbortError') { outputLines.value.push('[连接错误] ' + String(e)); runStatus.value = 'failed' }
  } finally {
    running.value = false
    await loadLogs()
    if (deploy.value) deploy.value = await getDeploy(deploy.value.id)
  }
}

function stopRun() { runAbort?.abort(); running.value = false }

async function consumeSSE(res: Response, onEvent: (evt: { type: string; line: string; [k: string]: any }) => void) {
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
      try { onEvent(JSON.parse(line.slice(6))) } catch { /* ignore */ }
    }
  }
}

// ── File upload (native type) ──────────────────────────────────────────────

const fileInputRef = ref<HTMLInputElement>()
const uploadFile = ref<File | null>(null)
const uploading = ref(false)
const uploadPhase = ref<'' | 'uploading' | 'transferring' | 'done'>('')
const uploadProgress = ref(0)
const uploadTotal = ref(0)
const isDragging = ref(false)
let uploadAbort: AbortController | null = null

function onFileChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) { uploadFile.value = f; uploadPhase.value = '' }
}

function onFileDrop(e: DragEvent) {
  isDragging.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) { uploadFile.value = f; uploadPhase.value = '' }
}

function clearUpload() {
  uploadFile.value = null
  uploadPhase.value = ''
  uploadProgress.value = 0
  uploadTotal.value = 0
  if (fileInputRef.value) fileInputRef.value.value = ''
}

async function doUpload() {
  if (!deploy.value || !uploadFile.value) return
  uploading.value = true
  uploadPhase.value = 'uploading'
  uploadProgress.value = 0
  uploadTotal.value = 0
  uploadAbort = new AbortController()
  const formData = new FormData()
  formData.append('file', uploadFile.value)
  try {
    const res = await fetch(`/panel/api/v1/deploys/${deploy.value.id}/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.token}` },
      body: formData,
      signal: uploadAbort.signal,
    })
    await consumeSSE(res, (evt) => {
      if (evt.type === 'start') {
        uploadPhase.value = 'transferring'
        uploadTotal.value = evt.total ?? 0
      } else if (evt.type === 'progress') {
        uploadProgress.value = evt.bytes ?? 0
        uploadTotal.value = evt.total ?? uploadTotal.value
      } else if (evt.type === 'done') {
        uploadPhase.value = 'done'
        uploadProgress.value = uploadTotal.value
        MessagePlugin.success(`文件已传输至远程服务器：${evt.path}`)
      } else if (evt.type === 'error') {
        MessagePlugin.error('上传失败：' + evt.msg)
      }
    })
  } catch (e: any) {
    if (e.name !== 'AbortError') MessagePlugin.error('上传失败：' + String(e))
  } finally {
    uploading.value = false
  }
}

// ── Env vars ───────────────────────────────────────────────────────────────

const envVars = ref<LocalEnvVar[]>([])
const envLoading = ref(false)
const envSaving = ref(false)

async function loadEnv() {
  if (!deploy.value) return
  envLoading.value = true
  try {
    const vars = await getDeployEnv(deploy.value.id)
    envVars.value = vars.map(v => ({ ...v, _revealed: false }))
  } catch { /* ignore */ }
  finally { envLoading.value = false }
}

function addEnvRow() {
  envVars.value.push({ key: '', value: '', secret: false, _revealed: false })
}

function removeEnvRow(i: number) {
  envVars.value.splice(i, 1)
}

function onSecretToggle(v: LocalEnvVar) {
  if (v.secret) v._revealed = false
}

async function saveEnv() {
  if (!deploy.value) return
  envSaving.value = true
  try {
    const payload: EnvVar[] = envVars.value
      .filter(v => v.key.trim())
      .map(({ key, value, secret }) => ({ key: key.trim(), value, secret }))
    await putDeployEnv(deploy.value.id, payload)
    MessagePlugin.success('环境变量已保存')
    await loadEnv()
  } catch (e: any) {
    MessagePlugin.error(e?.message || '保存失败')
  } finally {
    envSaving.value = false
  }
}

// ── Webhook ────────────────────────────────────────────────────────────────

const webhookInfo = ref<{ url: string; secret: string } | null>(null)
const showSecret = ref(false)

async function loadWebhook() {
  if (!deploy.value) return
  try { webhookInfo.value = await getWebhookInfo(deploy.value.id) } catch { /* ignore */ }
}

function copyText(text: string, msg: string) {
  navigator.clipboard.writeText(text).then(() => MessagePlugin.success(msg))
}

// ── Deploy logs ────────────────────────────────────────────────────────────

const logs = ref<DeployLog[]>([])
const logsLoading = ref(false)

const logColumns = [
  { colKey: 'expand', type: 'expand', width: 52 },
  { colKey: 'created_at', title: '时间', width: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'duration', title: '耗时', width: 90 },
  { colKey: 'output', title: '输出摘要', minWidth: 200, ellipsis: true },
]

async function loadLogs() {
  if (!deploy.value) return
  logsLoading.value = true
  try { logs.value = await getDeployLogs(deploy.value.id, 20) }
  finally { logsLoading.value = false }
}

// ── Helpers ────────────────────────────────────────────────────────────────

const TYPE_LABELS: Record<string, string> = {
  'docker-compose': 'Docker Compose',
  'docker': 'Docker 单容器',
  'native': '文件部署',
}

function typeLabel(t: string) { return TYPE_LABELS[t] ?? t }

function syncTheme(s: string) {
  return ({ synced: 'success', drifted: 'warning', error: 'danger' } as Record<string, string>)[s] ?? 'default'
}

function syncLabel(s: string) {
  return ({ synced: '已同步', drifted: '有差异', syncing: '同步中', error: '错误', '': '空闲' } as Record<string, string>)[s] ?? s
}

function fmtTime(s: string) {
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`
  return `${(n / 1073741824).toFixed(2)} GB`
}

// ── Lifecycle ──────────────────────────────────────────────────────────────

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadDeploy()
})
</script>

<style scoped>
/* ── Wizard ── */
.wizard-card {
  background: var(--sh-card-bg);
  border: var(--sh-card-border);
  border-radius: var(--sh-card-radius);
  box-shadow: var(--sh-card-shadow);
  padding: 28px 32px 32px;
  max-width: 800px;
  margin: 0 auto;
}

.wizard-header { margin-bottom: 24px; }
.wizard-title { font-size: 16px; font-weight: 600; color: var(--sh-text-primary); margin-bottom: 4px; }
.wizard-subtitle { font-size: 13px; color: var(--sh-text-secondary); }

.type-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
  margin-bottom: 24px;
}

.type-card {
  border: 2px solid var(--sh-border);
  border-radius: 8px;
  padding: 20px 16px;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  text-align: center;
}
.type-card:hover { border-color: var(--sh-blue); box-shadow: 0 0 0 2px rgba(0,82,217,0.08); }
.type-card.active { border-color: var(--sh-blue); background: var(--sh-blue-bg); }
.type-card-icon { font-size: 28px; margin-bottom: 10px; }
.type-card-title { font-size: 14px; font-weight: 600; color: var(--sh-text-primary); margin-bottom: 6px; }
.type-card-desc { font-size: 12px; color: var(--sh-text-secondary); line-height: 1.5; }

.wizard-divider {
  font-size: 12px;
  font-weight: 600;
  color: var(--sh-text-secondary);
  letter-spacing: 0.5px;
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--sh-border);
}

.wizard-form-grid { max-width: 600px; }

.wizard-footer { margin-top: 24px; }

/* ── Form grid ── */
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.form-field { display: flex; flex-direction: column; gap: 6px; }
.form-field--full { grid-column: span 2; }
.form-label { font-size: 13px; color: var(--sh-text-primary); font-weight: 500; }
.form-required { color: var(--sh-red); margin-left: 2px; }
.form-hint { font-size: 12px; color: var(--sh-text-secondary); line-height: 1.4; }

/* ── Config body ── */
.config-body { padding: 16px 20px 20px; }

:deep(.t-descriptions__label) { color: var(--sh-text-secondary); font-size: 13px; min-width: 80px; }
:deep(.t-descriptions__content) { font-size: 13px; }

/* ── Upload area ── */
.upload-area { padding: 16px 20px 4px; }

.upload-zone {
  border: 2px dashed var(--sh-border);
  border-radius: 8px;
  padding: 24px 20px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  position: relative;
  margin-bottom: 12px;
}
.upload-zone:hover,
.upload-zone--drag { border-color: var(--sh-blue); background: var(--sh-blue-bg); }
.upload-zone--active { border-color: var(--sh-green); background: var(--sh-green-bg); border-style: solid; }
.file-input-hidden { position: absolute; opacity: 0; width: 0; height: 0; }
.upload-zone-icon { font-size: 28px; margin-bottom: 8px; }
.upload-zone-text { font-size: 14px; font-weight: 500; color: var(--sh-text-primary); }
.upload-zone-hint { font-size: 12px; color: var(--sh-text-secondary); margin-top: 4px; }

.upload-progress-area { margin-bottom: 12px; }
.upload-progress-header { display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--sh-text-secondary); margin-bottom: 8px; }
.upload-size-text { font-size: 12px; color: var(--sh-text-secondary); }

.upload-actions { display: flex; gap: 8px; margin-bottom: 16px; }

/* ── Terminal ── */
.terminal-wrap { padding: 0 20px 20px; }
.terminal-status-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.deploy-terminal {
  background: #1a2332;
  color: #e0e0e0;
  font-family: 'JetBrains Mono', Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.65;
  padding: 14px 16px;
  border-radius: 6px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 480px;
  min-height: 120px;
  margin: 0;
}
.terminal-placeholder { padding: 20px; text-align: center; font-size: 13px; color: var(--sh-text-secondary); }

/* ── Env vars ── */
.env-body { padding: 12px 20px 20px; display: flex; flex-direction: column; gap: 8px; }
.env-empty { font-size: 13px; color: var(--sh-text-secondary); padding: 8px 0; }
.env-row { display: flex; align-items: center; gap: 8px; }
.env-key { width: 180px; flex-shrink: 0; }
.env-value-wrap { flex: 1; }
.env-value { width: 100%; }

/* ── Webhook ── */
.webhook-body { padding: 14px 20px 20px; display: flex; flex-direction: column; gap: 12px; }
.webhook-row { display: flex; align-items: center; gap: 12px; }
.webhook-label { font-size: 13px; color: var(--sh-text-secondary); width: 100px; flex-shrink: 0; }
.webhook-value-wrap { display: flex; align-items: center; gap: 6px; flex: 1; min-width: 0; }
.webhook-url { font-family: 'JetBrains Mono', Menlo, monospace; font-size: 12px; background: var(--sh-gray-bg); padding: 3px 8px; border-radius: 4px; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.webhook-alert { margin-top: 4px; }

/* ── Log table ── */
.table-wrap { padding: 0 20px 16px; }
:deep(.t-table td) { font-size: 13px; }
.log-detail { background: #1a2332; color: #e0e0e0; font-size: 12px; line-height: 1.6; padding: 12px 16px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; max-height: 300px; overflow-y: auto; margin: 8px 16px; }

/* ── Empty state ── */
.empty-block { padding: 40px 20px; display: flex; justify-content: center; }
</style>
