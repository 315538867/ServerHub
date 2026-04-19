<template>
  <div class="servers-page">
    <div class="page-header">
      <h2>服务器管理</h2>
      <el-button type="primary" :icon="Plus" @click="openCreate">添加服务器</el-button>
    </div>

    <el-table :data="servers" v-loading="loading" stripe border>
      <el-table-column label="名称" prop="name" min-width="120" />
      <el-table-column label="主机" min-width="160">
        <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
      </el-table-column>
      <el-table-column label="用户" prop="username" width="100" />
      <el-table-column label="认证" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="row.auth_type === 'key' ? 'warning' : 'info'">
            {{ row.auth_type === 'key' ? '密钥' : '密码' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后检测" width="160">
        <template #default="{ row }">
          {{ row.last_check_at ? dayjs(row.last_check_at).format('MM-DD HH:mm:ss') : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="备注" prop="remark" min-width="120" show-overflow-tooltip />
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link size="small" :loading="testing === row.id" @click="handleTest(row)">测试</el-button>
          <el-button link size="small" @click="openEdit(row)">编辑</el-button>
          <el-popconfirm title="确认删除此服务器?" @confirm="handleDelete(row)">
            <template #reference>
              <el-button link size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create / Edit dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editId ? '编辑服务器' : '添加服务器'"
      width="520px"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="My Server" />
        </el-form-item>
        <el-form-item label="主机" prop="host">
          <el-input v-model="form.host" placeholder="192.168.1.100 或 example.com" />
        </el-form-item>
        <el-form-item label="端口" prop="port">
          <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="root" />
        </el-form-item>
        <el-form-item label="认证方式">
          <el-radio-group v-model="form.auth_type">
            <el-radio value="password">密码</el-radio>
            <el-radio value="key">私钥</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'password'" :label="editId ? '新密码' : '密码'" :prop="editId ? undefined : 'password'">
          <el-input v-model="form.password" type="password" show-password :placeholder="editId ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item v-else :label="editId ? '新私钥' : '私钥'" :prop="editId ? undefined : 'private_key'">
          <el-input v-model="form.private_key" type="textarea" :rows="5" :placeholder="editId ? '留空则不修改' : '-----BEGIN RSA PRIVATE KEY-----'" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import dayjs from 'dayjs'
import type { Server, ServerForm } from '@/types/api'
import { getServers, createServer, updateServer, deleteServer, testServer } from '@/api/servers'

const servers = ref<Server[]>([])
const loading = ref(false)
const testing = ref<number | null>(null)

const dialogVisible = ref(false)
const editId = ref<number | null>(null)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const defaultForm = (): ServerForm => ({
  name: '', host: '', port: 22, username: 'root',
  auth_type: 'password', password: '', private_key: '', remark: '',
})
const form = reactive<ServerForm>(defaultForm())

const rules: FormRules = {
  name: [{ required: true, message: '请输入名称' }],
  host: [{ required: true, message: '请输入主机地址' }],
  port: [{ required: true, type: 'number', message: '请输入端口' }],
  username: [{ required: true, message: '请输入用户名' }],
  password: [{ required: true, message: '请输入密码' }],
  private_key: [{ required: true, message: '请输入私钥' }],
}

function statusType(s: Server['status']) {
  return { online: 'success', offline: 'danger', unknown: 'info' }[s]
}
function statusText(s: Server['status']) {
  return { online: '在线', offline: '离线', unknown: '未知' }[s]
}

async function loadServers() {
  loading.value = true
  try {
    servers.value = await getServers()
  } finally {
    loading.value = false
  }
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
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (editId.value) {
      const payload: Partial<ServerForm> = { ...form }
      if (!payload.password) delete payload.password
      if (!payload.private_key) delete payload.private_key
      await updateServer(editId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await createServer(form)
      ElMessage.success('添加成功')
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
      ElMessage.success(`${row.name} 连接成功`)
    } else {
      ElMessage.error(`${row.name} 连接失败: ${res.error ?? ''}`)
    }
    await loadServers()
  } finally {
    testing.value = null
  }
}

async function handleDelete(row: Server) {
  await deleteServer(row.id)
  ElMessage.success('已删除')
  await loadServers()
}

onMounted(loadServers)
</script>

<style scoped>
.servers-page { padding: 20px; }
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.page-header h2 { margin: 0; font-size: 18px; }
</style>
