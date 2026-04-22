<template>
  <div class="setup-page">
    <div class="setup-box">
      <div class="setup-header">
        <div class="setup-brand">
          <div class="setup-brand-icon">S</div>
          <span class="setup-brand-name">ServerHub</span>
        </div>
        <h2 class="setup-title">首次初始化</h2>
        <p class="setup-sub">完成以下步骤，开始使用 ServerHub</p>
      </div>

      <NSteps :current="currentStep" size="small" class="setup-steps">
        <NStep title="创建管理员" />
        <NStep v-if="needsLocal" title="本机纳管" />
        <NStep title="完成" />
      </NSteps>

      <!-- Step 1: create admin -->
      <section v-if="stepKey === 'admin'" class="setup-step">
        <p class="setup-hint">创建第一个管理员账号，以便登录面板。</p>
        <NForm ref="adminFormRef" :model="adminForm" :rules="adminRules" label-placement="top" @submit.prevent="handleCreateAdmin">
          <NFormItem label="用户名" path="username">
            <NInput v-model:value="adminForm.username" placeholder="至少 3 字符" size="large" />
          </NFormItem>
          <NFormItem label="密码" path="password">
            <NInput v-model:value="adminForm.password" type="password" show-password-on="click" placeholder="至少 6 字符" size="large" />
          </NFormItem>
          <NFormItem label="确认密码" path="confirm">
            <NInput v-model:value="adminForm.confirm" type="password" show-password-on="click" placeholder="再次输入密码" size="large" @keydown.enter="handleCreateAdmin" />
          </NFormItem>
          <UiButton variant="primary" size="lg" :loading="loading" block @click="handleCreateAdmin">
            创建管理员并继续
          </UiButton>
        </NForm>
      </section>

      <!-- Step 2: local host bootstrap -->
      <section v-else-if="stepKey === 'local'" class="setup-step">
        <p class="setup-hint">
          ServerHub 运行在容器里，需要通过 SSH 回连宿主机才能管理 Nginx / Docker / systemd。
          下面的命令会在宿主上授权 ServerHub 的公钥并配置免密 sudo。
        </p>

        <NFormItem label="宿主机 SSH 用户名" path="targetUser">
          <NInput v-model:value="targetUser" placeholder="如 ubuntu / root / ec2-user" size="large" :disabled="!!initResult" />
        </NFormItem>

        <UiButton v-if="!initResult" variant="primary" size="lg" :loading="loading" block @click="handleInit">
          生成 SSH 密钥
        </UiButton>

        <template v-if="initResult">
          <div class="setup-code-label">
            请在宿主机（<code>{{ initResult.host_gateway }}</code>）上以 root 或可 sudo 的账号执行：
          </div>
          <NCode :code="initResult.command" language="bash" class="setup-code" show-line-numbers />
          <div class="setup-code-actions">
            <UiButton variant="ghost" size="md" @click="copyCmd">{{ copied ? '已复制' : '复制命令' }}</UiButton>
            <UiButton variant="ghost" size="md" @click="resetInit">重新生成</UiButton>
          </div>

          <UiButton variant="primary" size="lg" :loading="loading" block class="setup-activate" @click="handleActivate">
            我已在宿主上执行 → 连接本机
          </UiButton>
        </template>
      </section>

      <!-- Step 3: finish -->
      <section v-else class="setup-step">
        <NResult status="success" title="初始化完成" description="即将跳转到面板首页…" />
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { invalidateSetupCache } from '@/router'
import {
  NForm, NFormItem, NInput, NSteps, NStep, NCode, NResult,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import UiButton from '@/components/ui/UiButton.vue'
import { useAuthStore } from '@/stores/auth'
import {
  getSetupStatus, createAdmin, initLocal, activateLocal,
  type InitLocalResp,
} from '@/api/setup'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(false)
const needsAdmin = ref(true)
const needsLocal = ref(false)
const stepKey = ref<'admin' | 'local' | 'done'>('admin')

const adminFormRef = ref<FormInst | null>(null)
const adminForm = reactive({ username: '', password: '', confirm: '' })

const adminRules: FormRules = {
  username: [{ required: true, min: 3, message: '用户名至少 3 字符', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少 6 字符', trigger: 'blur' }],
  confirm: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule: unknown, val: string) => val === adminForm.password || new Error('两次密码不一致'),
      trigger: ['blur', 'input'],
    },
  ],
}

const targetUser = ref('ubuntu')
const initResult = ref<InitLocalResp | null>(null)
const copied = ref(false)

const currentStep = computed(() => {
  if (stepKey.value === 'admin') return 1
  if (stepKey.value === 'local') return 2
  return needsLocal.value ? 3 : 2
})

onMounted(async () => {
  try {
    const st = await getSetupStatus()
    needsAdmin.value = st.needs_admin
    needsLocal.value = st.needs_local_server
    if (!needsAdmin.value && !needsLocal.value) {
      router.replace('/')
      return
    }
    stepKey.value = needsAdmin.value ? 'admin' : 'local'
  } catch {
    /* already handled by interceptor */
  }
})

async function handleCreateAdmin() {
  try { await adminFormRef.value?.validate() } catch { return }
  loading.value = true
  try {
    await createAdmin(adminForm.username, adminForm.password)
    // Auto-login immediately so the freshly created token drives step 3+.
    await auth.login(adminForm.username, adminForm.password)
    if (needsLocal.value) {
      stepKey.value = 'local'
    } else {
      finish()
    }
  } catch {
    /* error message shown by interceptor */
  } finally {
    loading.value = false
  }
}

async function handleInit() {
  if (!targetUser.value.trim()) {
    message.error('请填写宿主机 SSH 用户名')
    return
  }
  loading.value = true
  try {
    initResult.value = await initLocal(targetUser.value.trim())
  } finally {
    loading.value = false
  }
}

function resetInit() {
  initResult.value = null
  copied.value = false
}

async function copyCmd() {
  if (!initResult.value) return
  try {
    await navigator.clipboard.writeText(initResult.value.command)
    copied.value = true
    message.success('命令已复制')
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    message.warning('浏览器不支持剪贴板 API，请手动选中复制')
  }
}

async function handleActivate() {
  loading.value = true
  try {
    await activateLocal()
    message.success('本机纳管成功')
    finish()
  } catch (e: unknown) {
    const err = e as { msg?: string; message?: string }
    message.error('连接本机失败：' + (err.msg ?? err.message ?? '请检查宿主侧命令是否已执行'))
  } finally {
    loading.value = false
  }
}

function finish() {
  stepKey.value = 'done'
  invalidateSetupCache()
  setTimeout(() => router.replace('/'), 1200)
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background:
    radial-gradient(80% 60% at 0% 0%, rgba(62, 207, 142, .15) 0%, transparent 55%),
    radial-gradient(70% 70% at 100% 100%, rgba(70, 177, 201, .12) 0%, transparent 55%),
    var(--ui-bg-1);
}
.setup-box {
  width: 100%;
  max-width: 620px;
  background: var(--ui-bg-2, #14181C);
  border: 1px solid var(--ui-border, rgba(255,255,255,.08));
  border-radius: var(--radius-lg, 12px);
  padding: var(--space-8);
}
.setup-header { text-align: center; margin-bottom: var(--space-6); }
.setup-brand {
  display: inline-flex; align-items: center; gap: var(--space-2);
  margin-bottom: var(--space-4);
}
.setup-brand-icon {
  width: 32px; height: 32px;
  background: var(--ui-brand, #3ECF8E);
  border-radius: 8px;
  display: grid; place-items: center;
  font-size: 16px; font-weight: 800; color: #0A0A0A;
}
.setup-brand-name { font-size: 16px; font-weight: 700; color: var(--ui-fg, #fff); }
.setup-title {
  font-size: var(--fs-2xl, 26px);
  font-weight: var(--fw-semibold, 600);
  margin: 0 0 var(--space-2);
  letter-spacing: -0.02em;
  color: var(--ui-fg, #fff);
}
.setup-sub {
  margin: 0;
  font-size: var(--fs-sm, 13px);
  color: var(--ui-fg-3, rgba(255,255,255,.55));
}
.setup-steps { margin: var(--space-6) 0; }

.setup-step { margin-top: var(--space-4); }
.setup-hint {
  color: var(--ui-fg-2, rgba(255,255,255,.75));
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: var(--space-4);
}
.setup-code-label {
  font-size: 12px;
  color: var(--ui-fg-3, rgba(255,255,255,.55));
  margin: var(--space-4) 0 var(--space-2);
}
.setup-code-label code {
  color: var(--ui-brand, #3ECF8E);
  font-family: monospace;
}
.setup-code { max-height: 280px; overflow: auto; }
.setup-code-actions {
  display: flex; gap: var(--space-2);
  margin: var(--space-3) 0;
}
.setup-activate { margin-top: var(--space-4); }
</style>
