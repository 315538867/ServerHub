<template>
  <div class="login-page">
    <div class="login-left">
      <div class="login-brand">
        <div class="login-brand-icon">S</div>
        <span class="login-brand-name">ServerHub</span>
      </div>
      <div class="login-tagline">
        <h2>服务器托管控制台</h2>
        <p>统一管理您的服务器、应用与部署流程</p>
      </div>
      <div class="login-features">
        <div class="login-feature-item">
          <div class="login-feature-dot" />
          SSH-native · 原生 SSH 连接，无 Agent 依赖
        </div>
        <div class="login-feature-item">
          <div class="login-feature-dot" />
          Zero-config · 开箱即用，配置极简
        </div>
        <div class="login-feature-item">
          <div class="login-feature-dot" />
          Desktop-first · 专为桌面端设计
        </div>
      </div>
      <div class="login-decor login-decor-1" />
      <div class="login-decor login-decor-2" />
      <div class="login-decor login-decor-3" />
    </div>

    <div class="login-right">
      <div class="login-box">
        <div class="login-box-header">
          <h3 class="login-box-title">{{ totpStep ? '两步验证' : '账号登录' }}</h3>
          <p class="login-box-sub">{{ totpStep ? '请输入 Authenticator App 中的验证码' : '使用您的管理员账号登录' }}</p>
        </div>

        <NForm
          v-if="!totpStep"
          ref="formRef"
          :model="form"
          :rules="rules"
          label-placement="top"
          @submit.prevent="handleLogin"
        >
          <NFormItem label="用户名" path="username">
            <NInput v-model:value="form.username" placeholder="请输入用户名" autocomplete="username" size="large" />
          </NFormItem>
          <NFormItem label="密码" path="password">
            <NInput
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="请输入密码"
              autocomplete="current-password"
              size="large"
              @keydown.enter="handleLogin"
            />
          </NFormItem>
          <UiButton variant="primary" size="lg" :loading="loading" block native-type="submit" class="login-submit-btn" @click="handleLogin">
            登 录
          </UiButton>
        </NForm>

        <div v-else class="totp-area">
          <NInput
            v-model:value="totpCode"
            placeholder="请输入 6 位验证码"
            :maxlength="6"
            size="large"
            autofocus
            @keydown.enter="handleTotpLogin"
          />
          <UiButton variant="primary" size="lg" :loading="loading" block class="login-submit-btn" @click="handleTotpLogin">
            验 证
          </UiButton>
          <UiButton variant="ghost" size="md" block class="back-btn" @click="totpStep = false">
            返回登录
          </UiButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { totpLogin } from '@/api/auth'
import UiButton from '@/components/ui/UiButton.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const message = useMessage()
const formRef = ref<FormInst | null>(null)
const loading = ref(false)

const form = reactive({ username: '', password: '' })
const totpStep = ref(false)
const totpCode = ref('')
const tmpToken = ref('')

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  try { await formRef.value?.validate() } catch { return }
  loading.value = true
  try {
    const res = await authStore.login(form.username, form.password)
    if (res && 'require_totp' in res && res.require_totp) {
      tmpToken.value = res.tmp_token
      totpStep.value = true
      return
    }
    router.push((route.query.redirect as string) ?? '/')
  } finally {
    loading.value = false
  }
}

async function handleTotpLogin() {
  if (!totpCode.value) return
  loading.value = true
  try {
    const data = await totpLogin(tmpToken.value, totpCode.value)
    authStore.setTokenAndUser(data.token, data.user)
    router.push((route.query.redirect as string) ?? '/')
  } catch {
    message.error('验证码错误')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  background: var(--ui-bg-1);
}

.login-left {
  flex: 1;
  background:
    radial-gradient(80% 60% at 0% 0%, rgba(62, 207, 142, .25) 0%, transparent 60%),
    radial-gradient(70% 70% at 100% 100%, rgba(70, 177, 201, .18) 0%, transparent 55%),
    linear-gradient(135deg, #0F1316 0%, #14181C 55%, #0A0C0F 100%);
  display: flex;
  flex-direction: column;
  padding: var(--space-8);
  position: relative;
  overflow: hidden;
  color: #fff;
}
.login-left::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255,255,255,.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,.04) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(70% 60% at 30% 40%, #000 20%, transparent 75%);
  pointer-events: none;
}

.login-brand {
  display: flex; align-items: center; gap: var(--space-3);
  margin-bottom: auto;
  position: relative; z-index: 1;
}
.login-brand-icon {
  width: 38px; height: 38px;
  background: var(--ui-brand);
  border-radius: 10px;
  display: grid; place-items: center;
  font-size: 18px; font-weight: 800; color: #0A0A0A;
  box-shadow: 0 6px 20px rgba(62,207,142,.4), inset 0 1px 0 rgba(255,255,255,.2);
}
.login-brand-name {
  font-size: 20px; font-weight: 700; color: #fff;
  letter-spacing: .3px;
}

.login-tagline {
  margin-bottom: var(--space-7);
  position: relative; z-index: 1;
}
.login-tagline h2 {
  font-size: 34px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 var(--space-3);
  line-height: 1.2;
  letter-spacing: -0.02em;
}
.login-tagline p {
  font-size: 14px; color: rgba(255,255,255,.55); margin: 0;
}

.login-features {
  display: flex; flex-direction: column; gap: var(--space-3);
  position: relative; z-index: 1;
}
.login-feature-item {
  display: flex; align-items: center; gap: var(--space-3);
  font-size: 13px; color: rgba(255,255,255,.6);
}
.login-feature-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--ui-brand);
  box-shadow: 0 0 0 3px rgba(62,207,142,.18);
  flex-shrink: 0;
}

.login-decor {
  position: absolute;
  border-radius: 50%;
  opacity: .12;
  background: radial-gradient(circle, rgba(62,207,142,.7), transparent 70%);
}
.login-decor-1 { width: 420px; height: 420px; bottom: -120px; right: -80px; }
.login-decor-2 { width: 200px; height: 200px; bottom: 140px; right: 180px; opacity: .08; }
.login-decor-3 { width: 120px; height: 120px; top: 160px; right: 80px; opacity: .06; }

.login-right {
  width: 480px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ui-bg-1);
  padding: var(--space-8);
}

.login-box {
  width: 100%;
  max-width: 400px;
  background: transparent;
  border-radius: var(--radius-md);
  padding: var(--space-2) 0;
}

.login-box-header { margin-bottom: var(--space-6); }
.login-box-title {
  margin: 0 0 var(--space-2);
  font-size: var(--fs-2xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.02em;
}
.login-box-sub {
  margin: 0;
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
}

.login-submit-btn {
  margin-top: var(--space-5);
  letter-spacing: .1em;
  font-weight: var(--fw-semibold);
}

.totp-area { display: flex; flex-direction: column; }
.back-btn { margin-top: var(--space-2); }
</style>
