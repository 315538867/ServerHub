<template>
  <NModal
    v-model:show="visible"
    preset="card"
    title="账号设置"
    style="width: 480px"
    :bordered="false"
    @after-leave="resetForm"
  >
    <NForm ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="100">
      <NFormItem label="旧密码" path="oldPassword">
        <NInput v-model:value="form.oldPassword" type="password" show-password-on="click" placeholder="请输入当前密码" />
      </NFormItem>
      <NFormItem label="新用户名" path="newUsername">
        <NInput v-model:value="form.newUsername" placeholder="不修改则留空" />
      </NFormItem>
      <NFormItem label="新密码" path="newPassword">
        <NInput v-model:value="form.newPassword" type="password" show-password-on="click" placeholder="不修改则留空" />
      </NFormItem>
      <NFormItem label="确认新密码" path="confirmPassword">
        <NInput v-model:value="form.confirmPassword" type="password" show-password-on="click" placeholder="再次输入新密码" />
      </NFormItem>
    </NForm>
    <template #footer>
      <div class="modal-foot">
        <UiButton variant="secondary" size="sm" @click="visible = false">取消</UiButton>
        <UiButton variant="primary" size="sm" :loading="submitting" @click="handleSubmit">保存</UiButton>
      </div>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { NModal, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { updateProfile } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import UiButton from '@/components/ui/UiButton.vue'

const visible = defineModel<boolean>('visible', { default: false })

const authStore = useAuthStore()
const message = useMessage()
const submitting = ref(false)
const formRef = ref<FormInst | null>(null)

const form = reactive({
  oldPassword: '',
  newUsername: '',
  newPassword: '',
  confirmPassword: '',
})

const rules: FormRules = {
  oldPassword: [{ required: true, message: '旧密码不能为空', trigger: 'blur' }],
  confirmPassword: [
    {
      validator: (_rule, val: string) => {
        if (form.newPassword && val !== form.newPassword) {
          return new Error('两次密码不一致')
        }
        return true
      },
      trigger: 'blur',
    },
  ],
}

async function handleSubmit() {
  try { await formRef.value?.validate() } catch { return }

  if (!form.newUsername && !form.newPassword) {
    message.warning('请至少修改用户名或密码')
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
    message.success('账号信息已更新')
    visible.value = false
    resetForm()
  } catch {
    /* request interceptor handles error toast */
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  form.oldPassword = ''
  form.newUsername = ''
  form.newPassword = ''
  form.confirmPassword = ''
  formRef.value?.restoreValidation()
}
</script>

<style scoped>
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }
</style>
