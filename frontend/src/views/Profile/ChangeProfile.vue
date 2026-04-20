<template>
  <t-dialog
    v-model:visible="visible"
    header="账号设置"
    width="480px"
    :confirm-btn="{ content: '保存', loading: submitting }"
    @confirm="handleSubmit"
    @close="resetForm"
  >
    <t-form ref="formRef" :data="form" :rules="rules" label-width="100px" colon>
      <t-form-item label="旧密码" name="oldPassword">
        <t-input v-model="form.oldPassword" type="password" placeholder="请输入当前密码" />
      </t-form-item>
      <t-form-item label="新用户名" name="newUsername">
        <t-input v-model="form.newUsername" placeholder="不修改则留空" />
      </t-form-item>
      <t-form-item label="新密码" name="newPassword">
        <t-input v-model="form.newPassword" type="password" placeholder="不修改则留空" />
      </t-form-item>
      <t-form-item label="确认新密码" name="confirmPassword">
        <t-input v-model="form.confirmPassword" type="password" placeholder="再次输入新密码" />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { updateProfile } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const visible = defineModel<boolean>('visible', { default: false })

const authStore = useAuthStore()
const submitting = ref(false)
const formRef = ref()

const form = reactive({
  oldPassword: '',
  newUsername: '',
  newPassword: '',
  confirmPassword: '',
})

const rules = {
  oldPassword: [{ required: true, message: '旧密码不能为空', trigger: 'blur' }],
  confirmPassword: [
    {
      validator: (val: string) => {
        if (form.newPassword && val !== form.newPassword) {
          return { result: false, message: '两次密码不一致' }
        }
        return { result: true }
      },
      trigger: 'blur',
    },
  ],
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return

  if (!form.newUsername && !form.newPassword) {
    MessagePlugin.warning('请至少修改用户名或密码')
    return
  }

  submitting.value = true
  try {
    const user = await updateProfile(
      form.oldPassword,
      form.newUsername || undefined,
      form.newPassword || undefined,
    )
    authStore.user = user
    MessagePlugin.success('账号信息已更新')
    visible.value = false
    resetForm()
  } catch {
    // request interceptor handles error toast
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  form.oldPassword = ''
  form.newUsername = ''
  form.newPassword = ''
  form.confirmPassword = ''
  formRef.value?.clearValidate()
}
</script>
