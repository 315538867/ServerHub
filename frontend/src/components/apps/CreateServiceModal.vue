<template>
  <NModal v-model:show="show" preset="card" title="新建服务" style="max-width:480px">
    <NForm label-placement="top" :model="form">
      <NFormItem label="名称" required>
        <NInput v-model:value="form.name" placeholder="例如：my-api" />
      </NFormItem>
      <NFormItem label="类型" required>
        <NSelect v-model:value="form.type" :options="typeOptions" />
      </NFormItem>
      <NFormItem label="工作目录">
        <NInput v-model:value="form.work_dir" placeholder="留空使用默认" />
      </NFormItem>
      <NFormItem label="服务器">
        <span class="cm-text">{{ serverName }}</span>
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <UiButton variant="secondary" size="sm" @click="show = false">取消</UiButton>
        <UiButton variant="primary" size="sm" :loading="submitting" :disabled="!canSubmit" @click="handleSubmit">创建</UiButton>
      </NSpace>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { NModal, NForm, NFormItem, NInput, NSelect, NSpace, useMessage } from 'naive-ui'
import { createService } from '@/api/release'
import { useServerStore } from '@/stores/server'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{
  serverId: number
  applicationId?: number
}>()

const emit = defineEmits<{
  done: []
}>()

const message = useMessage()
const serverStore = useServerStore()
const show = ref(false)
const submitting = ref(false)

const form = reactive({
  name: '',
  type: 'docker' as string,
  work_dir: '',
})

const typeOptions = [
  { label: 'Docker', value: 'docker' },
  { label: 'Docker Compose', value: 'docker-compose' },
  { label: 'Native', value: 'native' },
  { label: 'Static', value: 'static' },
]

const serverName = computed(() => {
  const s = serverStore.servers.find(x => x.id === props.serverId)
  return s ? `${s.name} · ${s.host}` : `#${props.serverId}`
})

const canSubmit = computed(() => !!form.name.trim() && !!form.type)

function open() {
  form.name = ''
  form.type = 'docker'
  form.work_dir = ''
  show.value = true
}

async function handleSubmit() {
  if (!canSubmit.value) return
  submitting.value = true
  try {
    await createService({
      name: form.name.trim(),
      server_id: props.serverId,
      type: form.type as any,
      work_dir: form.work_dir.trim() || undefined,
      application_id: props.applicationId,
    })
    message.success('服务创建成功')
    show.value = false
    emit('done')
  } catch (e: any) {
    message.error(e.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.cm-text {
  font-size: var(--fs-sm);
  color: var(--ui-fg-2);
}
</style>
