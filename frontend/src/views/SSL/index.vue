<template>
  <div class="ssl-page">
    <div class="page-toolbar">
      <el-select v-model="selectedServerId" placeholder="选择服务器" style="width:220px" @change="loadCerts">
        <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
      </el-select>
      <el-button type="primary" @click="openRequest">申请证书</el-button>
      <el-button @click="openUpload">上传证书</el-button>
      <el-button @click="doScan" :loading="scanning">扫描证书</el-button>
      <el-button :icon="Refresh" :loading="loading" @click="loadCerts">刷新</el-button>
    </div>

    <el-table :data="certs" v-loading="loading" style="width:100%">
      <el-table-column label="域名" prop="domain" min-width="180" show-overflow-tooltip />
      <el-table-column label="签发方" prop="issuer" width="140" />
      <el-table-column label="证书路径" prop="cert_path" min-width="220" show-overflow-tooltip />
      <el-table-column label="到期时间" prop="expires_at" width="120" />
      <el-table-column label="剩余天数" width="110">
        <template #default="{ row }">
          <el-tag :type="tagType(row.days_left)" size="small">{{ row.days_left }} 天</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="自动续期" width="90">
        <template #default="{ row }">
          <el-tag :type="row.auto_renew ? 'success' : 'info'" size="small">
            {{ row.auto_renew ? '是' : '否' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openRenew(row)">续期</el-button>
          <el-popconfirm :title="`确认删除 ${row.domain} 的证书记录？`" @confirm="deleteCertItem(row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- ── Request cert dialog ─────────────────────────────────── -->
    <el-dialog v-model="requestVisible" title="申请 Let's Encrypt 证书" width="520px" :close-on-click-modal="false">
      <el-form :model="requestForm" label-width="80px">
        <el-form-item label="域名">
          <el-input v-model="requestForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="Webroot">
          <el-input v-model="requestForm.webroot" placeholder="/var/www/html" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="requestForm.email" placeholder="admin@example.com" />
        </el-form-item>
        <el-form-item label="进度输出">
          <pre class="ws-output" ref="requestOutputEl">{{ requestOutput }}</pre>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="requestVisible = false" :disabled="requesting">关闭</el-button>
        <el-button type="primary" :loading="requesting" @click="startRequest">申请</el-button>
      </template>
    </el-dialog>

    <!-- ── Upload cert dialog ──────────────────────────────────── -->
    <el-dialog v-model="uploadVisible" title="上传证书" width="600px">
      <el-form :model="uploadForm" label-width="90px">
        <el-form-item label="域名">
          <el-input v-model="uploadForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="证书路径">
          <el-input v-model="uploadForm.cert_path" placeholder="/etc/ssl/certs/example.com.pem" />
        </el-form-item>
        <el-form-item label="私钥路径">
          <el-input v-model="uploadForm.key_path" placeholder="/etc/ssl/private/example.com.key" />
        </el-form-item>
        <el-form-item label="证书内容">
          <el-input v-model="uploadForm.cert" type="textarea" :rows="6" placeholder="-----BEGIN CERTIFICATE-----" />
        </el-form-item>
        <el-form-item label="私钥内容">
          <el-input v-model="uploadForm.key" type="textarea" :rows="6" placeholder="-----BEGIN PRIVATE KEY-----" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="confirmUpload">上传</el-button>
      </template>
    </el-dialog>

    <!-- ── Renew cert dialog ───────────────────────────────────── -->
    <el-dialog v-model="renewVisible" :title="`续期 — ${renewDomain}`" width="560px" :close-on-click-modal="false">
      <pre class="ws-output">{{ renewOutput }}</pre>
      <template #footer>
        <el-button @click="renewVisible = false" :disabled="renewing">关闭</el-button>
        <el-button type="primary" :loading="renewing" @click="startRenew">续期</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  listCerts, deleteCert, uploadCert, scanCerts,
  requestCertWsUrl, renewCertWsUrl,
} from '@/api/ssl'
import type { SSLCert } from '@/api/ssl'
import type { Server } from '@/types/api'

const auth = useAuthStore()
const servers = ref<Server[]>([])
const selectedServerId = ref<number | null>(null)
const certs = ref<SSLCert[]>([])
const loading = ref(false)
const scanning = ref(false)

function tagType(days: number) {
  if (days < 0) return 'danger'
  if (days <= 30) return 'warning'
  return 'success'
}

async function loadCerts() {
  if (!selectedServerId.value) return
  loading.value = true
  try {
    certs.value = await listCerts(selectedServerId.value)
  } finally { loading.value = false }
}

async function deleteCertItem(row: SSLCert) {
  if (!selectedServerId.value) return
  try {
    await deleteCert(selectedServerId.value, row.id)
    ElMessage.success('已删除')
    await loadCerts()
  } catch { ElMessage.error('删除失败') }
}

async function doScan() {
  if (!selectedServerId.value) return
  scanning.value = true
  try {
    const res = await scanCerts(selectedServerId.value)
    ElMessage.success(`扫描完成，导入 ${res?.imported ?? 0} 个证书`)
    await loadCerts()
  } catch { ElMessage.error('扫描失败') } finally {
    scanning.value = false
  }
}

// ── Request cert ────────────────────────────────────────────────
const requestVisible = ref(false)
const requesting = ref(false)
const requestOutput = ref('')
const requestOutputEl = ref<HTMLPreElement>()
const requestForm = ref({ domain: '', webroot: '', email: '' })

function openRequest() {
  if (!selectedServerId.value) { ElMessage.warning('请先选择服务器'); return }
  requestForm.value = { domain: '', webroot: '', email: '' }
  requestOutput.value = ''
  requestVisible.value = true
}

function startRequest() {
  if (!selectedServerId.value || !requestForm.value.domain) {
    ElMessage.warning('请填写域名')
    return
  }
  requesting.value = true
  requestOutput.value = ''
  const url = requestCertWsUrl(selectedServerId.value, {
    domain: requestForm.value.domain,
    webroot: requestForm.value.webroot || undefined,
    email: requestForm.value.email || undefined,
  }, auth.token)
  const ws = new WebSocket(url)
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') {
        requestOutput.value += msg.data + '\n'
        if (requestOutputEl.value) requestOutputEl.value.scrollTop = requestOutputEl.value.scrollHeight
      } else if (msg.type === 'done') {
        requesting.value = false
        ws.close()
        loadCerts()
      } else if (msg.type === 'error') {
        requestOutput.value += '[错误] ' + msg.data + '\n'
        requesting.value = false
        ws.close()
      }
    } catch { /* ignore */ }
  }
  ws.onerror = () => { requesting.value = false }
  ws.onclose = () => { if (requesting.value) requesting.value = false }
}

// ── Upload cert ─────────────────────────────────────────────────
const uploadVisible = ref(false)
const uploading = ref(false)
const uploadForm = ref({ domain: '', cert: '', key: '', cert_path: '', key_path: '' })

function openUpload() {
  if (!selectedServerId.value) { ElMessage.warning('请先选择服务器'); return }
  uploadForm.value = { domain: '', cert: '', key: '', cert_path: '', key_path: '' }
  uploadVisible.value = true
}

async function confirmUpload() {
  if (!selectedServerId.value) return
  if (!uploadForm.value.domain || !uploadForm.value.cert || !uploadForm.value.key) {
    ElMessage.warning('请填写域名、证书内容和私钥内容')
    return
  }
  uploading.value = true
  try {
    await uploadCert(selectedServerId.value, {
      domain: uploadForm.value.domain,
      cert: uploadForm.value.cert,
      key: uploadForm.value.key,
      cert_path: uploadForm.value.cert_path || undefined,
      key_path: uploadForm.value.key_path || undefined,
    })
    ElMessage.success('证书已上传')
    uploadVisible.value = false
    await loadCerts()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '上传失败')
  } finally {
    uploading.value = false
  }
}

// ── Renew cert ──────────────────────────────────────────────────
const renewVisible = ref(false)
const renewing = ref(false)
const renewOutput = ref('')
const renewDomain = ref('')
let currentCertId = 0

function openRenew(row: SSLCert) {
  renewDomain.value = row.domain
  currentCertId = row.id
  renewOutput.value = ''
  renewVisible.value = true
}

function startRenew() {
  if (!selectedServerId.value) return
  renewing.value = true
  renewOutput.value = ''
  const url = renewCertWsUrl(selectedServerId.value, currentCertId, auth.token)
  const ws = new WebSocket(url)
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') {
        renewOutput.value += msg.data + '\n'
      } else if (msg.type === 'done') {
        renewing.value = false
        ws.close()
        loadCerts()
      } else if (msg.type === 'error') {
        renewOutput.value += '[错误] ' + msg.data + '\n'
        renewing.value = false
        ws.close()
      }
    } catch { /* ignore */ }
  }
  ws.onerror = () => { renewing.value = false }
  ws.onclose = () => { if (renewing.value) renewing.value = false }
}

async function init() {
  servers.value = await getServers()
  if (servers.value.length) {
    selectedServerId.value = servers.value[0].id
    await loadCerts()
  }
}

init()
</script>

<style scoped>
.ssl-page { padding: 20px; }
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.ws-output {
  background: #1a1a2e;
  color: #a0f0a0;
  padding: 10px;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
  min-height: 120px;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  width: 100%;
  margin: 0;
}
</style>
