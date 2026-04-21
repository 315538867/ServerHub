<template>
  <div class="dp-page">
    <div v-if="loading" class="dp-loading">
      <NSpin size="medium" />
    </div>

    <template v-else-if="!app?.deploy_id">
      <UiCard padding="lg" class="wizard-card">
        <div class="wizard-header">
          <div class="wizard-title">选择部署方式</div>
          <div class="wizard-subtitle">为该应用创建一个部署配置，配置完成后即可一键部署</div>
        </div>

        <div class="type-cards">
          <div
            v-for="t in wizardOptions" :key="t.value"
            class="type-card" :class="{ 'is-active': wizardType === t.value }"
            @click="wizardType = t.value"
          >
            <div class="type-card__icon">{{ t.icon }}</div>
            <div class="type-card__title">{{ t.label }}</div>
            <div class="type-card__desc">{{ t.desc }}</div>
          </div>
        </div>

        <template v-if="wizardType">
          <div class="wizard-divider">基础配置</div>
          <div class="form-grid">
            <div class="form-field">
              <label class="form-label">工作目录 <span class="form-required">*</span></label>
              <NInput v-model:value="wizardForm.work_dir" :placeholder="app ? `/srv/apps/${app.name}` : '/srv/apps/myapp'" />
              <span class="form-hint">在远程服务器上的项目根目录</span>
            </div>
            <template v-if="wizardType === 'docker-compose'">
              <div class="form-field">
                <label class="form-label">Compose 文件</label>
                <NInput v-model:value="wizardForm.compose_file" placeholder="docker-compose.yml" />
              </div>
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <NInput v-model:value="wizardForm.image_name" placeholder="nginx（版本追踪用，可选）" />
              </div>
              <div class="form-field">
                <label class="form-label">期望版本</label>
                <NInput v-model:value="wizardForm.desired_version" placeholder="latest（可选）" />
              </div>
            </template>
            <template v-if="wizardType === 'docker'">
              <div class="form-field">
                <label class="form-label">镜像名称 <span class="form-required">*</span></label>
                <NInput v-model:value="wizardForm.image_name" placeholder="nginx" />
              </div>
              <div class="form-field">
                <label class="form-label">期望版本 <span class="form-required">*</span></label>
                <NInput v-model:value="wizardForm.desired_version" placeholder="latest" />
              </div>
            </template>
          </div>

          <template v-if="wizardType !== 'docker-compose' && wizardType !== 'static'">
            <div class="wizard-divider">运行环境 <span class="divider-hint">选择后自动生成 startup.sh 模板</span></div>
            <div class="runtime-chips">
              <div
                v-for="rt in RUNTIMES" :key="rt.value"
                class="runtime-chip" :class="{ 'is-active': wizardRuntime === rt.value }"
                @click="selectWizardRuntime(rt.value)"
              >
                <span class="rt-icon">{{ rt.icon }}</span>
                <span class="rt-label">{{ rt.label }}</span>
              </div>
            </div>
          </template>
          <div class="wizard-footer">
            <UiButton variant="primary" :loading="creating" @click="createAndLink">创建并关联部署配置</UiButton>
          </div>
        </template>
      </UiCard>
    </template>

    <template v-else-if="deploy">
      <Teleport to="#app-bar-actions">
        <div class="deploy-bar" :class="deployBarClass">
          <div class="deploy-bar__status">
            <span class="deploy-bar__dot" :class="dotClass" />
            <span class="deploy-bar__label">{{ syncStatusLabel }}</span>
            <template v-if="deploy.actual_version || deploy.desired_version">
              <code class="deploy-bar__ver">{{ deploy.actual_version || '—' }}</code>
              <template v-if="deploy.desired_version && deploy.desired_version !== deploy.actual_version">
                <span class="deploy-bar__arrow">→</span>
                <code class="deploy-bar__ver deploy-bar__ver--target">{{ deploy.desired_version }}</code>
              </template>
            </template>
          </div>
          <div class="deploy-bar__actions">
            <template v-if="!running">
              <UiButton variant="primary" size="sm" @click="doRun('run')">
                <template #icon><Play :size="12" /></template>
                立即部署
              </UiButton>
              <UiButton
                variant="secondary"
                size="sm"
                :disabled="!canRollback"
                @click="doRun('rollback')"
              >
                <template #icon><RotateCcw :size="12" /></template>
                回滚
              </UiButton>
            </template>
            <template v-else>
              <span class="deploy-bar__running">
                <Loader2 :size="12" class="spin" />
                部署中…
              </span>
              <UiButton variant="danger" size="sm" @click="stopRun">中止</UiButton>
            </template>
          </div>
        </div>
      </Teleport>

      <UiSection title="配置信息">
        <template #extra>
          <template v-if="!editMode">
            <UiButton variant="secondary" size="sm" @click="startEdit">编辑</UiButton>
          </template>
          <template v-else>
            <UiButton variant="secondary" size="sm" @click="cancelEdit">取消</UiButton>
            <UiButton variant="primary" size="sm" :loading="saving" @click="saveEdit">保存</UiButton>
          </template>
        </template>
        <UiCard padding="md">
          <div v-if="!editMode" class="cfg-grid">
            <div class="cfg-cell"><span class="lbl">部署方式</span><UiBadge tone="neutral">{{ typeLabel(deploy.type) }}</UiBadge></div>
            <div class="cfg-cell"><span class="lbl">同步状态</span><UiBadge :tone="syncTone(deploy.sync_status)">{{ syncLabel(deploy.sync_status) }}</UiBadge></div>
            <div class="cfg-cell"><span class="lbl">最后运行</span><span class="val">{{ deploy.last_run_at ? fmtTime(deploy.last_run_at) : '—' }}</span></div>
            <div class="cfg-cell"><span class="lbl">工作目录</span><code class="mono">{{ deploy.work_dir || '—' }}</code></div>
            <div v-if="deploy.type === 'docker-compose'" class="cfg-cell"><span class="lbl">Compose 文件</span><span class="val">{{ deploy.compose_file || '—' }}</span></div>
            <div v-if="deploy.type === 'docker' || deploy.type === 'docker-compose'" class="cfg-cell"><span class="lbl">镜像名称</span><span class="val">{{ deploy.image_name || '—' }}</span></div>
            <div v-if="(deploy.type === 'native' || deploy.type === 'docker') && deploy.start_cmd" class="cfg-cell"><span class="lbl">启动命令</span><code class="mono">{{ deploy.start_cmd }}</code></div>
            <div v-if="deploy.runtime" class="cfg-cell"><span class="lbl">运行时</span><UiBadge tone="neutral">{{ RUNTIME_LABELS[deploy.runtime] ?? deploy.runtime }}</UiBadge></div>
            <div v-if="deploy.type !== 'native'" class="cfg-cell"><span class="lbl">期望版本</span><span class="val">{{ deploy.desired_version || '—' }}</span></div>
            <div v-if="deploy.type !== 'native'" class="cfg-cell"><span class="lbl">实际版本</span><span class="val">{{ deploy.actual_version || '—' }}</span></div>
            <div v-if="deploy.type !== 'native'" class="cfg-cell"><span class="lbl">上一版本</span><span class="val">{{ prevVersionLabel || '—' }}</span></div>
          </div>
          <div v-else class="form-grid">
            <div class="form-field">
              <label class="form-label">工作目录</label>
              <NInput v-model:value="editForm.work_dir" />
            </div>
            <template v-if="editForm.type === 'docker-compose'">
              <div class="form-field">
                <label class="form-label">Compose 文件</label>
                <NInput v-model:value="editForm.compose_file" />
              </div>
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <NInput v-model:value="editForm.image_name" />
              </div>
            </template>
            <template v-if="editForm.type === 'docker'">
              <div class="form-field">
                <label class="form-label">镜像名称</label>
                <NInput v-model:value="editForm.image_name" />
              </div>
            </template>
            <template v-if="editForm.type === 'native' || editForm.type === 'docker'">
              <div class="form-field form-field--full">
                <label class="form-label">启动命令 <span class="form-hint">（可选，有 startup.sh 时此项被忽略）</span></label>
                <NInput v-model:value="editForm.start_cmd" placeholder="作为 startup.sh 的备用方案" />
              </div>
              <div class="form-field form-field--full">
                <label class="form-label">运行时</label>
                <div class="runtime-chips runtime-chips--sm">
                  <div
                    v-for="rt in RUNTIMES" :key="rt.value"
                    class="runtime-chip" :class="{ 'is-active': editForm.runtime === rt.value }"
                    @click="selectEditRuntime(rt.value)"
                  >
                    <span class="rt-icon">{{ rt.icon }}</span>
                    <span class="rt-label">{{ rt.label }}</span>
                  </div>
                </div>
              </div>
            </template>
            <div class="form-field">
              <label class="form-label">期望版本</label>
              <NInput v-model:value="editForm.desired_version" />
            </div>
          </div>
        </UiCard>
      </UiSection>

      <UiSection title="配置文件">
        <template #extra>
          <UiButton variant="secondary" size="sm" :loading="cfSaving" @click="saveCfList">保存</UiButton>
          <UiButton variant="primary" size="sm" @click="openCfEditor(null)">添加文件</UiButton>
        </template>
        <UiCard padding="md">
          <div v-if="cfList.length === 0" class="cf-empty">
            暂无配置文件。添加 <code>startup.sh</code> 将在部署时自动执行；亦可添加 <code>application.yml</code>、<code>config.toml</code> 等配套文件。
          </div>
          <div v-for="(f, i) in cfList" :key="i" class="cf-row">
            <span class="cf-name">{{ f.name }}</span>
            <span class="cf-ext-badge">{{ getExtBadge(f.name) }}</span>
            <div class="cf-actions">
              <UiButton variant="ghost" size="sm" @click="openCfEditor(i)">编辑</UiButton>
              <UiButton variant="ghost" size="sm" @click="deleteCfItem(i)">
                <span class="text-danger">删除</span>
              </UiButton>
            </div>
          </div>
        </UiCard>
      </UiSection>

      <UiSection title="操作台">
        <template #extra>
          <template v-if="!running">
            <UiButton variant="primary" size="sm" @click="doRun('run')">立即部署</UiButton>
            <UiButton
              variant="secondary" size="sm"
              :disabled="!canRollback"
              @click="doRun('rollback')"
            >回滚{{ prevVersionLabel ? ` (→ ${prevVersionLabel})` : '' }}</UiButton>
          </template>
          <UiButton v-else variant="danger" size="sm" @click="stopRun">中止</UiButton>
        </template>

        <UiCard v-if="deploy.type === 'native' || deploy.type === 'static'" padding="md" class="upload-card">
          <div
            class="upload-zone"
            :class="{ 'is-active': !!uploadFile, 'is-drag': isDragging }"
            @dragover.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="onFileDrop"
            @click="fileInputRef?.click()"
          >
            <input ref="fileInputRef" type="file" class="file-input-hidden" @change="onFileChange" />
            <template v-if="!uploadFile">
              <div class="upload-zone__icon">📁</div>
              <div class="upload-zone__text">拖拽文件到此处，或点击选择文件</div>
              <div class="upload-zone__hint">{{ deploy.type === 'static' ? '上传 dist.zip / dist.tar.gz，将归档到 releases/<ts>/ 并切换 current 软链' : '支持任意可执行文件（jar、binary、zip 等）' }}</div>
            </template>
            <template v-else>
              <div class="upload-zone__icon">📄</div>
              <div class="upload-zone__text">{{ uploadFile.name }}</div>
              <div class="upload-zone__hint">{{ fmtBytes(uploadFile.size) }}</div>
            </template>
          </div>

          <div v-if="uploading || uploadPhase === 'done'" class="upload-progress">
            <div class="upload-progress__head">
              <span>{{ uploadPhase === 'uploading' ? '正在上传文件到服务器...' : uploadPhase === 'transferring' ? '正在传输到远程主机...' : '传输完成 ✓' }}</span>
              <span v-if="uploadTotal > 0" class="upload-size">{{ fmtBytes(uploadProgress) }} / {{ fmtBytes(uploadTotal) }}</span>
            </div>
            <NProgress
              v-if="uploadTotal > 0"
              :percentage="Math.min(Math.round(uploadProgress / uploadTotal * 100), 100)"
              :show-indicator="false"
            />
          </div>

          <div class="upload-actions">
            <UiButton variant="primary" size="sm" :loading="uploading" :disabled="!uploadFile" @click="doUpload">上传到服务器</UiButton>
            <UiButton v-if="uploadFile && !uploading" variant="secondary" size="sm" @click="clearUpload">清除选择</UiButton>
          </div>
        </UiCard>

        <UiCard padding="md" class="terminal-card">
          <div v-if="runStatus" class="term-status">
            <UiBadge :tone="runStatus === 'success' ? 'success' : 'danger'">
              {{ runStatus === 'success' ? '部署成功' : '部署失败' }}
            </UiBadge>
          </div>
          <div v-else-if="running" class="term-status">
            <UiBadge tone="warning">执行中…</UiBadge>
          </div>
          <pre v-if="outputLines.length > 0 || running" ref="termRef" class="deploy-terminal">{{ outputLines.join('\n') }}</pre>
          <div v-else class="terminal-placeholder">点击「立即部署」执行部署，输出将实时显示在此处</div>
        </UiCard>
      </UiSection>

      <UiSection title="环境变量">
        <template #extra>
          <UiButton variant="secondary" size="sm" :loading="envLoading" @click="loadEnv">刷新</UiButton>
          <UiButton variant="secondary" size="sm" @click="addEnvRow">添加变量</UiButton>
          <UiButton variant="primary" size="sm" :loading="envSaving" @click="saveEnv">保存全部</UiButton>
        </template>
        <UiCard padding="md">
          <div v-if="!envLoading && envVars.length === 0" class="env-empty">暂无环境变量，点击「添加变量」新增</div>
          <div v-for="(v, i) in envVars" :key="i" class="env-row">
            <NInput v-model:value="v.key" placeholder="变量名（如 PORT）" class="env-key" size="small" />
            <NInput
              v-model:value="v.value"
              :type="v.secret && !v._revealed ? 'password' : 'text'"
              placeholder="变量值"
              class="env-value"
              size="small"
            />
            <NCheckbox v-model:checked="v.secret" @update:checked="onSecretToggle(v)">Secret</NCheckbox>
            <UiButton v-if="v.secret" variant="ghost" size="sm" @click="v._revealed = !v._revealed">
              {{ v._revealed ? '隐藏' : '显示' }}
            </UiButton>
            <UiButton variant="ghost" size="sm" @click="removeEnvRow(i)">
              <span class="text-danger">删除</span>
            </UiButton>
          </div>
        </UiCard>
      </UiSection>

      <UiSection title="Webhook">
        <UiCard padding="md">
          <template v-if="webhookInfo">
            <div class="webhook-row">
              <span class="webhook-label">Webhook URL</span>
              <code class="webhook-url">{{ webhookInfo.url }}</code>
              <UiButton variant="ghost" size="sm" @click="copyText(webhookInfo.url, '链接已复制')">复制</UiButton>
            </div>
            <div class="webhook-row">
              <span class="webhook-label">Secret Token</span>
              <code class="webhook-url">{{ showSecret ? webhookInfo.secret : '••••••••••••••••••••••••' }}</code>
              <UiButton variant="ghost" size="sm" @click="showSecret = !showSecret">{{ showSecret ? '隐藏' : '显示' }}</UiButton>
              <UiButton variant="ghost" size="sm" @click="copyText(webhookInfo.secret, 'Secret 已复制')">复制</UiButton>
            </div>
            <NAlert
              type="info"
              title="支持 GitHub（X-Hub-Signature-256 HMAC 签名）和 GitLab（X-Gitlab-Token 原始 token 对比），推送时自动触发部署"
              class="webhook-alert"
            />
          </template>
          <div v-else class="env-empty">加载中...</div>
        </UiCard>
      </UiSection>

      <UiSection title="部署历史">
        <template #extra>
          <UiButton variant="secondary" size="sm" :loading="logsLoading" @click="loadLogs">
            <template #icon><RefreshCw :size="14" /></template>
            刷新
          </UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="logColumns"
            :data="logs"
            :loading="logsLoading"
            :row-key="(row: DeployLog) => row.id"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>

      <NModal
        v-model:show="editorVisible"
        preset="card"
        :title="editorIsNew ? '添加配置文件' : `编辑：${editorFileName}`"
        style="width: 760px"
        :bordered="false"
        @after-leave="destroyCfEditor"
      >
        <div class="cf-editor-toolbar">
          <NInput
            v-if="editorIsNew"
            v-model:value="editorFileName"
            placeholder="文件名（如 startup.sh）"
            size="small"
            style="width: 220px"
          />
          <span v-else class="cf-editor-filename">{{ editorFileName }}</span>
          <input ref="cfFileInputRef" type="file" style="display:none" @change="onCfFileUpload" />
          <UiButton variant="secondary" size="sm" @click="cfFileInputRef?.click()">上传本地文件</UiButton>
        </div>
        <div ref="cfEditorEl" class="code-editor" />
        <template #footer>
          <div class="modal-foot">
            <UiButton variant="secondary" size="sm" @click="editorVisible = false">取消</UiButton>
            <UiButton variant="primary" size="sm" :loading="editorSaving" @click="saveCfEditor">保存</UiButton>
          </div>
        </template>
      </NModal>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import { NInput, NCheckbox, NAlert, NModal, NDataTable, NProgress, NSpin, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Play, RotateCcw, Loader2 } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { getDeploy, createDeploy, updateDeploy, getDeployLogs, getDeployEnv, putDeployEnv, getWebhookInfo, getDeployVersions } from '@/api/deploy'
import { updateApp } from '@/api/application'
import type { Deploy, DeployLog, DeployForm, ConfigFile, DeployVersion } from '@/types/api'
import type { EnvVar } from '@/api/deploy'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { yaml } from '@codemirror/lang-yaml'
import { json } from '@codemirror/lang-json'
import { javascript } from '@codemirror/lang-javascript'
import { xml } from '@codemirror/lang-xml'
import { html } from '@codemirror/lang-html'
import { StreamLanguage } from '@codemirror/language'
import { toml } from '@codemirror/legacy-modes/mode/toml'
import { properties } from '@codemirror/legacy-modes/mode/properties'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

type LocalEnvVar = EnvVar & { _revealed: boolean }

const RUNTIMES = [
  { value: 'java',   icon: '☕', label: 'Java'   },
  { value: 'go',     icon: '🐹', label: 'Go'     },
  { value: 'node',   icon: '🟢', label: 'Node'   },
  { value: 'rust',   icon: '🦀', label: 'Rust'   },
  { value: 'python', icon: '🐍', label: 'Python' },
  { value: 'custom', icon: '⚙️', label: '自定义'  },
]

const RUNTIME_LABELS: Record<string, string> = Object.fromEntries(RUNTIMES.map(r => [r.value, r.label]))

const RUNTIME_TEMPLATES: Record<string, ConfigFile[]> = {
  java: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\nJAVA_OPTS="${JAVA_OPTS:--Xmx512m -Xms256m}"\nexec java $JAVA_OPTS -jar app.jar "$@"\n' },
    { name: 'application.yml', content: 'server:\n  port: 8080\n\nspring:\n  application:\n    name: myapp\n  profiles:\n    active: prod\n' },
  ],
  go: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\nexec ./server "$@"\n' },
    { name: 'config.yaml', content: 'server:\n  host: 0.0.0.0\n  port: 8080\n\nlog:\n  level: info\n' },
  ],
  node: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\nexec node dist/main.js "$@"\n' },
  ],
  rust: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\nexec ./app "$@"\n' },
    { name: 'config.toml', content: '[server]\nhost = "0.0.0.0"\nport = 8080\n\n[log]\nlevel = "info"\n' },
  ],
  python: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\nexec python main.py "$@"\n' },
    { name: 'requirements.txt', content: '# Add your dependencies here\n' },
  ],
  custom: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\n# Write your startup command here\n' },
  ],
  docker: [
    { name: 'startup.sh', content: '#!/bin/bash\nset -e\ndocker stop myapp 2>/dev/null || true\ndocker rm myapp 2>/dev/null || true\ndocker run -d \\\n  --name myapp \\\n  --restart unless-stopped \\\n  -p 8080:8080 \\\n  IMAGE_NAME:latest\n' },
  ],
}

function getTemplateFiles(runtime: string, deployType?: string): ConfigFile[] {
  if (deployType === 'docker') return RUNTIME_TEMPLATES['docker'] ?? []
  return RUNTIME_TEMPLATES[runtime] ?? []
}

function getExtLang(filename: string) {
  const ext = filename.split('.').pop()?.toLowerCase() ?? ''
  if (['yml', 'yaml'].includes(ext)) return yaml()
  if (ext === 'json') return json()
  if (['js', 'ts', 'sh', 'bash'].includes(ext)) return javascript()
  if (ext === 'xml') return xml()
  if (['html', 'htm'].includes(ext)) return html()
  if (ext === 'toml') return StreamLanguage.define(toml)
  if (['properties', 'ini', 'env'].includes(ext)) return StreamLanguage.define(properties)
  return []
}

function getExtBadge(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() ?? ''
  return ext || 'txt'
}

function parseCfJson(raw: string): ConfigFile[] {
  if (!raw) return []
  try { return JSON.parse(raw) } catch { return [] }
}

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const message = useMessage()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const deploy = ref<Deploy | null>(null)
const loading = ref(false)

const cfList = ref<ConfigFile[]>([])
const cfSaving = ref(false)

const wizardOptions = [
  { value: 'docker-compose', icon: '⚙️', label: 'Docker Compose', desc: '使用 docker-compose.yml 编排多个容器，支持 pull + up -d' },
  { value: 'docker', icon: '🐳', label: 'Docker 单容器', desc: '拉取指定镜像，执行 docker run 命令启动单个容器' },
  { value: 'native', icon: '📦', label: '文件部署', desc: '上传可执行文件（jar / binary / zip）到服务器并运行' },
  { value: 'static', icon: '🌐', label: '静态站点', desc: '上传 dist.zip / tar.gz，归档到 releases/<ts>/ 并切换 current 软链' },
] as const

async function saveCfList() {
  if (!deploy.value) return
  cfSaving.value = true
  try {
    await updateDeploy(deploy.value.id, { ...deploy.value, config_files: JSON.stringify(cfList.value) } as any)
    deploy.value = await getDeploy(deploy.value.id)
    message.success('配置文件已保存')
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    cfSaving.value = false
  }
}

function deleteCfItem(i: number) { cfList.value.splice(i, 1) }

const editorVisible = ref(false)
const editorIsNew = ref(false)
const editorFileName = ref('')
const editorEditIdx = ref(-1)
const editorSaving = ref(false)
const cfEditorEl = ref<HTMLDivElement>()
const cfFileInputRef = ref<HTMLInputElement>()
let cfEditorView: EditorView | null = null

function openCfEditor(idx: number | null) {
  if (idx === null) {
    editorIsNew.value = true; editorFileName.value = ''; editorEditIdx.value = -1
  } else {
    editorIsNew.value = false; editorFileName.value = cfList.value[idx].name; editorEditIdx.value = idx
  }
  editorVisible.value = true
  nextTick(() => initCfEditor(idx !== null ? cfList.value[idx].content : ''))
}

function initCfEditor(content: string) {
  cfEditorView?.destroy()
  if (!cfEditorEl.value) return
  const lang = getExtLang(editorFileName.value)
  const extensions = [basicSetup, oneDark, ...(Array.isArray(lang) ? lang : [lang])]
  cfEditorView = new EditorView({
    state: EditorState.create({ doc: content, extensions }),
    parent: cfEditorEl.value,
  })
}

function destroyCfEditor() { cfEditorView?.destroy(); cfEditorView = null }

async function saveCfEditor() {
  const fname = editorFileName.value.trim()
  if (!fname) { message.warning('请输入文件名'); return }
  const content = cfEditorView?.state.doc.toString() ?? ''
  editorSaving.value = true
  try {
    if (editorIsNew.value) cfList.value.push({ name: fname, content })
    else cfList.value[editorEditIdx.value] = { name: fname, content }
    await saveCfList()
    editorVisible.value = false
  } finally {
    editorSaving.value = false
  }
}

function onCfFileUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (ev) => {
    const text = ev.target?.result as string
    cfEditorView?.destroy()
    if (cfEditorEl.value) initCfEditor(text)
    if (editorIsNew.value && !editorFileName.value) editorFileName.value = file.name
  }
  reader.readAsText(file)
  ;(e.target as HTMLInputElement).value = ''
}

const wizardRuntime = ref('')
function selectWizardRuntime(rt: string) { wizardRuntime.value = rt }

async function loadDeploy() {
  const deployId = app.value?.deploy_id
  if (!deployId) { deploy.value = null; return }
  loading.value = true
  try {
    deploy.value = await getDeploy(deployId)
    cfList.value = parseCfJson(deploy.value.config_files)
    await Promise.all([loadLogs(), loadEnv(), loadWebhook(), loadVersions()])
  } finally {
    loading.value = false
  }
}

watch(() => app.value?.deploy_id, (newId) => {
  if (newId) loadDeploy()
  else deploy.value = null
})

const wizardType = ref<'docker-compose' | 'docker' | 'native' | 'static' | ''>('')
const wizardForm = reactive({
  work_dir: '', compose_file: 'docker-compose.yml', image_name: '', desired_version: '',
})
const creating = ref(false)

watch(wizardType, () => {
  wizardForm.compose_file = 'docker-compose.yml'
  wizardForm.image_name = ''
  wizardForm.desired_version = ''
  wizardRuntime.value = ''
  if (app.value && !wizardForm.work_dir) {
    wizardForm.work_dir = app.value.base_dir || `/srv/apps/${app.value.name}`
  }
})

async function createAndLink() {
  if (!app.value || !wizardType.value) return
  if (!wizardForm.work_dir) { message.warning('请填写工作目录'); return }
  if (wizardType.value === 'docker' && !wizardForm.image_name) {
    message.warning('Docker 单容器需要填写镜像名称'); return
  }
  creating.value = true
  try {
    const initFiles = wizardRuntime.value
      ? getTemplateFiles(wizardRuntime.value, wizardType.value)
      : (wizardType.value === 'docker' ? getTemplateFiles('docker', 'docker') : [])
    const payload: DeployForm = {
      name: `${app.value.name}-deploy`,
      server_id: app.value.server_id,
      type: wizardType.value as 'docker-compose' | 'docker' | 'native' | 'static',
      work_dir: wizardForm.work_dir,
      compose_file: wizardForm.compose_file || 'docker-compose.yml',
      image_name: wizardForm.image_name,
      desired_version: wizardForm.desired_version,
      runtime: wizardRuntime.value,
      config_files: JSON.stringify(initFiles),
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
    message.success('部署配置已创建并关联')
  } catch (e: any) {
    message.error(e?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

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
    runtime: deploy.value.runtime ?? '',
  })
  editMode.value = true
}

function cancelEdit() { editMode.value = false }

function selectEditRuntime(rt: string) {
  if (editForm.runtime === rt) { editForm.runtime = ''; return }
  editForm.runtime = rt
  if (!cfList.value.length || confirm(`用「${RUNTIME_LABELS[rt]}」的模板初始化配置文件？（将覆盖当前列表）`)) {
    cfList.value = getTemplateFiles(rt, editForm.type)
  }
}

async function saveEdit() {
  if (!deploy.value) return
  saving.value = true
  try {
    await updateDeploy(deploy.value.id, { ...editForm, config_files: JSON.stringify(cfList.value) } as any)
    deploy.value = await getDeploy(deploy.value.id)
    cfList.value = parseCfJson(deploy.value.config_files)
    editMode.value = false
    message.success('配置已保存')
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const running = ref(false)
const runStatus = ref('')
const outputLines = ref<string[]>([])
const termRef = ref<HTMLPreElement>()
let runAbort: AbortController | null = null

async function doRun(endpoint: 'run' | 'rollback') {
  if (!deploy.value) return
  running.value = true; runStatus.value = ''; outputLines.value = []
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
        message[evt.line === 'success' ? 'success' : 'error'](evt.line === 'success' ? '部署成功' : '部署失败')
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
  uploadFile.value = null; uploadPhase.value = ''
  uploadProgress.value = 0; uploadTotal.value = 0
  if (fileInputRef.value) fileInputRef.value.value = ''
}

async function doUpload() {
  if (!deploy.value || !uploadFile.value) return
  uploading.value = true; uploadPhase.value = 'uploading'
  uploadProgress.value = 0; uploadTotal.value = 0
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
      if (evt.type === 'start') { uploadPhase.value = 'transferring'; uploadTotal.value = evt.total ?? 0 }
      else if (evt.type === 'progress') { uploadProgress.value = evt.bytes ?? 0; uploadTotal.value = evt.total ?? uploadTotal.value }
      else if (evt.type === 'done') { uploadPhase.value = 'done'; uploadProgress.value = uploadTotal.value; message.success(`文件已传输至远程服务器：${evt.path}`) }
      else if (evt.type === 'error') { message.error('上传失败：' + evt.msg) }
    })
  } catch (e: any) {
    if (e.name !== 'AbortError') message.error('上传失败：' + String(e))
  } finally {
    uploading.value = false
  }
}

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

function addEnvRow() { envVars.value.push({ key: '', value: '', secret: false, _revealed: false }) }
function removeEnvRow(i: number) { envVars.value.splice(i, 1) }
function onSecretToggle(v: LocalEnvVar) { if (v.secret) v._revealed = false }

async function saveEnv() {
  if (!deploy.value) return
  envSaving.value = true
  try {
    const payload: EnvVar[] = envVars.value
      .filter(v => v.key.trim())
      .map(({ key, value, secret }) => ({ key: key.trim(), value, secret }))
    await putDeployEnv(deploy.value.id, payload)
    message.success('环境变量已保存')
    await loadEnv()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    envSaving.value = false
  }
}

const webhookInfo = ref<{ url: string; secret: string } | null>(null)
const showSecret = ref(false)

async function loadWebhook() {
  if (!deploy.value) return
  try { webhookInfo.value = await getWebhookInfo(deploy.value.id) } catch { /* ignore */ }
}

function copyText(text: string, msg: string) {
  navigator.clipboard.writeText(text).then(() => message.success(msg))
}

const logs = ref<DeployLog[]>([])
const logsLoading = ref(false)
const versions = ref<DeployVersion[]>([])

const canRollback = computed(() => {
  if (!deploy.value) return false
  const cur = deploy.value.actual_version
  return versions.value.some(v => v.version && v.version !== cur)
})
const prevVersionLabel = computed(() => {
  if (!deploy.value) return ''
  const cur = deploy.value.actual_version
  return versions.value.find(v => v.version && v.version !== cur)?.version ?? ''
})
async function loadVersions() {
  if (!deploy.value) return
  try { versions.value = await getDeployVersions(deploy.value.id) } catch { versions.value = [] }
}

const logColumns = computed<DataTableColumns<DeployLog>>(() => [
  { type: 'expand', renderExpand: (row) => h('pre', { class: 'log-detail' }, row.output) },
  { title: '时间', key: 'created_at', width: 180 },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge, { tone: row.status === 'success' ? 'success' : 'danger' },
      () => row.status === 'success' ? '成功' : '失败'),
  },
  { title: '耗时', key: 'duration', width: 90, render: (row) => `${row.duration}s` },
  { title: '输出摘要', key: 'output', minWidth: 200, ellipsis: { tooltip: true } },
])

async function loadLogs() {
  if (!deploy.value) return
  logsLoading.value = true
  try { logs.value = await getDeployLogs(deploy.value.id, 20) }
  finally { logsLoading.value = false }
}

const TYPE_LABELS: Record<string, string> = {
  'docker-compose': 'Docker Compose',
  'docker': 'Docker 单容器',
  'native': '文件部署',
}

function typeLabel(t: string) { return TYPE_LABELS[t] ?? t }

function syncTone(s: string): 'success' | 'warning' | 'danger' | 'neutral' {
  return ({ synced: 'success', drifted: 'warning', error: 'danger' } as const)[s as 'synced' | 'drifted' | 'error'] ?? 'neutral'
}

function syncLabel(s: string) {
  return ({ synced: '已同步', drifted: '有差异', syncing: '同步中', error: '错误', '': '空闲' } as Record<string, string>)[s] ?? s
}

const syncStatusLabel = computed(() => syncLabel(deploy.value?.sync_status ?? ''))
const dotClass = computed(() => {
  switch (deploy.value?.sync_status) {
    case 'synced':  return 'dot--ok'
    case 'drifted': return 'dot--warn'
    case 'syncing': return 'dot--info'
    case 'error':   return 'dot--err'
    default:        return 'dot--idle'
  }
})
const deployBarClass = computed(() => ({
  'deploy-bar--running': running.value,
  'deploy-bar--drift':   deploy.value?.sync_status === 'drifted' || deploy.value?.sync_status === 'error',
}))

function fmtTime(s: string) {
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`
  return `${(n / 1073741824).toFixed(2)} GB`
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await loadDeploy()
})

onBeforeUnmount(() => { cfEditorView?.destroy() })
</script>

<style scoped>
.dp-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.dp-loading { display: flex; justify-content: center; padding: var(--space-12); }

.wizard-card { max-width: 820px; margin: 0 auto; }
.wizard-header { margin-bottom: var(--space-5); }
.wizard-title { font-size: var(--fs-md); font-weight: var(--fw-semibold); color: var(--ui-fg); margin-bottom: var(--space-1); }
.wizard-subtitle { font-size: var(--fs-sm); color: var(--ui-fg-3); }

.type-cards {
  display: grid; grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3); margin-bottom: var(--space-5);
}
.type-card {
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-5) var(--space-3);
  cursor: pointer; text-align: center;
  background: var(--ui-bg-1);
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease), transform var(--dur-fast) var(--ease);
}
.type-card:hover { border-color: var(--ui-brand); transform: translateY(-2px); box-shadow: var(--shadow-md); }
.type-card.is-active { border-color: var(--ui-brand); background: var(--ui-brand-soft); box-shadow: var(--shadow-ring); }
.type-card__icon { font-size: 28px; margin-bottom: var(--space-2); }
.type-card__title { font-size: var(--fs-sm); font-weight: var(--fw-semibold); color: var(--ui-fg); margin-bottom: var(--space-1); }
.type-card__desc { font-size: var(--fs-xs); color: var(--ui-fg-3); line-height: var(--lh-relaxed); }

.wizard-divider {
  font-size: var(--fs-xs); font-weight: var(--fw-semibold);
  color: var(--ui-fg-3); letter-spacing: 0.5px;
  margin-bottom: var(--space-3); padding-bottom: var(--space-2);
  border-bottom: 1px solid var(--ui-border);
}
.divider-hint { font-weight: var(--fw-regular); color: var(--ui-fg-4); margin-left: var(--space-2); }
.wizard-footer { margin-top: var(--space-5); display: flex; justify-content: flex-end; }

.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
.form-field { display: flex; flex-direction: column; gap: var(--space-2); }
.form-field--full { grid-column: span 2; }
.form-label { font-size: var(--fs-sm); color: var(--ui-fg); font-weight: var(--fw-medium); }
.form-required { color: var(--ui-danger); margin-left: var(--space-1); }
.form-hint { font-size: var(--fs-xs); color: var(--ui-fg-3); line-height: 1.4; }

.runtime-chips { display: flex; flex-wrap: wrap; gap: var(--space-2); padding-bottom: var(--space-3); }
.runtime-chips--sm { padding: 0; }
.runtime-chip {
  display: flex; align-items: center; gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-size: var(--fs-sm); color: var(--ui-fg-3);
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
}
.runtime-chip:hover { border-color: var(--ui-brand); color: var(--ui-brand-fg); }
.runtime-chip.is-active { border-color: var(--ui-brand); background: var(--ui-brand-soft); color: var(--ui-brand-fg); font-weight: var(--fw-medium); }
.rt-icon { font-size: 16px; }
.rt-label { font-size: var(--fs-sm); }

.cfg-grid {
  display: grid; grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-2) var(--space-6);
}
@media (max-width: 720px) { .cfg-grid { grid-template-columns: 1fr; } }
.cfg-cell { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) 0; min-width: 0; }
.cfg-cell .lbl { flex-shrink: 0; width: 84px; font-size: var(--fs-xs); color: var(--ui-fg-3); }
.cfg-cell .val { font-size: var(--fs-sm); color: var(--ui-fg); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }

.cf-empty { font-size: var(--fs-sm); color: var(--ui-fg-3); line-height: 1.6; padding: var(--space-2) 0; }
.cf-empty code { font-family: var(--font-mono); font-size: var(--fs-xs); background: var(--ui-bg-2); padding: 1px 6px; border-radius: var(--radius-sm); color: var(--ui-brand-fg); }
.cf-row {
  display: flex; align-items: center; gap: var(--space-2);
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--ui-border);
}
.cf-row:last-child { border-bottom: none; }
.cf-name { font-family: var(--font-mono); font-size: var(--fs-sm); font-weight: var(--fw-medium); color: var(--ui-fg); flex: 1; }
.cf-ext-badge {
  font-size: var(--fs-xs); color: var(--ui-fg-3);
  background: var(--ui-bg-2); padding: 1px 6px;
  border-radius: var(--radius-sm); text-transform: uppercase;
}
.cf-actions { margin-left: auto; display: flex; gap: var(--space-1); }

.upload-card { margin-bottom: var(--space-3); }
.upload-zone {
  border: 1px dashed var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-6);
  text-align: center; cursor: pointer;
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
  position: relative;
  margin-bottom: var(--space-3);
}
.upload-zone:hover, .upload-zone.is-drag { border-color: var(--ui-brand); background: var(--ui-brand-soft); }
.upload-zone.is-active { border-color: var(--ui-success); background: var(--ui-success-soft); border-style: solid; }
.file-input-hidden { position: absolute; opacity: 0; width: 0; height: 0; }
.upload-zone__icon { font-size: 28px; margin-bottom: var(--space-2); }
.upload-zone__text { font-size: var(--fs-sm); font-weight: var(--fw-medium); color: var(--ui-fg); }
.upload-zone__hint { font-size: var(--fs-xs); color: var(--ui-fg-3); margin-top: var(--space-1); }

.upload-progress { margin-bottom: var(--space-3); }
.upload-progress__head { display: flex; justify-content: space-between; align-items: center; font-size: var(--fs-sm); color: var(--ui-fg-3); margin-bottom: var(--space-2); }
.upload-size { font-size: var(--fs-xs); color: var(--ui-fg-3); }
.upload-actions { display: flex; gap: var(--space-2); }

.terminal-card { padding: 0 !important; }
.term-status { padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--ui-border); }
.deploy-terminal {
  background: #0A0A0A; color: #E4E4E7;
  font-family: var(--font-mono); font-size: 12.5px; line-height: 1.65;
  padding: var(--space-4);
  overflow-y: auto;
  white-space: pre-wrap; word-break: break-all;
  max-height: 480px; min-height: 120px;
  margin: 0;
}
.terminal-placeholder { padding: var(--space-6); text-align: center; font-size: var(--fs-sm); color: var(--ui-fg-3); }

.env-empty { font-size: var(--fs-sm); color: var(--ui-fg-3); padding: var(--space-2) 0; }
.env-row { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1) 0; }
.env-key { width: 180px; flex-shrink: 0; }
.env-value { flex: 1; }

.webhook-row { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-3); }
.webhook-label { font-size: var(--fs-sm); color: var(--ui-fg-3); width: 100px; flex-shrink: 0; }
.webhook-url {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  background: var(--ui-bg-2); padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm); flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  border: 1px solid var(--ui-border);
}
.webhook-alert { margin-top: var(--space-2); }

.cf-editor-toolbar {
  display: flex; align-items: center; gap: var(--space-2);
  padding-bottom: var(--space-2);
}
.cf-editor-filename { font-family: var(--font-mono); font-size: var(--fs-sm); font-weight: var(--fw-medium); flex: 1; }
.code-editor { height: 420px; overflow: hidden; border-radius: var(--radius-sm); border: 1px solid var(--ui-border); }
:deep(.cm-editor) { height: 100%; }

.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

:deep(.mono) {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  background: var(--ui-bg-2); border: 1px solid var(--ui-border);
  padding: 1px 6px; border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
}
:deep(.text-danger) { color: var(--ui-danger-fg); }
:deep(.log-detail) {
  background: #0A0A0A; color: #E4E4E7;
  font-family: var(--font-mono); font-size: 12px; line-height: 1.6;
  padding: var(--space-3); border-radius: var(--radius-sm);
  white-space: pre-wrap; word-break: break-all;
  max-height: 300px; overflow-y: auto;
  margin: var(--space-2);
}
</style>

<style>
/* Teleport 到 #app-bar-actions，需用全局样式 */
.deploy-bar {
  display: flex; align-items: center; gap: var(--space-3);
}
.deploy-bar__status {
  display: inline-flex; align-items: center; gap: var(--space-2);
  padding: 4px 10px;
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-pill);
  font-size: var(--fs-sm); color: var(--ui-fg-2);
}
.deploy-bar__dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
}
.deploy-bar__dot.dot--ok   { background: var(--ui-success); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-success) 22%, transparent); }
.deploy-bar__dot.dot--warn { background: var(--ui-warning); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-warning) 22%, transparent); }
.deploy-bar__dot.dot--err  { background: var(--ui-danger);  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-danger) 22%, transparent); }
.deploy-bar__dot.dot--info { background: var(--ui-brand);   box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-brand) 22%, transparent); animation: deploy-bar-blink 1.2s ease-in-out infinite; }
.deploy-bar__dot.dot--idle { background: var(--ui-fg-4); opacity: .55; }
@keyframes deploy-bar-blink { 50% { opacity: .45; } }

.deploy-bar__label { font-weight: var(--fw-medium); color: var(--ui-fg); }
.deploy-bar__ver {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  background: var(--ui-bg-1);
  border: 1px solid var(--ui-border);
  padding: 1px 6px; border-radius: var(--radius-sm);
  color: var(--ui-fg);
}
.deploy-bar__ver--target { color: var(--ui-brand-fg); border-color: var(--ui-brand); }
.deploy-bar__arrow { color: var(--ui-fg-3); }

.deploy-bar--drift .deploy-bar__status { border-color: var(--ui-warning); background: var(--ui-warning-soft); }
.deploy-bar--running .deploy-bar__status { border-color: var(--ui-brand); background: var(--ui-brand-soft); }

.deploy-bar__actions { display: inline-flex; align-items: center; gap: var(--space-2); }
.deploy-bar__running {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: var(--fs-xs); color: var(--ui-brand-fg);
}
.deploy-bar__running .spin { animation: deploy-bar-spin 1s linear infinite; }
@keyframes deploy-bar-spin { to { transform: rotate(360deg); } }
</style>
