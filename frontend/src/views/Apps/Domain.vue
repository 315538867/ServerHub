<template>
  <div class="dom-page">
    <template v-if="app?.site_name && app?.server_id">
      <UiSection title="Nginx 站点">
        <template #extra>
          <UiButton variant="primary" size="sm" @click="openCreate">添加站点</UiButton>
          <UiButton variant="secondary" size="sm" :loading="reloading" @click="doReload">重载</UiButton>
          <UiButton variant="warning" size="sm" @click="doRestart">重启</UiButton>
          <UiButton variant="secondary" size="sm" :loading="loading" @click="loadSites">
            <template #icon><RefreshCw :size="14" /></template>
            刷新
          </UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="siteColumns"
            :data="sites"
            :loading="loading"
            :row-key="(row: SiteItem) => row.name"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>

      <UiSection title="SSL 证书">
        <template #extra>
          <UiButton variant="primary" size="sm" @click="openRequestCert">申请证书 (Let's Encrypt)</UiButton>
          <UiButton variant="secondary" size="sm" @click="loadCerts">
            <template #icon><RefreshCw :size="14" /></template>
            刷新
          </UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="certColumns"
            :data="certs"
            :loading="certLoading"
            :row-key="(row: SSLCert) => row.id"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>
    </template>
    <UiCard v-else padding="lg">
      <EmptyBlock description="该应用未关联 Nginx 站点，请先在应用设置中配置 site_name" />
    </UiCard>

    <NModal
      v-model:show="createVisible"
      preset="card"
      title="添加站点"
      style="width: 540px"
      :bordered="false"
    >
      <NForm :model="createForm" label-placement="left" label-width="90">
        <NFormItem label="站点名称">
          <NInput v-model:value="createForm.name" placeholder="my-site" />
        </NFormItem>
        <NFormItem label="类型">
          <NSelect v-model:value="createForm.type" :options="typeOptions" />
        </NFormItem>
        <NFormItem label="域名">
          <NInput v-model:value="createForm.domain" placeholder="example.com" />
        </NFormItem>
        <NFormItem label="监听端口">
          <NInputNumber v-model:value="createForm.port" :min="1" :max="65535" style="width: 100%" />
        </NFormItem>
        <NFormItem v-if="createForm.type !== 'proxy'" label="根目录">
          <NInput v-model:value="createForm.root" placeholder="/var/www/html" />
        </NFormItem>
        <NFormItem v-if="createForm.type === 'proxy'" label="代理地址">
          <NInput v-model:value="createForm.proxy" placeholder="http://127.0.0.1:3000" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="createVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="creating" @click="confirmCreate">创建</UiButton>
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="editVisible"
      preset="card"
      :title="`编辑配置 — ${editName}`"
      style="width: 800px"
      :bordered="false"
      :mask-closable="false"
      @after-leave="destroyEditor"
    >
      <div ref="editorEl" class="code-editor" />
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="editVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="saving" @click="saveConfig">保存并验证</UiButton>
        </div>
      </template>
    </NModal>

    <NDrawer v-model:show="logsVisible" :width="720" @after-leave="closeLogs">
      <NDrawerContent :title="`日志 — ${logsSite}`" :native-scrollbar="false">
        <UiTabs :items="logsTabs" :model-value="logsTab" @change="onLogsTabChange" />
        <div ref="logsEl" class="logs-terminal" />
      </NDrawerContent>
    </NDrawer>

    <NModal
      v-model:show="certReqVisible"
      preset="card"
      title="申请 Let's Encrypt 证书"
      style="width: 520px"
      :bordered="false"
      :mask-closable="!certRequesting"
    >
      <NForm :model="certReqForm" label-placement="left" label-width="80">
        <NFormItem label="域名">
          <NInput v-model:value="certReqForm.domain" :placeholder="app?.domain || 'example.com'" />
        </NFormItem>
        <NFormItem label="邮箱">
          <NInput v-model:value="certReqForm.email" placeholder="admin@example.com" />
        </NFormItem>
        <NFormItem label="Webroot">
          <NInput v-model:value="certReqForm.webroot" placeholder="/var/www/html（留空使用 standalone）" />
        </NFormItem>
      </NForm>
      <pre v-if="certOutput" ref="certOutputEl" class="cert-output">{{ certOutput }}</pre>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" :disabled="certRequesting" @click="certReqVisible = false">关闭</UiButton>
          <UiButton variant="primary" size="sm" :loading="certRequesting" @click="startRequestCert">申请</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  NDataTable, NModal, NDrawer, NDrawerContent, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { showApiError } from '@/utils/errors'
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
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiTabs from '@/components/ui/UiTabs.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const route = useRoute()
const auth = useAuthStore()
const appStore = useAppStore()
const message = useMessage()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const serverId = computed(() => app.value?.server_id ?? 0)

const sites = ref<SiteItem[]>([])
const loading = ref(false)
const reloading = ref(false)
const certs = ref<SSLCert[]>([])
const certLoading = ref(false)

const typeOptions = [
  { label: '静态文件', value: 'static' },
  { label: '反向代理', value: 'proxy' },
  { label: 'PHP', value: 'php' },
]

const logsTabs = [
  { value: 'access', label: '访问日志' },
  { value: 'error', label: '错误日志' },
]

const siteColumns = computed<DataTableColumns<SiteItem>>(() => [
  { title: '站点名', key: 'name', minWidth: 180, ellipsis: { tooltip: true } },
  { title: '配置文件', key: 'path', minWidth: 280, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge,
      { tone: row.enabled ? 'success' : 'neutral' },
      () => row.enabled ? '启用' : '禁用'),
  },
  {
    title: '操作', key: 'ops', width: 320, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      !row.enabled ? h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => toggleSite(row, true) }, () => '启用') : null,
      row.enabled ? h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => toggleSite(row, false) }, () => '禁用') : null,
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEdit(row) }, () => '编辑配置'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openLogs(row) }, () => '日志'),
      h(NPopconfirm, {
        onPositiveClick: () => delSite(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => `确认删除站点 ${row.name}？`,
      }),
    ]),
  },
])

const certColumns = computed<DataTableColumns<SSLCert>>(() => [
  { title: '域名', key: 'domain', minWidth: 180 },
  { title: '签发机构', key: 'issuer', minWidth: 140 },
  { title: '到期时间', key: 'expires_at', minWidth: 160 },
  {
    title: '剩余天数', key: 'days_left', width: 110,
    render: (row) => h(UiBadge, {
      tone: row.days_left < 14 ? 'danger' : row.days_left < 30 ? 'warning' : 'success',
    }, () => `${row.days_left}天`),
  },
  {
    title: '操作', key: 'ops', width: 160, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openRenew(row) }, () => '续签'),
      h(NPopconfirm, {
        onPositiveClick: () => delCert(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除该证书？',
      }),
    ]),
  },
])

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
    message.success('站点已创建')
    createVisible.value = false
    await loadSites()
  } catch (e: any) {
    showApiError(e, '创建失败')
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
  } catch { message.error('读取配置失败'); editVisible.value = false }
}

async function saveConfig() {
  if (!editorView) return
  saving.value = true
  try {
    await putSiteConfig(serverId.value, editName.value, editorView.state.doc.toString())
    message.success('配置已保存')
    editVisible.value = false
  } catch (e: any) {
    showApiError(e, '保存失败')
  } finally { saving.value = false }
}

function destroyEditor() { editorView?.destroy(); editorView = null }

async function toggleSite(row: SiteItem, enable: boolean) {
  try {
    if (enable) await enableSite(serverId.value, row.name)
    else await disableSite(serverId.value, row.name)
    message.success(enable ? '已启用' : '已禁用')
    await loadSites()
  } catch (e: any) { showApiError(e, '操作失败') }
}

async function delSite(row: SiteItem) {
  try { await deleteSite(serverId.value, row.name); message.success('已删除'); await loadSites() }
  catch (e: any) { showApiError(e, '删除失败') }
}

async function doReload() {
  reloading.value = true
  try { await nginxReload(serverId.value); message.success('nginx reload 成功') }
  catch (e: any) { showApiError(e, 'reload 失败') }
  finally { reloading.value = false }
}

async function doRestart() {
  try { await nginxRestart(serverId.value); message.success('nginx restart 成功') }
  catch (e: any) { showApiError(e, 'restart 失败') }
}

const logsVisible = ref(false)
const logsSite = ref('')
const logsTab = ref<string | number>('access')
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
  logsTab.value = val
  closeLogs()
  nextTick(() => startLogsStream(String(val)))
}

function startLogsStream(type: string) {
  if (!logsEl.value) return
  logsTerm?.dispose()
  logsTerm = new Terminal({
    theme: { background: '#0A0A0A', foreground: '#E4E4E7' },
    convertEol: true, fontSize: 12,
  })
  const fit = new FitAddon(); logsTerm.loadAddon(fit); logsTerm.open(logsEl.value); fit.fit()
  logsWs?.close()
  const url = type === 'access' ? accessLogsWsUrl(serverId.value) : errorLogsWsUrl(serverId.value)
  logsWs = new WebSocket(url, ['bearer', auth.token ?? ''])
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
  try { await apiDeleteCert(serverId.value, row.id); message.success('证书已删除'); await loadCerts() }
  catch { message.error('删除失败') }
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
  certWs = new WebSocket(requestCertWsUrl(serverId.value, { domain, email, webroot }), ['bearer', auth.token ?? ''])
  certWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') { certOutput.value += msg.data + '\n'; nextTick(() => { if (certOutputEl.value) certOutputEl.value.scrollTop = certOutputEl.value.scrollHeight }) }
      else if (msg.type === 'done') { certRequesting.value = false; message.success('证书申请成功'); loadCerts() }
      else if (msg.type === 'error') { certRequesting.value = false; certOutput.value += '[错误] ' + msg.data + '\n' }
    } catch { /* ignore */ }
  }
  certWs.onerror = () => { certRequesting.value = false }
}

function openRenew(row: SSLCert) {
  certOutput.value = ''; certReqVisible.value = true; certRequesting.value = true
  certWs?.close()
  certWs = new WebSocket(renewCertWsUrl(serverId.value, row.id), ['bearer', auth.token ?? ''])
  certWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') { certOutput.value += msg.data + '\n'; nextTick(() => { if (certOutputEl.value) certOutputEl.value.scrollTop = certOutputEl.value.scrollHeight }) }
      else if (msg.type === 'done') { certRequesting.value = false; message.success('续签成功'); loadCerts() }
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
.dom-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }
.code-editor { height: 60vh; overflow: auto; font-size: 13px; border-radius: var(--radius-sm); border: 1px solid var(--ui-border); }
:deep(.cm-editor) { height: 100%; }
:deep(.cm-scroller) { overflow: auto; }
.logs-terminal {
  width: 100%;
  height: calc(100vh - 240px);
  background: #0A0A0A;
  border-radius: var(--radius-sm);
  overflow: hidden;
  margin-top: var(--space-3);
  padding: var(--space-2);
}
.cert-output {
  background: #0A0A0A;
  color: #E4E4E7;
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 280px;
  margin: var(--space-3) 0 0;
  white-space: pre-wrap;
  word-break: break-all;
}
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
