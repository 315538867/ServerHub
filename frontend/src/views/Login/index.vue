<template>
  <div class="login-page">
    <!-- 左侧装饰区 -->
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
      <!-- 几何装饰 -->
      <div class="login-decor login-decor-1" />
      <div class="login-decor login-decor-2" />
      <div class="login-decor login-decor-3" />
    </div>

    <!-- 右侧登录区 -->
    <div class="login-right">
      <div class="login-box">
        <div class="login-box-header">
          <h3 class="login-box-title">{{ totpStep ? '两步验证' : '账号登录' }}</h3>
          <p class="login-box-sub">{{ totpStep ? '请输入 Authenticator App 中的验证码' : '使用您的管理员账号登录' }}</p>
        </div>

        <t-form
          v-if="!totpStep"
          ref="formRef"
          :data="form"
          :rules="rules"
          label-align="top"
          @submit="handleLogin"
        >
          <t-form-item label="用户名" name="username">
            <t-input
              v-model="form.username"
              placeholder="请输入用户名"
              autocomplete="username"
              size="large"
            />
          </t-form-item>
          <t-form-item label="密码" name="password">
            <t-input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              autocomplete="current-password"
              size="large"
              @keydown.enter="handleLogin"
            />
          </t-form-item>
          <t-button
            theme="primary"
            block
            size="large"
            :loading="loading"
            type="submit"
            class="login-submit-btn"
          >
            登 录
          </t-button>
        </t-form>

        <div v-else class="totp-area">
          <div class="totp-code-wrap">
            <t-input
              v-model="totpCode"
              placeholder="请输入 6 位验证码"
              :maxlength="6"
              size="large"
              autofocus
              @keydown.enter="handleTotpLogin"
            />
          </div>
          <t-button
            theme="primary"
            block
            size="large"
            :loading="loading"
            class="login-submit-btn"
            @click="handleTotpLogin"
          >
            验 证
          </t-button>
          <t-button
            variant="text"
            block
            style="margin-top: var(--ui-space-2);"
            @click="totpStep = false"
          >
            返回登录
          </t-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAuthStore } from '@/stores/auth'
import { totpLogin } from '@/api/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref()
const loading = ref(false)

const form = reactive({ username: '', password: '' })
const totpStep = ref(false)
const totpCode = ref('')
const tmpToken = ref('')

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' as const }],
  password: [{ required: true, message: '请输入密码',   trigger: 'blur' as const }],
}

async function handleLogin() {
  const result = await formRef.value?.validate()
  if (result !== true) return
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
    MessagePlugin.error('验证码错误')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  background: var(--ui-bg-canvas);
}

/* ── 左侧：品牌区 ── */
.login-left {
  flex: 1;
  background:
    radial-gradient(80% 60% at 0% 0%, rgba(94, 106, 210, .35) 0%, transparent 60%),
    radial-gradient(70% 70% at 100% 100%, rgba(70, 177, 201, .22) 0%, transparent 55%),
    linear-gradient(135deg, #1a1a2e 0%, #16213e 55%, #0f1628 100%);
  background-size: 200% 200%;
  animation: ui-grad-drift 16s ease-in-out infinite;
  display: flex;
  flex-direction: column;
  padding: var(--ui-space-8);
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
  display: flex; align-items: center; gap: var(--ui-space-3);
  margin-bottom: auto;
  position: relative; z-index: 1;
  animation: ui-slide-right var(--ui-dur-slow) var(--ui-ease-standard);
}
.login-brand-icon {
  width: 38px; height: 38px;
  background: var(--ui-brand-grad);
  background-size: 200% 200%;
  animation: ui-grad-drift 10s ease-in-out infinite;
  border-radius: 10px;
  display: grid; place-items: center;
  font-size: 18px; font-weight: 800; color: #fff;
  box-shadow: 0 6px 20px rgba(94,106,210,.5), inset 0 1px 0 rgba(255,255,255,.2);
}
.login-brand-name {
  font-size: 20px; font-weight: 700; color: #fff;
  letter-spacing: .3px;
}

.login-tagline {
  margin-bottom: var(--ui-space-7);
  position: relative; z-index: 1;
  animation: ui-slide-up var(--ui-dur-slow) var(--ui-ease-standard) .1s both;
}
.login-tagline h2 {
  font-size: 34px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 var(--ui-space-3);
  line-height: 1.2;
  letter-spacing: var(--ui-tracking-tight);
  background: linear-gradient(135deg, #fff 0%, #c7cdf3 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.login-tagline p {
  font-size: 14px; color: rgba(255,255,255,.55); margin: 0;
}

.login-features {
  display: flex; flex-direction: column; gap: var(--ui-space-3);
  position: relative; z-index: 1;
}
.login-feature-item {
  display: flex; align-items: center; gap: var(--ui-space-3);
  font-size: 13px; color: rgba(255,255,255,.6);
  animation: ui-slide-up var(--ui-dur-slow) var(--ui-ease-standard) both;
}
.login-feature-item:nth-child(1) { animation-delay: .25s; }
.login-feature-item:nth-child(2) { animation-delay: .35s; }
.login-feature-item:nth-child(3) { animation-delay: .45s; }
.login-feature-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--ui-brand);
  box-shadow: 0 0 0 3px rgba(94,106,210,.25);
  flex-shrink: 0;
  animation: ui-status-pulse 3s var(--ui-ease-standard) infinite;
}

/* 浮动装饰 */
.login-decor {
  position: absolute;
  border-radius: 50%;
  opacity: .12;
  background: radial-gradient(circle, rgba(94,106,210,.8), transparent 70%);
  animation: ui-float 8s ease-in-out infinite;
}
.login-decor-1 { width: 420px; height: 420px; bottom: -120px; right: -80px; }
.login-decor-2 { width: 200px; height: 200px; bottom: 140px; right: 180px; animation-delay: -3s; opacity: .08; }
.login-decor-3 { width: 120px; height: 120px; top: 160px; right: 80px; opacity: .06; animation-delay: -5s; }

@keyframes ui-float {
  0%, 100% { transform: translateY(0); }
  50%      { transform: translateY(-16px); }
}

/* ── 右侧：登录区 ── */
.login-right {
  width: 480px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ui-bg-subtle);
  padding: var(--ui-space-8);
}

.login-box {
  width: 100%;
  max-width: 400px;
  background: var(--ui-bg-surface);
  border-radius: var(--ui-radius-xl);
  border: 1px solid var(--ui-border);
  box-shadow: var(--ui-shadow-lg);
  padding: var(--ui-space-7);
  animation: ui-scale-in var(--ui-dur-base) var(--ui-ease-spring);
}

.login-box-header { margin-bottom: var(--ui-space-6); }
.login-box-title {
  margin: 0 0 var(--ui-space-2);
  font-size: var(--ui-fs-3xl);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  letter-spacing: var(--ui-tracking-tight);
}
.login-box-sub {
  margin: 0;
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
}

.login-submit-btn {
  margin-top: var(--ui-space-5);
  letter-spacing: .1em;
  font-weight: var(--ui-fw-semibold);
}

.totp-area { display: flex; flex-direction: column; }
.totp-code-wrap { margin-bottom: var(--ui-space-2); }
</style>
