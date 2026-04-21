<template>
  <div class="page-container create-page">
    <div class="create-card">

      <!-- 顶部步骤条 -->
      <div class="create-header">
        <h2 class="create-title">新建应用</h2>
        <p class="create-subtitle">分 4 步完成应用创建：信息 → 容器 → 网络 → 部署</p>
        <t-steps :current="step" class="create-steps" theme="dot">
          <t-step-item v-for="(s, i) in stepDefs" :key="i" :title="s.title" :content="s.subtitle" />
        </t-steps>
      </div>

      <!-- 主体 -->
      <div class="create-body">

        <!-- ── Step 0：基本信息 ───────────────────────────────────── -->
        <transition name="fade-step" mode="out-in">
          <div v-if="step === 0" key="s0" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">应用名称 <span class="form-required">*</span></label>
                <t-input v-model="form.name" placeholder="例如：my-blog" autofocus />
                <span class="form-hint">唯一标识符，创建后不可修改；将用于自动生成基础目录</span>
              </div>
              <div class="form-field">
                <label class="form-label">描述</label>
                <t-input v-model="form.description" placeholder="简短描述用途（可选）" />
              </div>
              <div class="form-field">
                <label class="form-label">关联服务器 <span class="form-required">*</span></label>
                <t-select v-model="form.server_id" placeholder="选择服务器">
                  <t-option v-for="s in serverStore.servers" :key="s.id" :label="`${s.name} · ${s.host}`" :value="s.id" />
                </t-select>
                <span class="form-hint">应用的所有运行操作（部署、容器、终端）都将在该服务器上执行</span>
              </div>
            </div>
          </div>

          <!-- ── Step 1：容器与目录 ────────────────────────────────── -->
          <div v-else-if="step === 1" key="s1" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">Docker 容器名</label>
                <t-input v-model="form.container_name" placeholder="例如：my-nginx（可选）" />
                <span class="form-hint">关联已有/即将创建的容器名；填写后开启「运维」Tab，可看实时指标、日志、控制</span>
              </div>
              <div class="form-field">
                <label class="form-label">应用基础目录</label>
                <t-input v-model="form.base_dir" :placeholder="`/srv/apps/${form.name || 'my-app'}`" />
                <span class="form-hint">服务器上自动创建 data / logs / config / backup 子目录；留空按应用名自动填充</span>
              </div>
            </div>
            <t-alert theme="info" class="step-alert">
              <template #title>这一步全部可选</template>
              如果暂不绑定容器或目录，可点「跳过」直接进入下一步——后续仍可在「总览」编辑。
            </t-alert>
          </div>

          <!-- ── Step 2：网络暴露 ────────────────────────────────── -->
          <div v-else-if="step === 2" key="s2" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">Nginx 暴露方式 <span class="form-required">*</span></label>
                <div class="expose-cards">
                  <div
                    v-for="o in exposeOptions"
                    :key="o.value"
                    class="expose-card"
                    :class="{ 'is-active': form.expose_mode === o.value }"
                    @click="form.expose_mode = o.value"
                  >
                    <div class="ec-icon">{{ o.icon }}</div>
                    <div class="ec-title">{{ o.label }}</div>
                    <div class="ec-desc">{{ o.desc }}</div>
                  </div>
                </div>
              </div>

              <div v-if="form.expose_mode !== 'none'" class="form-field">
                <label class="form-label">Nginx 站点</label>
                <t-input v-model="form.site_name" :placeholder="form.expose_mode === 'site' ? '将自动按域名生成' : 'conf.d 中的配置文件名'" />
                <span class="form-hint">填写后开启「网络 / 域名」Tab，用于管理对应 Nginx 站点配置</span>
              </div>

              <div v-if="form.expose_mode === 'site'" class="form-field">
                <label class="form-label">域名 <span class="form-required">*</span></label>
                <t-input v-model="form.domain" placeholder="blog.example.com" />
                <span class="form-hint">独立站点的访问域名，Nginx 将以此域名生成 server_name 配置</span>
              </div>
            </div>
          </div>

          <!-- ── Step 3：部署方式 ────────────────────────────────── -->
          <div v-else key="s3" class="step-pane">
            <div class="deploy-type-cards">
              <div
                v-for="t in deployTypes" :key="t.value"
                class="deploy-type-card"
                :class="{ 'is-active': deployType === t.value }"
                @click="deployType = deployType === t.value ? '' : t.value"
              >
                <div class="dtc-icon">{{ t.icon }}</div>
                <div class="dtc-title">{{ t.label }}</div>
                <div class="dtc-desc">{{ t.desc }}</div>
              </div>
            </div>

            <transition name="fade-step">
              <div v-if="deployType" class="deploy-type-fields">
                <div class="form-grid">
                  <div class="form-field">
                    <label class="form-label">工作目录</label>
                    <t-input v-model="deployForm.work_dir" :placeholder="form.base_dir || '/srv/apps/...'" />
                  </div>
                  <template v-if="deployType === 'docker-compose'">
                    <div class="form-field">
                      <label class="form-label">Compose 文件名</label>
                      <t-input v-model="deployForm.compose_file" placeholder="docker-compose.yml" />
                    </div>
                    <div class="form-field">
                      <label class="form-label">镜像名（可选）</label>
                      <t-input v-model="deployForm.image_name" placeholder="例如：nginx:latest" />
                    </div>
                  </template>
                  <template v-if="deployType === 'docker'">
                    <div class="form-field">
                      <label class="form-label">镜像名 <span class="form-required">*</span></label>
                      <t-input v-model="deployForm.image_name" placeholder="例如：nginx:latest" />
                    </div>
                  </template>
                </div>
              </div>
              <t-alert v-else theme="info" class="step-alert">
                <template #title>暂不配置部署？</template>
                可点「完成创建」直接建好应用，稍后随时在「部署」Tab 配置。
              </t-alert>
            </transition>

            <!-- 摘要预览 -->
            <div class="summary-block">
              <div class="summary-title">即将创建</div>
              <div class="summary-grid">
                <div><span class="sl">应用</span><span class="sv">{{ form.name || '—' }}</span></div>
                <div><span class="sl">服务器</span><span class="sv">{{ serverName }}</span></div>
                <div><span class="sl">容器</span><span class="sv">{{ form.container_name || '未绑定' }}</span></div>
                <div><span class="sl">基础目录</span><span class="sv">{{ form.base_dir || '—' }}</span></div>
                <div><span class="sl">网络</span><span class="sv">{{ exposeSummary }}</span></div>
                <div><span class="sl">部署方式</span><span class="sv">{{ deployType ? deployTypes.find(t=>t.value===deployType)?.label : '稍后配置' }}</span></div>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <!-- 底部操作 -->
      <div class="create-footer">
        <t-button variant="text" @click="$router.back()">取消</t-button>
        <span class="footer-spacer" />
        <t-button v-if="step > 0" variant="outline" @click="step--">上一步</t-button>
        <t-button v-if="step === 1 || (step === 3 && !deployType)" variant="outline" @click="onSkip">跳过</t-button>
        <t-button v-if="step < 3" theme="primary" :disabled="!canNext" @click="onNext">下一步</t-button>
        <t-button v-else theme="primary" :loading="saving" :disabled="!canCreate" @click="handleCreate">完成创建</t-button>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { createApp, updateApp } from '@/api/application'
import { createDeploy } from '@/api/deploy'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const serverStore = useServerStore()
const appStore = useAppStore()
const saving = ref(false)

const step = ref(0)
const stepDefs = [
  { title: '基本信息', subtitle: '名称 / 服务器' },
  { title: '容器目录', subtitle: '容器 / base_dir' },
  { title: '网络暴露', subtitle: 'Nginx 模式' },
  { title: '部署方式', subtitle: 'Compose / Docker / 文件' },
]

const form = reactive({
  name: '', description: '',
  server_id: null as number | null,
  domain: '', site_name: '', container_name: '',
  base_dir: '',
  expose_mode: 'none' as 'none' | 'path' | 'site',
  deploy_id: null as number | null, db_conn_id: null as number | null,
})

const exposeOptions = [
  { value: 'none', icon: '🚫', label: '不暴露',   desc: '内部服务或自管反代' },
  { value: 'path', icon: '🔀', label: '路径转发', desc: '反代到已有 Nginx 站点' },
  { value: 'site', icon: '🌐', label: '独立站点', desc: '新建 server 块 + 域名' },
] as const

const deployTypes = [
  { value: 'docker-compose', icon: '🐙', label: 'Docker Compose', desc: 'compose 文件管理多容器' },
  { value: 'docker',         icon: '🐳', label: 'Docker',         desc: '直接拉取镜像并运行' },
  { value: 'native',         icon: '📦', label: '文件部署',        desc: '上传可执行文件 + 启动命令' },
] as const
type DeployTypeVal = typeof deployTypes[number]['value']

const deployType = ref<DeployTypeVal | ''>('')
const deployForm = reactive({ work_dir: '', compose_file: 'docker-compose.yml', image_name: '', start_cmd: '' })

watch(() => form.name, (name, oldName) => {
  const autoOld = oldName ? `/srv/apps/${oldName}` : ''
  if (!form.base_dir || form.base_dir === autoOld) {
    form.base_dir = name ? `/srv/apps/${name}` : ''
  }
})
watch(() => form.base_dir, (dir) => {
  if (!deployForm.work_dir) deployForm.work_dir = dir
})

const serverName = computed(() => {
  const s = serverStore.servers.find(x => x.id === form.server_id)
  return s ? `${s.name} · ${s.host}` : '—'
})
const exposeSummary = computed(() => {
  const o = exposeOptions.find(o => o.value === form.expose_mode)
  if (!o) return '—'
  if (form.expose_mode === 'site') return `${o.label}（${form.domain || '待填'}）`
  if (form.expose_mode === 'path') return `${o.label}${form.site_name ? '（'+form.site_name+'）' : ''}`
  return o.label
})

const canNext = computed(() => {
  if (step.value === 0) return !!(form.name && form.server_id)
  if (step.value === 2) return form.expose_mode !== 'site' || !!form.domain
  return true
})
const canCreate = computed(() => {
  if (deployType.value === 'docker' && !deployForm.image_name) return false
  return true
})

function onNext() {
  if (!canNext.value) {
    if (step.value === 0) MessagePlugin.warning('请填写应用名称和服务器')
    else if (step.value === 2) MessagePlugin.warning('独立站点模式需填写域名')
    return
  }
  step.value++
}
function onSkip() {
  if (step.value === 1) { step.value = 2; return }
  if (step.value === 3) handleCreate()
}

async function handleCreate() {
  if (!form.name || !form.server_id) { step.value = 0; MessagePlugin.warning('请填写应用名称和服务器'); return }
  if (form.expose_mode === 'site' && !form.domain) { step.value = 2; MessagePlugin.warning('独立站点模式需填写域名'); return }
  if (deployType.value === 'docker' && !deployForm.image_name) { MessagePlugin.warning('Docker 部署需填写镜像名'); return }
  saving.value = true
  try {
    const app = await createApp(form as any)
    if (deployType.value) {
      const deploy = await createDeploy({
        name: app.name,
        server_id: app.server_id,
        type: deployType.value,
        work_dir: deployForm.work_dir || form.base_dir,
        compose_file: deployForm.compose_file || 'docker-compose.yml',
        image_name: deployForm.image_name,
        start_cmd: deployForm.start_cmd,
      })
      await updateApp(app.id, { ...app, deploy_id: deploy.id } as any)
      MessagePlugin.success('应用创建成功，已关联部署配置')
      await appStore.fetch()
      router.push(`/apps/${app.id}/deploy`)
    } else {
      MessagePlugin.success('应用创建成功')
      await appStore.fetch()
      router.push(`/apps/${app.id}/overview`)
    }
  } catch (e: any) {
    MessagePlugin.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => serverStore.fetch())
</script>

<style scoped>
.create-page {
  display: flex; justify-content: center; align-items: flex-start;
  padding: var(--ui-space-5);
}
.create-card {
  width: 100%; max-width: 820px;
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-xl);
  box-shadow: var(--ui-shadow-md);
  overflow: hidden;
  opacity: 0;
  animation: ui-scale-in var(--ui-dur-slow) var(--ui-ease-spring) forwards;
}

.create-header {
  padding: var(--ui-space-5) var(--ui-space-6) var(--ui-space-3);
  border-bottom: 1px solid var(--ui-border);
  background:
    radial-gradient(circle at 0% 0%, var(--ui-brand-soft), transparent 40%),
    var(--ui-bg-surface);
}
.create-title {
  margin: 0 0 var(--ui-space-1);
  font-size: var(--ui-fs-lg);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  letter-spacing: var(--ui-tracking-tight);
}
.create-subtitle {
  margin: 0 0 var(--ui-space-4);
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
}
.create-steps { padding: var(--ui-space-1) 0 var(--ui-space-3); }

.create-body { padding: var(--ui-space-1) 0; min-height: 320px; }
.step-pane { padding: var(--ui-space-5) var(--ui-space-6) var(--ui-space-4); }
.step-alert { margin-top: var(--ui-space-4); }

.form-grid { display: flex; flex-direction: column; gap: var(--ui-space-4); }
.form-field {
  display: flex; flex-direction: column; gap: var(--ui-space-2);
  opacity: 0;
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard) forwards;
}
.form-field:nth-child(1) { animation-delay: 60ms; }
.form-field:nth-child(2) { animation-delay: 120ms; }
.form-field:nth-child(3) { animation-delay: 180ms; }
.form-field:nth-child(4) { animation-delay: 240ms; }
.form-label {
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg);
  font-weight: var(--ui-fw-medium);
  line-height: 1;
}
.form-required { color: var(--ui-danger); margin-left: var(--ui-space-1); }
.form-hint { font-size: var(--ui-fs-xs); color: var(--ui-fg-3); line-height: 1.45; }

/* 选择卡（通用） */
.expose-cards, .deploy-type-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--ui-space-3);
}
.deploy-type-cards { margin-bottom: var(--ui-space-4); }

.expose-card, .deploy-type-card {
  position: relative;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  padding: var(--ui-space-4) var(--ui-space-3);
  cursor: pointer;
  text-align: center;
  background: var(--ui-bg-surface);
  transition:
    border-color var(--ui-dur-fast) var(--ui-ease-standard),
    background var(--ui-dur-fast) var(--ui-ease-standard),
    box-shadow var(--ui-dur-fast) var(--ui-ease-standard),
    transform var(--ui-dur-fast) var(--ui-ease-standard);
  overflow: hidden;
}
.expose-card::after, .deploy-type-card::after {
  content: '';
  position: absolute; inset: 0;
  background: radial-gradient(circle at 50% 0%, var(--ui-brand-soft), transparent 70%);
  opacity: 0;
  transition: opacity var(--ui-dur-base) var(--ui-ease-standard);
  pointer-events: none;
}
.expose-card:hover, .deploy-type-card:hover {
  border-color: var(--ui-brand);
  transform: translateY(-2px);
  box-shadow: var(--ui-shadow-md);
}
.expose-card:hover::after, .deploy-type-card:hover::after { opacity: 0.5; }

.expose-card.is-active, .deploy-type-card.is-active {
  border-color: var(--ui-brand);
  background: var(--ui-brand-soft);
  box-shadow: 0 0 0 3px var(--ui-brand-ring);
}
.expose-card.is-active::after, .deploy-type-card.is-active::after { opacity: 1; }

.ec-icon, .dtc-icon {
  font-size: 24px;
  margin-bottom: var(--ui-space-2);
  line-height: 1;
  position: relative;
  z-index: 1;
}
.ec-title, .dtc-title {
  font-size: var(--ui-fs-sm);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  margin-bottom: var(--ui-space-1);
  position: relative; z-index: 1;
}
.ec-desc, .dtc-desc {
  font-size: var(--ui-fs-2xs);
  color: var(--ui-fg-3);
  line-height: 1.45;
  position: relative; z-index: 1;
}
.deploy-type-fields {
  border-top: 1px dashed var(--ui-border-subtle);
  padding-top: var(--ui-space-4);
}

/* 摘要 */
.summary-block {
  margin-top: var(--ui-space-5);
  padding: var(--ui-space-4);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border-subtle);
  border-radius: var(--ui-radius-md);
  position: relative;
  overflow: hidden;
}
.summary-block::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0; width: 3px;
  background: linear-gradient(180deg, var(--ui-brand), var(--ui-brand-hover));
}
.summary-title {
  font-size: var(--ui-fs-2xs);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: 0.6px;
  margin-bottom: var(--ui-space-3);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--ui-space-2) var(--ui-space-5);
  font-size: var(--ui-fs-sm);
}
.summary-grid > div { display: flex; gap: var(--ui-space-2); min-width: 0; }
.sl { color: var(--ui-fg-3); flex-shrink: 0; min-width: 64px; font-size: var(--ui-fs-xs); }
.sv {
  color: var(--ui-fg);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-weight: var(--ui-fw-medium);
}

/* 底部 */
.create-footer {
  display: flex; align-items: center; gap: var(--ui-space-2);
  padding: var(--ui-space-3) var(--ui-space-6);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-subtle);
}
.footer-spacer { flex: 1; }

/* 步骤切换动画（更强的弹性） */
.fade-step-enter-active {
  transition: opacity var(--ui-dur-base) var(--ui-ease-standard),
              transform var(--ui-dur-base) var(--ui-ease-spring);
}
.fade-step-leave-active {
  transition: opacity var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard);
}
.fade-step-enter-from { opacity: 0; transform: translateX(16px); }
.fade-step-leave-to   { opacity: 0; transform: translateX(-16px); }

@media (prefers-reduced-motion: reduce) {
  .create-card, .form-field { animation: none; opacity: 1; }
  .fade-step-enter-active, .fade-step-leave-active { transition: opacity 0.1s; }
  .fade-step-enter-from, .fade-step-leave-to { transform: none; }
}
</style>
