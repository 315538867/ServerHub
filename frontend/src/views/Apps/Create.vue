<template>
  <div class="create-page">
    <div class="create-card">
      <div class="create-header">
        <h2 class="create-title">新建应用</h2>
        <p class="create-subtitle">分 4 步完成应用创建：信息 → 容器 → 网络 → 部署</p>
        <NSteps :current="step + 1" class="create-steps" size="small">
          <NStep v-for="(s, i) in stepDefs" :key="i" :title="s.title" :description="s.subtitle" />
        </NSteps>
      </div>

      <div class="create-body">
        <transition name="fade-step" mode="out-in">
          <div v-if="step === 0" key="s0" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">应用名称 <span class="form-required">*</span></label>
                <NInput v-model:value="form.name" placeholder="例如：my-blog" autofocus />
                <span class="form-hint">唯一标识符，创建后不可修改；将用于自动生成基础目录</span>
              </div>
              <div class="form-field">
                <label class="form-label">描述</label>
                <NInput v-model:value="form.description" placeholder="简短描述用途（可选）" />
              </div>
              <div class="form-field">
                <label class="form-label">关联服务器 <span class="form-required">*</span></label>
                <NSelect
                  v-model:value="form.server_id"
                  placeholder="选择服务器"
                  :options="serverOptions"
                />
                <span class="form-hint">应用的所有运行操作（部署、容器、终端）都将在该服务器上执行</span>
              </div>
            </div>
          </div>

          <div v-else-if="step === 1" key="s1" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">Docker 容器名</label>
                <NInput v-model:value="form.container_name" placeholder="例如：my-nginx（可选）" />
                <span class="form-hint">关联已有/即将创建的容器名；填写后开启「运维」Tab，可看实时指标、日志、控制</span>
              </div>
              <div class="form-field">
                <label class="form-label">应用基础目录</label>
                <NInput v-model:value="form.base_dir" :placeholder="`/srv/apps/${form.name || 'my-app'}`" />
                <span class="form-hint">服务器上自动创建 data / logs / config / backup 子目录；留空按应用名自动填充</span>
              </div>
            </div>
            <NAlert type="info" title="这一步全部可选" class="step-alert">
              如果暂不绑定容器或目录，可点「跳过」直接进入下一步——后续仍可在「总览」编辑。
            </NAlert>
          </div>

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
                <NInput v-model:value="form.site_name" :placeholder="form.expose_mode === 'site' ? '将自动按域名生成' : 'conf.d 中的配置文件名'" />
                <span class="form-hint">填写后开启「网络 / 域名」Tab，用于管理对应 Nginx 站点配置</span>
              </div>

              <div v-if="form.expose_mode === 'site'" class="form-field">
                <label class="form-label">域名 <span class="form-required">*</span></label>
                <NInput v-model:value="form.domain" placeholder="blog.example.com" />
                <span class="form-hint">独立站点的访问域名，Nginx 将以此域名生成 server_name 配置</span>
              </div>
            </div>
          </div>

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
                    <NInput v-model:value="deployForm.work_dir" :placeholder="form.base_dir || '/srv/apps/...'" />
                  </div>
                  <template v-if="deployType === 'docker-compose'">
                    <div class="form-field">
                      <label class="form-label">Compose 文件名</label>
                      <NInput v-model:value="deployForm.compose_file" placeholder="docker-compose.yml" />
                    </div>
                    <div class="form-field">
                      <label class="form-label">镜像名（可选）</label>
                      <NInput v-model:value="deployForm.image_name" placeholder="例如：nginx:latest" />
                    </div>
                  </template>
                  <template v-if="deployType === 'docker'">
                    <div class="form-field">
                      <label class="form-label">镜像名 <span class="form-required">*</span></label>
                      <NInput v-model:value="deployForm.image_name" placeholder="例如：nginx:latest" />
                    </div>
                  </template>
                </div>
              </div>
              <NAlert v-else type="info" title="暂不配置部署？" class="step-alert">
                可点「完成创建」直接建好应用，稍后随时在「部署」Tab 配置。
              </NAlert>
            </transition>

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

      <div class="create-footer">
        <UiButton variant="ghost" size="sm" @click="$router.back()">取消</UiButton>
        <span class="footer-spacer" />
        <UiButton v-if="step > 0" variant="secondary" size="sm" @click="step--">上一步</UiButton>
        <UiButton v-if="step === 1 || (step === 3 && !deployType)" variant="secondary" size="sm" @click="onSkip">跳过</UiButton>
        <UiButton v-if="step < 3" variant="primary" size="sm" :disabled="!canNext" @click="onNext">下一步</UiButton>
        <UiButton v-else variant="primary" size="sm" :loading="saving" :disabled="!canCreate" @click="handleCreate">完成创建</UiButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NSteps, NStep, NInput, NSelect, NAlert, useMessage } from 'naive-ui'
import { createApp } from '@/api/application'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import UiButton from '@/components/ui/UiButton.vue'

const router = useRouter()
const serverStore = useServerStore()
const appStore = useAppStore()
const message = useMessage()
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

const serverOptions = computed(() => serverStore.servers.map(s => ({
  label: `${s.name} · ${s.host}`, value: s.id,
})))

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
    if (step.value === 0) message.warning('请填写应用名称和服务器')
    else if (step.value === 2) message.warning('独立站点模式需填写域名')
    return
  }
  step.value++
}
function onSkip() {
  if (step.value === 1) { step.value = 2; return }
  if (step.value === 3) handleCreate()
}

async function handleCreate() {
  if (!form.name || !form.server_id) { step.value = 0; message.warning('请填写应用名称和服务器'); return }
  if (form.expose_mode === 'site' && !form.domain) { step.value = 2; message.warning('独立站点模式需填写域名'); return }
  saving.value = true
  try {
    const app = await createApp(form as any)
    // M3：部署方式不再由向导一步创建，用户在 Releases Tab 建 Release 时选择。
    if (deployType.value) {
      message.success('应用创建成功，请前往 Releases Tab 创建首个 Release')
      await appStore.fetch()
      router.push(`/apps/${app.id}/releases`)
    } else {
      message.success('应用创建成功')
      await appStore.fetch()
      router.push(`/apps/${app.id}/overview`)
    }
  } catch (e: any) {
    message.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => serverStore.fetch())
</script>

<style scoped>
.create-page {
  display: flex; justify-content: center; align-items: flex-start;
  padding: var(--space-6);
}
.create-card {
  width: 100%; max-width: 820px;
  background: var(--ui-bg-1);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  overflow: hidden;
}

.create-header {
  padding: var(--space-5) var(--space-6) var(--space-3);
  border-bottom: 1px solid var(--ui-border);
  background:
    radial-gradient(circle at 0% 0%, var(--ui-brand-soft), transparent 40%),
    var(--ui-bg-1);
}
.create-title {
  margin: 0 0 var(--space-1);
  font-size: var(--fs-lg);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}
.create-subtitle {
  margin: 0 0 var(--space-4);
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
}
.create-steps { padding: var(--space-1) 0 var(--space-3); }

.create-body { padding: var(--space-1) 0; min-height: 320px; }
.step-pane { padding: var(--space-5) var(--space-6) var(--space-4); }
.step-alert { margin-top: var(--space-4); }

.form-grid { display: flex; flex-direction: column; gap: var(--space-4); }
.form-field { display: flex; flex-direction: column; gap: var(--space-2); }
.form-label {
  font-size: var(--fs-sm); color: var(--ui-fg);
  font-weight: var(--fw-medium); line-height: 1;
}
.form-required { color: var(--ui-danger); margin-left: var(--space-1); }
.form-hint { font-size: var(--fs-xs); color: var(--ui-fg-3); line-height: 1.45; }

.expose-cards, .deploy-type-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
}
.deploy-type-cards { margin-bottom: var(--space-4); }

.expose-card, .deploy-type-card {
  position: relative;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-4) var(--space-3);
  cursor: pointer;
  text-align: center;
  background: var(--ui-bg-1);
  transition:
    border-color var(--dur-fast) var(--ease),
    background var(--dur-fast) var(--ease),
    box-shadow var(--dur-fast) var(--ease),
    transform var(--dur-fast) var(--ease);
}
.expose-card:hover, .deploy-type-card:hover {
  border-color: var(--ui-brand);
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}
.expose-card.is-active, .deploy-type-card.is-active {
  border-color: var(--ui-brand);
  background: var(--ui-brand-soft);
  box-shadow: var(--shadow-ring);
}

.ec-icon, .dtc-icon {
  font-size: 24px;
  margin-bottom: var(--space-2);
  line-height: 1;
}
.ec-title, .dtc-title {
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  margin-bottom: var(--space-1);
}
.ec-desc, .dtc-desc {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  line-height: 1.45;
}
.deploy-type-fields {
  border-top: 1px dashed var(--ui-border);
  padding-top: var(--space-4);
}

.summary-block {
  margin-top: var(--space-5);
  padding: var(--space-4);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
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
  font-size: var(--fs-xs);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: 0.6px;
  margin-bottom: var(--space-3);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-2) var(--space-5);
  font-size: var(--fs-sm);
}
.summary-grid > div { display: flex; gap: var(--space-2); min-width: 0; }
.sl { color: var(--ui-fg-3); flex-shrink: 0; min-width: 64px; font-size: var(--fs-xs); }
.sv {
  color: var(--ui-fg);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-weight: var(--fw-medium);
}

.create-footer {
  display: flex; align-items: center; gap: var(--space-2);
  padding: var(--space-3) var(--space-6);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-2);
}
.footer-spacer { flex: 1; }

.fade-step-enter-active { transition: opacity var(--dur-base) var(--ease), transform var(--dur-base) var(--ease); }
.fade-step-leave-active { transition: opacity var(--dur-fast) var(--ease), transform var(--dur-fast) var(--ease); }
.fade-step-enter-from { opacity: 0; transform: translateX(16px); }
.fade-step-leave-to   { opacity: 0; transform: translateX(-16px); }
</style>
