<template>
  <div class="page srv-page">
    <UiPageHeader title="服务器管理" subtitle="维护所有可被应用调度的物理或虚拟节点">
      <template #actions>
        <UiButton variant="primary" size="sm" @click="openCreate">
          <template #icon><Plus :size="14" /></template>
          添加服务器
        </UiButton>
      </template>
    </UiPageHeader>

    <UiCard padding="none">
      <NDataTable
        :columns="columns"
        :data="servers"
        :loading="loading"
        :row-key="(row: Server) => row.id"
        size="small"
        :bordered="false"
      />
    </UiCard>

    <NModal
      v-model:show="dialogVisible"
      preset="card"
      :title="editId ? '编辑服务器' : '添加服务器'"
      style="width: 540px;"
      :bordered="false"
      size="small"
      :on-close="resetForm"
    >
      <NForm ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80" require-mark-placement="right-hanging">
        <NFormItem label="名称" path="name">
          <NInput v-model:value="form.name" placeholder="My Server" />
        </NFormItem>
        <NFormItem label="主机" path="host">
          <NInput v-model:value="form.host" placeholder="192.168.1.100 或 example.com" />
        </NFormItem>
        <NFormItem label="端口" path="port">
          <NInputNumber v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
        </NFormItem>
        <NFormItem label="用户名" path="username">
          <NInput v-model:value="form.username" placeholder="root" />
        </NFormItem>
        <NFormItem label="认证方式">
          <NRadioGroup v-model:value="form.auth_type">
            <NRadio value="password">密码</NRadio>
            <NRadio value="key">私钥</NRadio>
          </NRadioGroup>
        </NFormItem>
        <NFormItem
          v-if="form.auth_type === 'password'"
          :label="editId ? '新密码' : '密码'"
          :path="editId ? undefined : 'password'"
        >
          <NInput
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="editId ? '留空则不修改' : ''"
          />
        </NFormItem>
        <NFormItem v-else :label="editId ? '新私钥' : '私钥'" :path="editId ? undefined : 'private_key'">
          <NInput
            v-model:value="form.private_key"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 10 }"
            :placeholder="editId ? '留空则不修改' : '-----BEGIN RSA PRIVATE KEY-----'"
          />
        </NFormItem>
        <NFormItem label="备注">
          <NInput v-model:value="form.remark" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="dialogVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="submitting" @click="handleSubmit">确定</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber,
  NRadioGroup, NRadio, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { Plus } from 'lucide-vue-next'
import dayjs from 'dayjs'
import type { Server, ServerForm } from '@/types/api'
import { getServers, createServer, updateServer, deleteServer, testServer } from '@/api/servers'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import StatusDot from '@/components/ui/StatusDot.vue'

const message = useMessage()

const servers = ref<Server[]>([])
const loading = ref(false)
const testing = ref<number | null>(null)

const dialogVisible = ref(false)
const editId = ref<number | null>(null)
const submitting = ref(false)
const formRef = ref<FormInst | null>(null)

const defaultForm = (): ServerForm => ({
  name: '', host: '', port: 22, username: 'root',
  auth_type: 'password', password: '', private_key: '', remark: '',
})
const form = reactive<ServerForm>(defaultForm())

const rules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  host: [{ required: true, message: '请输入主机地址', trigger: 'blur' }],
  port: [{ required: true, type: 'number', message: '请输入端口' }],
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  private_key: [{ required: true, message: '请输入私钥', trigger: 'blur' }],
}

function statusTone(s: string): any {
  return ({ online: 'success', offline: 'danger' } as Record<string, string>)[s] ?? 'neutral'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s] ?? s
}

const columns: DataTableColumns<Server> = [
  {
    title: '名称', key: 'name', minWidth: 200,
    render: (row) => h('div', { class: 'srv-name-cell' }, [
      h(StatusDot, { status: row.status, size: 8, pulse: row.status === 'online' }),
      h('span', { class: 'srv-name' }, row.name),
      row.remark ? h('span', { class: 'srv-remark' }, row.remark) : null,
    ]),
  },
  {
    title: '地址', key: 'host', minWidth: 180,
    render: (row) => h('code', { class: 'mono-text' }, `${row.host}:${row.port}`),
  },
  { title: '用户', key: 'username', width: 100 },
  {
    title: '认证', key: 'auth_type', width: 100,
    render: (row) => h(UiBadge, { tone: row.auth_type === 'key' ? 'warning' : 'neutral' },
      () => row.auth_type === 'key' ? '密钥' : '密码'),
  },
  {
    title: '状态', key: 'status', width: 100,
    render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => statusText(row.status)),
  },
  {
    title: '最后检测', key: 'last_check_at', width: 170,
    render: (row) => h('span', { class: 'time-text' },
      row.last_check_at ? dayjs(row.last_check_at).format('MM-DD HH:mm:ss') : '—'),
  },
  {
    title: '操作', key: 'operations', width: 220, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, {
        variant: 'ghost', size: 'sm',
        loading: testing.value === row.id,
        onClick: () => handleTest(row),
      }, () => '连接测试'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEdit(row) }, () => '编辑'),
      h(NPopconfirm, {
        onPositiveClick: () => handleDelete(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' }, () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除此服务器？',
      }),
    ]),
  },
]

async function loadServers() {
  loading.value = true
  try { servers.value = await getServers() } finally { loading.value = false }
}

function openCreate() {
  editId.value = null
  Object.assign(form, defaultForm())
  dialogVisible.value = true
}
function openEdit(row: Server) {
  editId.value = row.id
  Object.assign(form, {
    name: row.name, host: row.host, port: row.port,
    username: row.username, auth_type: row.auth_type,
    password: '', private_key: '', remark: row.remark,
  })
  dialogVisible.value = true
}
function resetForm() {
  formRef.value?.restoreValidation()
}

async function handleSubmit() {
  try { await formRef.value?.validate() } catch { return }
  submitting.value = true
  try {
    if (editId.value) {
      const payload: Partial<ServerForm> = { ...form }
      if (!payload.password) delete payload.password
      if (!payload.private_key) delete payload.private_key
      await updateServer(editId.value, payload)
      message.success('更新成功')
    } else {
      await createServer(form)
      message.success('添加成功')
    }
    dialogVisible.value = false
    await loadServers()
  } finally {
    submitting.value = false
  }
}

async function handleTest(row: Server) {
  testing.value = row.id
  try {
    const res = await testServer(row.id)
    if (res.status === 'online') message.success(`${row.name} 连接成功`)
    else message.error(`${row.name} 连接失败: ${res.error ?? ''}`)
    await loadServers()
  } finally { testing.value = null }
}

async function handleDelete(row: Server) {
  await deleteServer(row.id)
  message.success('已删除')
  await loadServers()
}

onMounted(loadServers)
</script>

<style scoped>
.srv-page {
  padding: var(--space-6);
  display: flex; flex-direction: column;
  gap: var(--space-4);
}

.modal-foot {
  display: flex; justify-content: flex-end; gap: var(--space-2);
}

:deep(.srv-name-cell) {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}
:deep(.srv-name) {
  font-weight: var(--fw-medium);
  color: var(--ui-fg);
  font-size: var(--fs-sm);
  white-space: nowrap;
}
:deep(.srv-remark) {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-left: var(--space-1);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
:deep(.mono-text) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
}
:deep(.time-text) {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  font-variant-numeric: tabular-nums;
}
:deep(.cell-ops) {
  display: inline-flex; gap: var(--space-1); align-items: center;
}
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
