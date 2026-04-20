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
            style="margin-top:8px;color:#666"
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
}

/* ── 左侧 ── */
.login-left {
  flex: 1;
  background: linear-gradient(145deg, #001529 0%, #002a5c 60%, #003380 100%);
  display: flex;
  flex-direction: column;
  padding: 48px 60px;
  position: relative;
  overflow: hidden;
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: auto;
}
.login-brand-icon {
  width: 36px;
  height: 36px;
  background: #0052d9;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 800;
  color: #fff;
}
.login-brand-name {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
}

.login-tagline {
  margin-bottom: 40px;
}
.login-tagline h2 {
  font-size: 32px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 12px;
  line-height: 1.3;
}
.login-tagline p {
  font-size: 15px;
  color: rgba(255,255,255,.6);
  margin: 0;
}

.login-features { display: flex; flex-direction: column; gap: 14px; }
.login-feature-item {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13.5px;
  color: rgba(255,255,255,.55);
}
.login-feature-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #0052d9;
  flex-shrink: 0;
}

/* 几何装饰 */
.login-decor {
  position: absolute;
  border-radius: 50%;
  opacity: .06;
  background: #fff;
}
.login-decor-1 { width: 400px; height: 400px; bottom: -120px; right: -80px; }
.login-decor-2 { width: 200px; height: 200px; bottom: 100px;  right: 160px; }
.login-decor-3 { width: 120px; height: 120px; top: 200px;    right: 60px; opacity: .04; }

/* ── 右侧 ── */
.login-right {
  width: 480px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f2f3f5;
  padding: 40px;
}

.login-box {
  width: 100%;
  max-width: 380px;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e7e7e7;
  box-shadow: 0 2px 12px rgba(0,0,0,.08);
  padding: 36px 32px 32px;
}

.login-box-header { margin-bottom: 28px; }
.login-box-title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 700;
  color: #0d0d0d;
}
.login-box-sub {
  margin: 0;
  font-size: 13px;
  color: #666;
}

.login-submit-btn { margin-top: 20px; letter-spacing: .05em; }

.totp-area { display: flex; flex-direction: column; }
.totp-code-wrap { margin-bottom: 4px; }
</style>
