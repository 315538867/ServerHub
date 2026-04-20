<template>
  <div class="page-container">
    <!-- 页面标题 -->
    <div class="section-block">
      <div class="section-title">
        <span>服务器管理</span>
        <t-button theme="primary" size="small" @click="openCreate">
          <template #icon><add-icon /></template>
          添加服务器
        </t-button>
      </div>
    </div>

    <!-- 服务器列表 -->
    <div class="section-block">
      <t-table
        :data="servers"
        :columns="columns"
        :loading="loading"
        row-key="id"
        bordered
        size="small"
        :header-affixed-top="{ offsetTop: 0 }"
        empty="暂无服务器，点击「添加服务器」开始"
      >
        <template #name="{ row }">
          <div class="server-name-cell">
            <span class="status-dot" :class="row.status" />
            <span class="server-name">{{ row.name }}</span>
            <span v-if="row.remark" class="server-remark">{{ row.remark }}</span>
          </div>
        </template>
        <template #host="{ row }">
          <span class="mono-text">{{ row.host }}:{{ row.port }}</span>
        </template>
        <template #auth_type="{ row }">
          <t-tag :theme="row.auth_type === 'key' ? 'warning' : 'default'" variant="light" size="small">
            {{ row.auth_type === 'key' ? '密钥' : '密码' }}
          </t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small">{{ statusText(row.status) }}</t-tag>
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
    </div>

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
import { ref, reactive, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { AddIcon } from 'tdesign-icons-vue-next'
import dayjs from 'dayjs'
import type { Server, ServerForm } from '@/types/api'
import { getServers, createServer, updateServer, deleteServer, testServer } from '@/api/servers'

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

function statusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', unknown: 'default' } as Record<string, string>)[s] ?? 'default'
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
.full-width { width: 100%; }

.server-name-cell {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
}
.server-name {
  font-weight: 500;
  color: var(--sh-text-primary);
}
.server-remark {
  font-size: 12px;
  color: var(--sh-text-secondary);
}
.mono-text {
  font-family: 'JetBrains Mono', 'Cascadia Code', Menlo, monospace;
  font-size: 12.5px;
}
.time-text {
  font-size: 12.5px;
  color: var(--sh-text-secondary);
}

:deep(.t-table) {
  font-size: 13px;
}
</style>
