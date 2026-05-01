<template>
  <NModal v-model:show="show" preset="card" title="挂载已有服务" style="max-width:640px">
    <NDataTable
      :columns="columns"
      :data="floatingServices"
      :loading="loading"
      :bordered="false"
      size="small"
    />
    <div v-if="!loading && floatingServices.length === 0" class="am-empty">
      没有可挂载的服务。当前服务器上所有服务都已绑定到应用。
    </div>
    <template #footer>
      <UiButton variant="secondary" size="sm" @click="show = false">关闭</UiButton>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import { ref, h } from 'vue'
import { NModal, NDataTable, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { listServices } from '@/api/release'
import { attachServiceToApp } from '@/api/application'
import type { Deploy } from '@/types/api'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{
  appId: number
  serverId: number
}>()

const emit = defineEmits<{
  done: []
}>()

const message = useMessage()
const show = ref(false)
const loading = ref(false)
const floatingServices = ref<Deploy[]>([])

const columns: DataTableColumns<Deploy> = [
  { title: '名称', key: 'name', width: 160 },
  { title: '类型', key: 'type', width: 120 },
  {
    title: '目录', key: 'work_dir', minWidth: 200,
    render: (row) => h('code', { class: 'am-code' }, row.work_dir || '—'),
  },
  {
    title: '操作', key: 'actions', width: 80,
    render: (row) => h(UiButton, {
      size: 'sm', variant: 'primary',
      onClick: () => handleAttach(row),
    }, () => '挂载'),
  },
]

async function open() {
  show.value = true
  loading.value = true
  try {
    const all = await listServices(props.serverId)
    floatingServices.value = all.filter(s => !s.application_id)
  } catch (e: any) {
    message.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function handleAttach(row: Deploy) {
  try {
    await attachServiceToApp(props.appId, row.id)
    message.success(`已挂载「${row.name}」`)
    floatingServices.value = floatingServices.value.filter(s => s.id !== row.id)
    emit('done')
  } catch (e: any) {
    message.error(e.message || '挂载失败')
  }
}

defineExpose({ open })
</script>

<style scoped>
.am-empty {
  padding: var(--space-4);
  text-align: center;
  color: var(--ui-fg-3);
  font-size: var(--fs-sm);
}
.am-code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
}
</style>
