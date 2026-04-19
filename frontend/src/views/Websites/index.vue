<template>
  <div class="websites-page">
    <div class="page-toolbar">
      <el-select v-model="selectedServerId" placeholder="选择服务器" style="width:220px" @change="loadSites">
        <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
      </el-select>
      <el-button type="primary" @click="openCreate">添加站点</el-button>
      <el-button @click="doReload" :loading="reloading">Nginx Reload</el-button>
      <el-button type="warning" @click="doRestart">Nginx Restart</el-button>
      <el-button :icon="Refresh" :loading="loading" @click="loadSites">刷新</el-button>
    </div>

    <el-table :data="sites" v-loading="loading" style="width:100%">
      <el-table-column label="站点名" prop="name" min-width="180" show-overflow-tooltip />
      <el-table-column label="配置文件" prop="path" min-width="280" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
            {{ row.enabled ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-button v-if="!row.enabled" size="small" type="success" @click="toggleSite(row, true)">启用</el-button>
          <el-button v-if="row.enabled" size="small" type="warning" @click="toggleSite(row, false)">禁用</el-button>
          <el-button size="small" @click="openEdit(row)">编辑配置</el-button>
          <el-button size="small" @click="openLogs(row)">日志</el-button>
          <el-popconfirm :title="`确认删除站点 ${row.name}？`" @confirm="delSite(row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- ── Create site dialog ───────────────────────────────── -->
    <el-dialog v-model="createVisible" title="添加站点" width="520px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="站点名称">
          <el-input v-model="createForm.name" placeholder="my-site" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="createForm.type" style="width:100%">
            <el-option label="静态文件" value="static" />
            <el-option label="反向代理" value="proxy" />
            <el-option label="PHP" value="php" />
          </el-select>
        </el-form-item>
        <el-form-item label="域名">
          <el-input v-model="createForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="监听端口">
          <el-input-number v-model="createForm.port" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item v-if="createForm.type !== 'proxy'" label="根目录">
          <el-input v-model="createForm.root" placeholder="/var/www/html" />
        </el-form-item>
        <el-form-item v-if="createForm.type === 'proxy'" label="代理地址">
          <el-input v-model="createForm.proxy" placeholder="http://127.0.0.1:3000" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="confirmCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- ── Edit config dialog ────────────────────────────────── -->
    <el-dialog
      v-model="editVisible"
      :title="`编辑配置 — ${editName}`"
      width="800px"
      top="4vh"
      :close-on-click-modal="false"
      @closed="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveConfig">保存并验证</el-button>
      </template>
    </el-dialog>

    <!-- ── Logs drawer ──────────────────────────────────────── -->
    <el-drawer
      v-model="logsVisible"
      :title="`日志 — ${logsSite}`"
      size="60%"
      direction="rtl"
      @opened="initLogs"
      @closed="closeLogs"
    >
      <el-tabs v-model="logsTab" @tab-change="switchLogsTab">
        <el-tab-pane label="访问日志" name="access" />
        <el-tab-pane label="错误日志" name="error" />
      </el-tabs>
      <div ref="logsEl" class="logs-terminal" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onBeforeUnmount } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { getServers } from '@/api/servers'
import {
  getSites, createSite, getSiteConfig, putSiteConfig,
  deleteSite, enableSite, disableSite, nginxReload, nginxRestart,
  accessLogsWsUrl, errorLogsWsUrl,
} from '@/api/nginx'
import type { SiteItem } from '@/api/nginx'
import type { Server } from '@/types/api'

const auth = useAuthStore()
const servers = ref<Server[]>([])
const selectedServerId = ref<number | null>(null)
const sites = ref<SiteItem[]>([])
const loading = ref(false)
const reloading = ref(false)

// ── Create ────────────────────────────────────────────────────────
const createVisible = ref(false)
const creating = ref(false)
const createForm = ref({ name: '', type: 'static' as 'static'|'proxy'|'php', domain: '', port: 80, root: '', proxy: '' })

function openCreate() {
  if (!selectedServerId.value) { ElMessage.warning('请先选择服务器'); return }
  createForm.value = { name: '', type: 'static', domain: '', port: 80, root: '', proxy: '' }
  createVisible.value = true
}

async function confirmCreate() {
  if (!selectedServerId.value || !createForm.value.name || !createForm.value.domain) return
  creating.value = true
  try {
    await createSite(selectedServerId.value, createForm.value)
    ElMessage.success('站点已创建')
    createVisible.value = false
    await loadSites()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '创建失败')
  } finally {
    creating.value = false
  }
}

// ── Edit config ───────────────────────────────────────────────────
const editVisible = ref(false)
const editName = ref('')
const saving = ref(false)
const editorEl = ref<HTMLDivElement>()
let editorView: EditorView | null = null

async function openEdit(row: SiteItem) {
  if (!selectedServerId.value) return
  editName.value = row.name
  editVisible.value = true
  await nextTick()
  try {
    const res = await getSiteConfig(selectedServerId.value, row.name)
    editorView?.destroy()
    editorView = new EditorView({
      state: EditorState.create({ doc: res.content, extensions: [basicSetup, oneDark] }),
      parent: editorEl.value!,
    })
  } catch {
    ElMessage.error('读取配置失败')
    editVisible.value = false
  }
}

async function saveConfig() {
  if (!selectedServerId.value || !editorView) return
  saving.value = true
  try {
    await putSiteConfig(selectedServerId.value, editName.value, editorView.state.doc.toString())
    ElMessage.success('配置已保存（nginx -t 验证通过）')
    editVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg ?? '保存失败')
  } finally {
    saving.value = false
  }
}

function destroyEditor() {
  editorView?.destroy()
  editorView = null
}

// ── Enable/Disable/Delete ─────────────────────────────────────────
async function toggleSite(row: SiteItem, enable: boolean) {
  if (!selectedServerId.value) return
  try {
    if (enable) await enableSite(selectedServerId.value, row.name)
    else await disableSite(selectedServerId.value, row.name)
    ElMessage.success(enable ? '已启用' : '已禁用')
    await loadSites()
  } catch { ElMessage.error('操作失败') }
}

async function delSite(row: SiteItem) {
  if (!selectedServerId.value) return
  try {
    await deleteSite(selectedServerId.value, row.name)
    ElMessage.success('已删除')
    await loadSites()
  } catch { ElMessage.error('删除失败') }
}

// ── Reload/Restart ────────────────────────────────────────────────
async function doReload() {
  if (!selectedServerId.value) return
  reloading.value = true
  try {
    await nginxReload(selectedServerId.value)
    ElMessage.success('nginx reload 成功')
  } catch { ElMessage.error('reload 失败') } finally {
    reloading.value = false
  }
}

async function doRestart() {
  if (!selectedServerId.value) return
  try {
    await nginxRestart(selectedServerId.value)
    ElMessage.success('nginx restart 成功')
  } catch { ElMessage.error('restart 失败') }
}

// ── Logs ─────────────────────────────────────────────────────────
const logsVisible = ref(false)
const logsSite = ref('')
const logsTab = ref('access')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

function openLogs(row: SiteItem) {
  logsSite.value = row.name
  logsTab.value = 'access'
  logsVisible.value = true
}

function initLogs() {
  if (!logsEl.value || !selectedServerId.value) return
  startLogsStream(logsTab.value)
}

function switchLogsTab(tab: string) {
  closeLogs()
  nextTick(() => startLogsStream(tab))
}

function startLogsStream(type: string) {
  if (!logsEl.value || !selectedServerId.value) return
  logsTerm?.dispose()
  logsTerm = new Terminal({ theme: { background: '#1a1a2e' }, convertEol: true, fontSize: 13 })
  const fit = new FitAddon()
  logsTerm.loadAddon(fit)
  logsTerm.open(logsEl.value)
  fit.fit()
  logsWs?.close()
  const url = type === 'access'
    ? accessLogsWsUrl(selectedServerId.value, auth.token)
    : errorLogsWsUrl(selectedServerId.value, auth.token)
  logsWs = new WebSocket(url)
  logsWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') logsTerm?.writeln(msg.data)
    } catch { /* ignore */ }
  }
}

function closeLogs() {
  logsWs?.close()
  logsWs = null
  logsTerm?.dispose()
  logsTerm = null
}

async function loadSites() {
  if (!selectedServerId.value) return
  loading.value = true
  try {
    sites.value = await getSites(selectedServerId.value)
  } finally { loading.value = false }
}

onBeforeUnmount(() => {
  closeLogs()
  editorView?.destroy()
})

async function init() {
  servers.value = await getServers()
  if (servers.value.length) {
    selectedServerId.value = servers.value[0].id
    await loadSites()
  }
}

init()
</script>

<style scoped>
.websites-page { padding: 20px; }
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.code-editor {
  height: 60vh;
  overflow: auto;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  font-size: 13px;
}
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
.logs-terminal {
  width: 100%;
  height: calc(100vh - 200px);
  background: #1a1a2e;
  border-radius: 4px;
  overflow: hidden;
}
</style>
