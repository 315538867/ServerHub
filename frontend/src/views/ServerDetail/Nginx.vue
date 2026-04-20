<template>
  <div class="nginx-page">
    <div class="page-toolbar">
      <t-button theme="primary" @click="openCreate">添加站点</t-button>
      <t-button variant="outline" :loading="reloading" @click="doReload">Nginx Reload</t-button>
      <t-button theme="warning" variant="outline" @click="doRestart">Nginx Restart</t-button>
      <t-button variant="outline" :loading="loading" @click="loadSites">
        <template #icon><refresh-icon /></template>
        刷新
      </t-button>
    </div>

    <t-table :data="sites" :columns="siteColumns" :loading="loading" row-key="name" stripe>
      <template #enabled="{ row }">
        <t-tag :theme="row.enabled ? 'success' : 'default'" variant="light" size="small">{{ row.enabled ? '启用' : '禁用' }}</t-tag>
      </template>
      <template #operations="{ row }">
        <t-space size="small">
          <t-button v-if="!row.enabled" theme="success" size="small" variant="text" @click="toggleSite(row, true)">启用</t-button>
          <t-button v-if="row.enabled" theme="warning" size="small" variant="text" @click="toggleSite(row, false)">禁用</t-button>
          <t-button size="small" variant="text" @click="openEdit(row)">编辑配置</t-button>
          <t-button size="small" variant="text" @click="openLogs(row)">日志</t-button>
          <t-popconfirm :content="`确认删除站点 ${row.name}？`" @confirm="delSite(row)">
            <t-button theme="danger" size="small" variant="text">删除</t-button>
          </t-popconfirm>
        </t-space>
      </template>
    </t-table>

    <t-dialog
      v-model:visible="createVisible"
      header="添加站点"
      width="520px"
      :confirm-btn="{ content: '创建', loading: creating }"
      @confirm="confirmCreate"
    >
      <t-form :data="createForm" label-width="90px" colon>
        <t-form-item label="站点名称">
          <t-input v-model="createForm.name" placeholder="my-site" />
        </t-form-item>
        <t-form-item label="类型">
          <t-select v-model="createForm.type" style="width:100%">
            <t-option label="静态文件" value="static" />
            <t-option label="反向代理" value="proxy" />
            <t-option label="PHP" value="php" />
          </t-select>
        </t-form-item>
        <t-form-item label="域名">
          <t-input v-model="createForm.domain" placeholder="example.com" />
        </t-form-item>
        <t-form-item label="监听端口">
          <t-input-number v-model="createForm.port" :min="1" :max="65535" style="width:100%" />
        </t-form-item>
        <t-form-item v-if="createForm.type !== 'proxy'" label="根目录">
          <t-input v-model="createForm.root" placeholder="/var/www/html" />
        </t-form-item>
        <t-form-item v-if="createForm.type === 'proxy'" label="代理地址">
          <t-input v-model="createForm.proxy" placeholder="http://127.0.0.1:3000" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="editVisible"
      :header="`编辑配置 — ${editName}`"
      width="800px"
      :close-on-overlay-click="false"
      :confirm-btn="{ content: '保存并验证', loading: saving }"
      @confirm="saveConfig"
      @closed="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
    </t-dialog>

    <t-drawer v-model:visible="logsVisible" :header="`日志 — ${logsSite}`" size="60%" @opened="initLogs" @close="closeLogs">
      <t-tabs :value="logsTab" @change="switchLogsTab">
        <t-tab-panel value="access" label="访问日志" />
        <t-tab-panel value="error" label="错误日志" />
      </t-tabs>
      <div ref="logsEl" class="logs-terminal" />
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import {
  getSites, createSite, getSiteConfig, putSiteConfig,
  deleteSite, enableSite, disableSite, nginxReload, nginxRestart,
  accessLogsWsUrl, errorLogsWsUrl,
} from '@/api/nginx'
import type { SiteItem } from '@/api/nginx'

const route = useRoute()
const auth = useAuthStore()
const serverId = computed(() => Number(route.params.serverId))
const sites = ref<SiteItem[]>([])
const loading = ref(false)
const reloading = ref(false)

const siteColumns = [
  { colKey: 'name', title: '站点名', minWidth: 180, ellipsis: true },
  { colKey: 'path', title: '配置文件', minWidth: 280, ellipsis: true },
  { colKey: 'enabled', title: '状态', width: 90 },
  { colKey: 'operations', title: '操作', width: 300, fixed: 'right' as const },
]

const createVisible = ref(false)
const creating = ref(false)
const createForm = ref({ name: '', type: 'static' as 'static' | 'proxy' | 'php', domain: '', port: 80, root: '', proxy: '' })

function openCreate() {
  createForm.value = { name: '', type: 'static', domain: '', port: 80, root: '', proxy: '' }
  createVisible.value = true
}

async function confirmCreate() {
  if (!createForm.value.name || !createForm.value.domain) return
  creating.value = true
  try {
    await createSite(serverId.value, createForm.value)
    MessagePlugin.success('站点已创建')
    createVisible.value = false
    await loadSites()
  } catch (e: any) {
    MessagePlugin.error(e?.response?.data?.msg ?? '创建失败')
  } finally {
    creating.value = false
  }
}

const editVisible = ref(false)
const editName = ref('')
const saving = ref(false)
const editorEl = ref<HTMLDivElement>()
let editorView: EditorView | null = null

async function openEdit(row: SiteItem) {
  editName.value = row.name
  editVisible.value = true
  await nextTick()
  try {
    const res = await getSiteConfig(serverId.value, row.name)
    editorView?.destroy()
    editorView = new EditorView({
      state: EditorState.create({ doc: res.content, extensions: [basicSetup, oneDark] }),
      parent: editorEl.value!,
    })
  } catch {
    MessagePlugin.error('读取配置失败')
    editVisible.value = false
  }
}

async function saveConfig() {
  if (!editorView) return
  saving.value = true
  try {
    await putSiteConfig(serverId.value, editName.value, editorView.state.doc.toString())
    MessagePlugin.success('配置已保存（nginx -t 验证通过）')
    editVisible.value = false
  } catch (e: any) {
    MessagePlugin.error(e?.response?.data?.msg ?? '保存失败')
  } finally {
    saving.value = false
  }
}

function destroyEditor() { editorView?.destroy(); editorView = null }

async function toggleSite(row: SiteItem, enable: boolean) {
  try {
    if (enable) await enableSite(serverId.value, row.name)
    else await disableSite(serverId.value, row.name)
    MessagePlugin.success(enable ? '已启用' : '已禁用')
    await loadSites()
  } catch { MessagePlugin.error('操作失败') }
}

async function delSite(row: SiteItem) {
  try { await deleteSite(serverId.value, row.name); MessagePlugin.success('已删除'); await loadSites() }
  catch { MessagePlugin.error('删除失败') }
}

async function doReload() {
  reloading.value = true
  try { await nginxReload(serverId.value); MessagePlugin.success('nginx reload 成功') }
  catch { MessagePlugin.error('reload 失败') }
  finally { reloading.value = false }
}

async function doRestart() {
  try { await nginxRestart(serverId.value); MessagePlugin.success('nginx restart 成功') }
  catch { MessagePlugin.error('restart 失败') }
}

const logsVisible = ref(false)
const logsSite = ref('')
const logsTab = ref<string>('access')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

function openLogs(row: SiteItem) {
  logsSite.value = row.name; logsTab.value = 'access'; logsVisible.value = true
}

function initLogs() { startLogsStream(logsTab.value) }

function switchLogsTab(tab: string | number) {
  closeLogs()
  logsTab.value = tab as string
  nextTick(() => startLogsStream(logsTab.value))
}

function startLogsStream(type: string) {
  if (!logsEl.value) return
  logsTerm?.dispose()
  logsTerm = new Terminal({ theme: { background: '#1a2332' }, convertEol: true, fontSize: 13 })
  const fit = new FitAddon()
  logsTerm.loadAddon(fit); logsTerm.open(logsEl.value); fit.fit()
  logsWs?.close()
  const url = type === 'access'
    ? accessLogsWsUrl(serverId.value, auth.token)
    : errorLogsWsUrl(serverId.value, auth.token)
  logsWs = new WebSocket(url)
  logsWs.onmessage = (e) => {
    try { const msg = JSON.parse(e.data); if (msg.type === 'output') logsTerm?.writeln(msg.data) } catch { /* ignore */ }
  }
}

function closeLogs() { logsWs?.close(); logsWs = null; logsTerm?.dispose(); logsTerm = null }

async function loadSites() {
  loading.value = true
  try { sites.value = await getSites(serverId.value) } finally { loading.value = false }
}

onMounted(() => loadSites())
onBeforeUnmount(() => { closeLogs(); editorView?.destroy() })
</script>

<style scoped>
.nginx-page { padding: 4px 0; }
.page-toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.code-editor { height: 60vh; overflow: auto; border: 1px solid var(--td-component-border); border-radius: 4px; font-size: 13px; }
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
.logs-terminal { width: 100%; height: calc(100vh - 220px); background: #1a2332; border-radius: 4px; overflow: hidden; margin-top: 8px; }
</style>
