<template>
  <div class="page-container srv-page">
    <UiPageHeader title="服务器管理" subtitle="维护所有可被应用调度的物理或虚拟节点">
      <template #actions>
        <UiButton variant="primary" size="sm" @click="openCreate">
          <template #icon><add-icon /></template>
          添加服务器
        </UiButton>
      </template>
    </UiPageHeader>

    <UiSection padding="flush">
      <t-table
        :data="servers"
        :columns="columns"
        :loading="loading"
        row-key="id"
        size="small"
        :empty="emptyEl"
      >
        <template #name="{ row }">
          <div class="srv-name-cell">
            <StatusDot :status="row.status" :size="8" pulse />
            <span class="srv-name">{{ row.name }}</span>
            <span v-if="row.remark" class="srv-remark">{{ row.remark }}</span>
          </div>
        </template>
        <template #host="{ row }">
          <code class="mono-text">{{ row.host }}:{{ row.port }}</code>
        </template>
        <template #auth_type="{ row }">
          <UiBadge :tone="row.auth_type === 'key' ? 'warning' : 'neutral'" variant="soft">
            {{ row.auth_type === 'key' ? '密钥' : '密码' }}
          </UiBadge>
        </template>
        <template #status="{ row }">
          <UiBadge :tone="statusTone(row.status)" variant="soft">{{ statusText(row.status) }}</UiBadge>
        </template>
        <template #last_check_at="{ row }">
          <span class="time-text">{{ row.last_check_at ? dayjs(row.last_check_at).format('MM-DD HH:mm:ss') : '—' }}</span>
        </template>
        <template #operations="{ row }">
          <t-space size="small">
            <t-button size="small" variant="text" theme="primary" :loading="testing === row.id" @click="handleTest(row)">连接测试</t-button>
            <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-popconfirm content="确认删除此服务器?" @confirm="handleDelete(row)">
              <t-button size="small" variant="text" theme="danger">删除</t-button>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>
    </UiSection>

    <!-- 添加/编辑对话框 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="editId ? '编辑服务器' : '添加服务器'"
      width="520px"
      :confirm-btn="{ content: '确定', loading: submitting }"
      @confirm="handleSubmit"
      @close="resetForm"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-width="80px" colon>
        <t-form-item label="名称" name="name">
          <t-input v-model="form.name" placeholder="My Server" />
        </t-form-item>
        <t-form-item label="主机" name="host">
          <t-input v-model="form.host" placeholder="192.168.1.100 或 example.com" />
        </t-form-item>
        <t-form-item label="端口" name="port">
          <t-input-number v-model="form.port" :min="1" :max="65535" class="full-width" />
        </t-form-item>
        <t-form-item label="用户名" name="username">
          <t-input v-model="form.username" placeholder="root" />
        </t-form-item>
        <t-form-item label="认证方式">
          <t-radio-group v-model="form.auth_type">
            <t-radio value="password">密码</t-radio>
            <t-radio value="key">私钥</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="form.auth_type === 'password'" :label="editId ? '新密码' : '密码'" :name="editId ? undefined : 'password'">
          <t-input v-model="form.password" type="password" :placeholder="editId ? '留空则不修改' : ''" />
        </t-form-item>
        <t-form-item v-else :label="editId ? '新私钥' : '私钥'" :name="editId ? undefined : 'private_key'">
          <t-textarea v-model="form.private_key" :autosize="{ minRows: 5 }" :placeholder="editId ? '留空则不修改' : '-----BEGIN RSA PRIVATE KEY-----'" />
        </t-form-item>
        <t-form-item label="备注">
          <t-input v-model="form.remark" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { AddIcon } from 'tdesign-icons-vue-next'
import dayjs from 'dayjs'
import type { Server, ServerForm } from '@/types/api'
import { getServers, createServer, updateServer, deleteServer, testServer } from '@/api/servers'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSection from '@/components/ui/UiSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const servers = ref<Server[]>([])
const loading = ref(false)
const testing = ref<number | null>(null)

const dialogVisible = ref(false)
const editId = ref<number | null>(null)
const submitting = ref(false)
const formRef = ref()

const defaultForm = (): ServerForm => ({
  name: '', host: '', port: 22, username: 'root',
  auth_type: 'password', password: '', private_key: '', remark: '',
})
const form = reactive<ServerForm>(defaultForm())

const rules = {
  name: [{ required: true, message: '请输入名称' }],
  host: [{ required: true, message: '请输入主机地址' }],
  port: [{ required: true, type: 'number' as const, message: '请输入端口' }],
  username: [{ required: true, message: '请输入用户名' }],
  password: [{ required: true, message: '请输入密码' }],
  private_key: [{ required: true, message: '请输入私钥' }],
}

const columns = [
  { colKey: 'name', title: '名称', minWidth: 180 },
  { colKey: 'host', title: '地址', minWidth: 160 },
  { colKey: 'username', title: '用户', width: 90 },
  { colKey: 'auth_type', title: '认证方式', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'last_check_at', title: '最后检测', width: 160 },
  { colKey: 'operations', title: '操作', width: 200, fixed: 'right' as const },
]

const emptyEl = () => h(EmptyBlock, { title: '暂无服务器', description: '点击「添加服务器」开始' })

function statusTone(s: string): any {
  return ({ online: 'success', offline: 'danger' } as Record<string, string>)[s] ?? 'neutral'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s] ?? s
}

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
  formRef.value?.clearValidate()
}

async function handleSubmit() {
  const result = await formRef.value?.validate()
  if (result !== true) return
  submitting.value = true
  try {
    if (editId.value) {
      const payload: Partial<ServerForm> = { ...form }
      if (!payload.password) delete payload.password
      if (!payload.private_key) delete payload.private_key
      await updateServer(editId.value, payload)
      MessagePlugin.success('更新成功')
    } else {
      await createServer(form)
      MessagePlugin.success('添加成功')
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
    if (res.status === 'online') {
      MessagePlugin.success(`${row.name} 连接成功`)
    } else {
      MessagePlugin.error(`${row.name} 连接失败: ${res.error ?? ''}`)
    }
    await loadServers()
  } finally {
    testing.value = null
  }
}

async function handleDelete(row: Server) {
  await deleteServer(row.id)
  MessagePlugin.success('已删除')
  await loadServers()
}

onMounted(loadServers)
</script>

<style scoped>
.srv-page { padding: var(--ui-space-4) var(--ui-space-5); }
.full-width { width: 100%; }

.srv-name-cell {
  display: inline-flex;
  align-items: center;
  gap: var(--ui-space-2);
  min-width: 0;
}
.srv-name {
  font-weight: var(--ui-fw-medium);
  color: var(--ui-fg);
  font-size: var(--ui-fs-sm);
  white-space: nowrap;
}
.srv-remark {
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  margin-left: var(--ui-space-1);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.mono-text {
  font-family: var(--ui-font-mono);
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-2);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border-subtle);
  padding: 1px 6px;
  border-radius: var(--ui-radius-sm);
}
.time-text {
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  font-variant-numeric: tabular-nums;
}

:deep(.t-table) { font-size: var(--ui-fs-sm); }
</style>
