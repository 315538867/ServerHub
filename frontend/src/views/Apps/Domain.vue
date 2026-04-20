<template>
  <div class="page-container">
    <template v-if="app?.site_name && app?.server_id">
      <!-- Nginx 站点 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">Nginx 站点</span>
          <t-space size="small">
            <t-button theme="primary" size="small" @click="openCreate">添加站点</t-button>
            <t-button size="small" :loading="reloading" @click="doReload">重载</t-button>
            <t-button size="small" theme="warning" @click="doRestart">重启</t-button>
            <t-button size="small" variant="outline" :loading="loading" @click="loadSites">
              <template #icon><refresh-icon /></template>
              刷新
            </t-button>
          </t-space>
        </div>
        <div class="table-wrap">
          <t-table :data="sites" :columns="siteColumns" :loading="loading" row-key="name" stripe size="small">
            <template #status="{ row }">
              <t-tag :theme="row.enabled ? 'success' : 'default'" variant="light" size="small">
                {{ row.enabled ? '启用' : '禁用' }}
              </t-tag>
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
        </div>
      </div>

      <!-- SSL 证书 -->
      <div class="section-block">
        <div class="section-title">
          <span class="title-text">SSL 证书</span>
          <t-space size="small">
            <t-button size="small" theme="primary" @click="openRequestCert">申请证书 (Let's Encrypt)</t-button>
            <t-button size="small" variant="outline" @click="loadCerts">刷新</t-button>
          </t-space>
        </div>
        <div class="table-wrap">
          <t-table :data="certs" :columns="certColumns" :loading="certLoading" row-key="id" stripe size="small">
            <template #days_left="{ row }">
              <t-tag :theme="row.days_left < 14 ? 'danger' : row.days_left < 30 ? 'warning' : 'success'" variant="light" size="small">
                {{ row.days_left }}天
              </t-tag>
            </template>
            <template #operations="{ row }">
              <t-space size="small">
                <t-button size="small" variant="text" @click="openRenew(row)">续签</t-button>
                <t-popconfirm content="确认删除该证书？" @confirm="delCert(row)">
                  <t-button theme="danger" size="small" variant="text">删除</t-button>
                </t-popconfirm>
              </t-space>
            </template>
          </t-table>
        </div>
      </div>
    </template>
    <div v-else class="section-block empty-block">
      <t-empty description="该应用未关联 Nginx 站点，请先在应用设置中配置 site_name" />
    </div>

    <!-- 添加站点 Dialog -->
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
          <t-select v-model="createForm.type" class="full-width">
            <t-option label="静态文件" value="static" />
            <t-option label="反向代理" value="proxy" />
            <t-option label="PHP" value="php" />
          </t-select>
        </t-form-item>
        <t-form-item label="域名">
          <t-input v-model="createForm.domain" placeholder="example.com" />
        </t-form-item>
        <t-form-item label="监听端口">
          <t-input-number v-model="createForm.port" :min="1" :max="65535" class="full-width" />
        </t-form-item>
        <t-form-item v-if="createForm.type !== 'proxy'" label="根目录">
          <t-input v-model="createForm.root" placeholder="/var/www/html" />
        </t-form-item>
        <t-form-item v-if="createForm.type === 'proxy'" label="代理地址">
          <t-input v-model="createForm.proxy" placeholder="http://127.0.0.1:3000" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 编辑配置 Dialog -->
    <t-dialog
      v-model:visible="editVisible"
      :header="`编辑配置 — ${editName}`"
      width="800px"
      placement="center"
      :close-on-overlay-click="false"
      :confirm-btn="{ content: '保存并验证', loading: saving }"
      class="code-editor-dialog"
      @confirm="saveConfig"
      @closed="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
    </t-dialog>

    <!-- 日志 Drawer -->
    <t-drawer v-model:visible="logsVisible" :header="`日志 — ${logsSite}`" size="60%" @closed="closeLogs">
      <t-tabs :value="logsTab" @change="onLogsTabChange">
        <t-tab-panel value="access" label="访问日志" />
        <t-tab-panel value="error" label="错误日志" />
      </t-tabs>
      <div ref="logsEl" class="logs-terminal" />
    </t-drawer>

    <!-- 申请证书 Dialog -->
    <t-dialog
      v-model:visible="certReqVisible"
      header="申请 Let's Encrypt 证书"
      width="480px"
      :close-on-overlay-click="!certRequesting"
      :confirm-btn="{ content: '申请', loading: certRequesting }"
      @confirm="startRequestCert"
    >
      <t-form :data="certReqForm" label-width="80px" colon>
        <t-form-item label="域名">
          <t-input v-model="certReqForm.domain" :placeholder="app?.domain || 'example.com'" />
        </t-form-item>
        <t-form-item label="邮箱">
          <t-input v-model="certReqForm.email" placeholder="admin@example.com" />
        </t-form-item>
        <t-form-item label="Webroot">
          <t-input v-model="certReqForm.webroot" placeholder="/var/www/html（留空使用 standalone）" />
        </t-form-item>
      </t-form>
      <pre v-if="certOutput" ref="certOutputEl" class="cert-output">{{ certOutput }}</pre>
    </t-dialog>
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
import { useAppStore } from '@/stores/app'
import {
  getSites, createSite, getSiteConfig, putSiteConfig, deleteSite,
  enableSite, disableSite, nginxReload, nginxRestart, accessLogsWsUrl, errorLogsWsUrl,
} from '@/api/nginx'
import type { SiteItem } from '@/api/nginx'
import { listCerts, deleteCert as apiDeleteCert, requestCertWsUrl, renewCertWsUrl } from '@/api/ssl'
import type { SSLCert } from '@/api/ssl'

const route = useRoute()
const auth = useAuthStore()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const serverId = computed(() => app.value?.server_id ?? 0)

const sites = ref<SiteItem[]>([])
const loading = ref(false)
const reloading = ref(false)
const certs = ref<SSLCert[]>([])
const certLoading = ref(false)

const siteColumns = [
  { colKey: 'name', title: '站点名', minWidth: 180, ellipsis: true },
  { colKey: 'path', title: '配置文件', minWidth: 280, ellipsis: true },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'operations', title: '操作', width: 320, fixed: 'right' as const },
]

const certColumns = [
  { colKey: 'domain', title: '域名', minWidth: 180 },
  { colKey: 'issuer', title: '签发机构', minWidth: 140 },
  { colKey: 'expires_at', title: '到期时间', minWidth: 140 },
  { colKey: 'days_left', title: '剩余天数', width: 100 },
  { colKey: 'operations', title: '操作', width: 160, fixed: 'right' as const },
]

const createVisible = ref(false)
const creating = ref(false)
const createForm = ref({ name: '', type: 'static' as 'static' | 'proxy' | 'php', domain: '', port: 80, root: '', proxy: '' })

function openCreate() {
  createForm.value = { name: app.value?.site_name || '', type: 'proxy', domain: app.value?.domain || '', port: 80, root: '', proxy: '' }
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
  } finally { creating.value = false }
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
  } catch { MessagePlugin.error('读取配置失败'); editVisible.value = false }
}

async function saveConfig() {
  if (!editorView) return
  saving.value = true
  try {
    await putSiteConfig(serverId.value, editName.value, editorView.state.doc.toString())
    MessagePlugin.success('配置已保存')
    editVisible.value = false
  } catch (e: any) {
    MessagePlugin.error(e?.response?.data?.msg ?? '保存失败')
  } finally { saving.value = false }
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
const logsTab = ref('access')
const logsEl = ref<HTMLDivElement>()
let logsTerm: Terminal | null = null
let logsWs: WebSocket | null = null

function openLogs(row: SiteItem) {
  logsSite.value = row.name
  logsTab.value = 'access'
  logsVisible.value = true
  nextTick(() => startLogsStream('access'))
}

function onLogsTabChange(val: string | number) {
  const tab = val as string
  logsTab.value = tab
  closeLogs()
  nextTick(() => startLogsStream(tab))
}

function startLogsStream(type: string) {
  if (!logsEl.value) return
  logsTerm?.dispose()
  logsTerm = new Terminal({ theme: { background: '#1a2332' }, convertEol: true, fontSize: 13 })
  const fit = new FitAddon(); logsTerm.loadAddon(fit); logsTerm.open(logsEl.value); fit.fit()
  logsWs?.close()
  const url = type === 'access' ? accessLogsWsUrl(serverId.value, auth.token) : errorLogsWsUrl(serverId.value, auth.token)
  logsWs = new WebSocket(url)
  logsWs.onmessage = (e) => {
    try { const msg = JSON.parse(e.data); if (msg.type === 'output') logsTerm?.writeln(msg.data) } catch { /* ignore */ }
  }
}

function closeLogs() { logsWs?.close(); logsWs = null; logsTerm?.dispose(); logsTerm = null }

async function loadCerts() {
  certLoading.value = true
  try { certs.value = await listCerts(serverId.value) } catch { /* ignore */ }
  finally { certLoading.value = false }
}

async function delCert(row: SSLCert) {
  try { await apiDeleteCert(serverId.value, row.id); MessagePlugin.success('证书已删除'); await loadCerts() }
  catch { MessagePlugin.error('删除失败') }
}

const certReqVisible = ref(false)
const certRequesting = ref(false)
const certReqForm = ref({ domain: '', email: '', webroot: '' })
const certOutput = ref('')
const certOutputEl = ref<HTMLPreElement>()
let certWs: WebSocket | null = null

function openRequestCert() {
  certReqForm.value = { domain: app.value?.domain || '', email: '', webroot: '' }
  certOutput.value = ''
  certReqVisible.value = true
}

function startRequestCert() {
  const { domain, email, webroot } = certReqForm.value
  if (!domain) return
  certRequesting.value = true; certOutput.value = ''
  certWs?.close()
  certWs = new WebSocket(requestCertWsUrl(serverId.value, { domain, email, webroot }, auth.token))
  certWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') { certOutput.value += msg.data + '\n'; nextTick(() => { if (certOutputEl.value) certOutputEl.value.scrollTop = certOutputEl.value.scrollHeight }) }
      else if (msg.type === 'done') { certRequesting.value = false; MessagePlugin.success('证书申请成功'); loadCerts() }
      else if (msg.type === 'error') { certRequesting.value = false; certOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  certWs.onerror = () => { certRequesting.value = false }
}

function openRenew(row: SSLCert) {
  certOutput.value = ''; certReqVisible.value = true; certRequesting.value = true
  certWs?.close()
  certWs = new WebSocket(renewCertWsUrl(serverId.value, row.id, auth.token))
  certWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') { certOutput.value += msg.data + '\n'; nextTick(() => { if (certOutputEl.value) certOutputEl.value.scrollTop = certOutputEl.value.scrollHeight }) }
      else if (msg.type === 'done') { certRequesting.value = false; MessagePlugin.success('续签成功'); loadCerts() }
      else if (msg.type === 'error') { certRequesting.value = false; certOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  certWs.onerror = () => { certRequesting.value = false }
}

async function loadSites() {
  if (!serverId.value) return
  loading.value = true
  try { sites.value = await getSites(serverId.value) } finally { loading.value = false }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (serverId.value) { await loadSites(); await loadCerts() }
})
onBeforeUnmount(() => { closeLogs(); editorView?.destroy(); certWs?.close() })
</script>

<style scoped>
.table-wrap {
  padding: 0 var(--sh-space-lg) var(--sh-space-md);
}
.empty-block {
  padding: var(--sh-space-xl) var(--sh-space-lg);
  display: flex;
  justify-content: center;
}
:deep(.t-table td) {
  font-size: 13px;
}
.code-editor { height: 60vh; overflow: auto; font-size: 13px; }
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
.logs-terminal { width: 100%; height: calc(100vh - 240px); background: #1a2332; border-radius: 4px; overflow: hidden; margin-top: var(--sh-space-md); }
.cert-output { background: #1a2332; color: #e0e0e0; border-radius: 4px; padding: var(--sh-space-md); font-size: 12px; line-height: 1.6; overflow: auto; max-height: 280px; margin: var(--sh-space-md) 0 0; white-space: pre-wrap; word-break: break-all; }
.full-width { width: 100%; }
</style>
