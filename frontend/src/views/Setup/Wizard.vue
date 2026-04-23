<template>
  <div class="setup-page">
    <div class="setup-box">
      <div class="setup-header">
        <div class="setup-brand">
          <div class="setup-brand-icon">S</div>
          <span class="setup-brand-name">ServerHub</span>
        </div>
        <h2 class="setup-title">首次初始化</h2>
        <p class="setup-sub">创建你的管理员账号，开始使用 ServerHub</p>
      </div>

      <section v-if="stepKey === 'admin'" class="setup-step">
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
            创建管理员并进入面板
          </UiButton>
        </NForm>
      </section>

      <section v-else class="setup-step">
        <NResult status="success" title="初始化完成" description="即将跳转到面板首页…" />
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { invalidateSetupCache } from '@/router'
import {
  NForm, NFormItem, NInput, NResult,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import UiButton from '@/components/ui/UiButton.vue'
import { useAuthStore } from '@/stores/auth'
import { getSetupStatus, createAdmin } from '@/api/setup'

const router = useRouter()
const auth = useAuthStore()

const loading = ref(false)
const stepKey = ref<'admin' | 'done'>('admin')

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

onMounted(async () => {
  try {
    const st = await getSetupStatus()
    if (!st.needs_admin) {
      router.replace('/')
    }
  } catch {
    /* already handled by interceptor */
  }
})

async function handleCreateAdmin() {
  try { await adminFormRef.value?.validate() } catch { return }
  loading.value = true
  try {
    await createAdmin(adminForm.username, adminForm.password)
    await auth.login(adminForm.username, adminForm.password)
    finish()
  } catch {
    /* error message shown by interceptor */
  } finally {
    loading.value = false
  }
}

function finish() {
  stepKey.value = 'done'
  invalidateSetupCache()
  setTimeout(() => router.replace('/'), 1000)
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
  max-width: 480px;
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
.setup-step { margin-top: var(--space-4); }
</style>
