<template>
  <div class="login-page">
    <t-card class="login-card">
      <div class="login-header">
        <h2>ServerHub</h2>
        <p>SSH-native · Zero-agent · Desktop-first</p>
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
          <t-input v-model="form.username" placeholder="请输入用户名" autocomplete="username" />
        </t-form-item>
        <t-form-item label="密码" name="password">
          <t-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            autocomplete="current-password"
            @keydown.enter="handleLogin"
          />
        </t-form-item>
        <t-button
          theme="primary"
          block
          :loading="loading"
          type="submit"
          style="margin-top: 8px"
        >
          登录
        </t-button>
      </t-form>

      <div v-else>
        <p class="totp-hint">请输入两步验证码（Authenticator App）</p>
        <t-input
          v-model="totpCode"
          placeholder="6 位验证码"
          :maxlength="6"
          size="large"
          autofocus
          @keydown.enter="handleTotpLogin"
        />
        <t-button theme="primary" block :loading="loading" style="margin-top:16px" @click="handleTotpLogin">
          验证
        </t-button>
        <t-button variant="outline" block style="margin-top:8px" @click="totpStep = false">
          返回
        </t-button>
      </div>
    </t-card>
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
  password: [{ required: true, message: '请输入密码', trigger: 'blur' as const }],
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
    const redirect = route.query.redirect as string | undefined
    router.push(redirect ?? '/')
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
    const redirect = route.query.redirect as string | undefined
    router.push(redirect ?? '/')
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
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0d1b2a 0%, #1a2e44 50%, #0f3460 100%);
}
.login-card { width: 400px; }
.login-header { text-align: center; margin-bottom: 24px; }
.login-header h2 {
  font-size: 28px;
  font-weight: 700;
  color: #0052d9;
  margin-bottom: 4px;
}
.login-header p { color: var(--td-text-color-secondary); font-size: 13px; }
.totp-hint { text-align: center; color: var(--td-text-color-secondary); margin-bottom: 16px; }
</style>
