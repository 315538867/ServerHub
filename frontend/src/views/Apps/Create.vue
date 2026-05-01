<template>
  <div class="create-page">
    <div class="create-card">
      <div class="create-header">
        <h2 class="create-title">新建项目</h2>
        <p class="create-subtitle">分 2 步完成项目创建：基本信息 → 网络暴露</p>
        <NSteps :current="step + 1" class="create-steps" size="small">
          <NStep v-for="(s, i) in stepDefs" :key="i" :title="s.title" :description="s.subtitle" />
        </NSteps>
      </div>

      <div class="create-body">
        <transition name="fade-step" mode="out-in">
          <!-- Step 0: 基本信息 -->
          <div v-if="step === 0" key="s0" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">项目名称 <span class="form-required">*</span></label>
                <NInput v-model:value="form.name" placeholder="例如：my-blog" autofocus />
                <span class="form-hint">唯一标识符，创建后不可修改</span>
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
                <span class="form-hint">项目的所有运行操作（部署、容器、终端）都将在该服务器上执行</span>
              </div>
            </div>
          </div>

          <!-- Step 1: 网络暴露 -->
          <div v-else key="s1" class="step-pane">
            <div class="form-grid">
              <div class="form-field">
                <label class="form-label">Nginx 暴露方式</label>
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
                <label class="form-label">Nginx 站点名</label>
                <NInput v-model:value="form.site_name" :placeholder="form.expose_mode === 'site' ? '将自动按域名生成' : 'conf.d 中的配置文件名'" />
                <span class="form-hint">填写后可在项目「网络」Tab 管理对应 Nginx 站点配置</span>
              </div>

              <div v-if="form.expose_mode === 'site'" class="form-field">
                <label class="form-label">域名 <span class="form-required">*</span></label>
                <NInput v-model:value="form.domain" placeholder="blog.example.com" />
                <span class="form-hint">独立站点的访问域名，Nginx 将以此域名生成 server_name 配置</span>
              </div>

              <div v-if="urlPreview" class="form-field">
                <label class="form-label">访问入口预览</label>
                <div class="url-preview">
                  <Globe :size="14" />
                  <code>{{ urlPreview }}</code>
                </div>
                <span class="form-hint">创建项目后自动生成 draft Ingress，前往对应 Edge 服务器「应用配置」后生效</span>
              </div>
            </div>

            <div class="summary-block">
              <div class="summary-title">即将创建</div>
              <div class="summary-grid">
                <div><span class="sl">项目</span><span class="sv">{{ form.name || '—' }}</span></div>
                <div><span class="sl">服务器</span><span class="sv">{{ serverName }}</span></div>
                <div><span class="sl">网络</span><span class="sv">{{ exposeSummary }}</span></div>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <div class="create-footer">
        <UiButton variant="ghost" size="sm" @click="$router.back()">取消</UiButton>
        <span class="footer-spacer" />
        <UiButton v-if="step > 0" variant="secondary" size="sm" @click="step--">上一步</UiButton>
        <UiButton v-if="step === 0" variant="primary" size="sm" :disabled="!canNext" @click="onNext">下一步</UiButton>
        <UiButton v-else variant="primary" size="sm" :loading="saving" :disabled="!canCreate" @click="handleCreate">完成创建</UiButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NSteps, NStep, NInput, NSelect, useMessage } from 'naive-ui'
import { Globe } from 'lucide-vue-next'
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
  { title: '网络暴露', subtitle: 'Nginx 模式' },
]

const form = reactive({
  name: '', description: '',
  server_id: null as number | null,
  domain: '', site_name: '',
  expose_mode: 'none' as 'none' | 'path' | 'site',
})

const exposeOptions = [
  { value: 'none', icon: '🚫', label: '不暴露',   desc: '内部服务或自管反代' },
  { value: 'path', icon: '🔀', label: '路径转发', desc: '反代到已有 Nginx 站点' },
  { value: 'site', icon: '🌐', label: '独立站点', desc: '新建 server 块 + 域名' },
] as const

const serverOptions = computed(() => serverStore.servers.map(s => ({
  label: `${s.name} · ${s.host}`, value: s.id,
})))

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

const urlPreview = computed(() => {
  if (form.expose_mode === 'none' || !form.domain) return ''
  if (form.expose_mode === 'site') return `https://${form.domain}`
  if (form.expose_mode === 'path') {
    if (!form.domain || !form.site_name) return ''
    return `https://${form.domain}/${form.site_name}`
  }
  return ''
})

const canNext = computed(() => !!(form.name && form.server_id))
const canCreate = computed(() => {
  if (form.expose_mode === 'site' && !form.domain) return false
  return true
})

function onNext() {
  if (!canNext.value) {
    message.warning('请填写项目名称和服务器')
    return
  }
  step.value++
}

async function handleCreate() {
  if (!form.name || !form.server_id) { step.value = 0; message.warning('请填写项目名称和服务器'); return }
  if (form.expose_mode === 'site' && !form.domain) { message.warning('独立站点模式需填写域名'); return }
  saving.value = true
  try {
    const app = await createApp(form as any)
    await appStore.fetch()
    const urlHint = app.access_url
      ? `访问入口：${app.access_url}（配置 Ingress 路由后生效）`
      : '项目创建成功'
    message.success(urlHint)
    router.push(`/apps/${app.id}/overview`)
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

.create-body { padding: var(--space-1) 0; min-height: 280px; }
.step-pane { padding: var(--space-5) var(--space-6) var(--space-4); }

.form-grid { display: flex; flex-direction: column; gap: var(--space-4); }
.form-field { display: flex; flex-direction: column; gap: var(--space-2); }
.form-label {
  font-size: var(--fs-sm); color: var(--ui-fg);
  font-weight: var(--fw-medium); line-height: 1;
}
.form-required { color: var(--ui-danger); margin-left: var(--space-1); }
.form-hint { font-size: var(--fs-xs); color: var(--ui-fg-3); line-height: 1.45; }

.expose-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
}

.expose-card {
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
.expose-card:hover {
  border-color: var(--ui-brand);
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}
.expose-card.is-active {
  border-color: var(--ui-brand);
  background: var(--ui-brand-soft);
  box-shadow: var(--shadow-ring);
}

.ec-icon {
  font-size: 24px;
  margin-bottom: var(--space-2);
  line-height: 1;
}
.ec-title {
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  margin-bottom: var(--space-1);
}
.ec-desc {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  line-height: 1.45;
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

.url-preview {
  display: inline-flex; align-items: center; gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--ui-brand-soft);
  border: 1px solid color-mix(in srgb, var(--ui-brand) 30%, transparent);
  border-radius: var(--radius-sm);
  color: var(--ui-brand-fg);
  font-size: var(--fs-sm);
}
.url-preview code {
  font-family: var(--font-mono);
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
