<template>
  <div class="login-page">
    <el-card class="login-card">
      <div class="login-header">
        <h2>ServerHub</h2>
        <p>SSH-native · Zero-agent · Desktop-first</p>
      </div>

      <!-- Step 1: username/password -->
      <el-form
        v-if="!totpStep"
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleLogin"
      >
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="请输入用户名"
            prefix-icon="User"
            autocomplete="username"
          />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            prefix-icon="Lock"
            show-password
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-button
          type="primary"
          class="login-btn"
          :loading="loading"
          @click="handleLogin"
        >
          登录
        </el-button>
      </el-form>

      <!-- Step 2: TOTP code -->
      <div v-else>
        <p class="totp-hint">请输入两步验证码（Authenticator App）</p>
        <el-input
          v-model="totpCode"
          placeholder="6 位验证码"
          maxlength="6"
          size="large"
          autofocus
          @keyup.enter="handleTotpLogin"
        />
        <el-button
          type="primary"
          class="login-btn"
          style="margin-top:16px"
          :loading="loading"
          @click="handleTotpLogin"
        >
          验证
        </el-button>
        <el-button class="login-btn" style="margin-top:8px" @click="totpStep = false">
          返回
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { totpLogin } from '@/api/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref<FormInstance>()
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
  await formRef.value?.validate()
  loading.value = true
  try {
    const result = await authStore.login(form.username, form.password)
    if (result && 'require_totp' in result && result.require_totp) {
      tmpToken.value = result.tmp_token
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
    ElMessage.error('验证码错误')
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
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}
.login-card {
  width: 400px;
}
.login-header {
  text-align: center;
  margin-bottom: 24px;
}
.login-header h2 {
  font-size: 28px;
  font-weight: 700;
  color: #409eff;
  margin-bottom: 4px;
}
.login-header p {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.login-btn {
  width: 100%;
}
.totp-hint {
  text-align: center;
  color: var(--el-text-color-secondary);
  margin-bottom: 16px;
}
</style>
