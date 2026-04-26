<template>
  <div class="ng-page">
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

    <UiSection title="实例信息（多实例 / 自编译适配）">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="probing" @click="doProbe">
          重新探测 nginx -V
        </UiButton>
        <UiButton variant="primary" size="sm" :loading="profileSaving" @click="saveProfile">
          保存路径/命令
        </UiButton>
      </template>
      <UiCard>
        <NForm
          :model="profileForm"
          label-placement="left"
          label-width="180"
          size="small"
          class="profile-form"
        >
          <NGrid :cols="2" x-gap="16">
            <NGi>
              <NFormItem label="nginx_conf_dir">
                <NInput v-model:value="profileForm.nginx_conf_dir" :placeholder="profile?.effective.nginx_conf_dir" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="nginx_conf_path">
                <NInput v-model:value="profileForm.nginx_conf_path" :placeholder="profile?.effective.nginx_conf_path" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="sites_available_dir">
                <NInput v-model:value="profileForm.sites_available_dir" :placeholder="profile?.effective.sites_available_dir" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="sites_enabled_dir">
                <NInput v-model:value="profileForm.sites_enabled_dir" :placeholder="profile?.effective.sites_enabled_dir" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="app_locations_dir">
                <NInput v-model:value="profileForm.app_locations_dir" :placeholder="profile?.effective.app_locations_dir" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="streams_conf">
                <NInput v-model:value="profileForm.streams_conf" :placeholder="profile?.effective.streams_conf" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="cert_dir">
                <NInput v-model:value="profileForm.cert_dir" :placeholder="profile?.effective.cert_dir" />
              </NFormItem>
            </NGi>
            <NGi>
              <NFormItem label="hub_site_name">
                <NInput v-model:value="profileForm.hub_site_name" :placeholder="profile?.effective.hub_site_name" />
              </NFormItem>
            </NGi>
            <NGi :span="2">
              <NFormItem label="test_cmd">
                <NInput v-model:value="profileForm.test_cmd" :placeholder="profile?.effective.test_cmd" />
              </NFormItem>
            </NGi>
            <NGi :span="2">
              <NFormItem label="reload_cmd">
                <NInput v-model:value="profileForm.reload_cmd" :placeholder="profile?.effective.reload_cmd" />
              </NFormItem>
            </NGi>
          </NGrid>
          <div class="profile-tips">
            空字段 = 走默认（即 placeholder 显示的值）。改路径/命令前请先确认 nginx 真的部署在该位置。
          </div>
        </NForm>
      </UiCard>

      <UiCard v-if="profile" class="probe-card">
        <div class="probe-header">
          <div>
            <span class="probe-label">nginx 版本</span>
            <span class="probe-value">{{ profile.version || '未探测' }}</span>
          </div>
          <div>
            <span class="probe-label">可执行路径</span>
            <span class="probe-value">{{ profile.binary_path || '—' }}</span>
          </div>
          <div>
            <span class="probe-label">--prefix</span>
            <span class="probe-value">{{ profile.build_prefix || '—' }}</span>
          </div>
          <div>
            <span class="probe-label">--conf-path</span>
            <span class="probe-value">{{ profile.build_conf || '—' }}</span>
          </div>
          <div>
            <span class="probe-label">最近探测</span>
            <span class="probe-value">{{ profile.last_probe_at ? new Date(profile.last_probe_at).toLocaleString() : '—' }}</span>
          </div>
        </div>
        <div v-if="profile.modules?.length" class="modules">
          <span class="probe-label">编译模块</span>
          <UiBadge v-for="m in profile.modules" :key="m" tone="neutral" class="mod">{{ m }}</UiBadge>
        </div>
      </UiCard>
    </UiSection>

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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  NDataTable, NModal, NDrawer, NDrawerContent, NForm, NFormItem,
  NInput, NInputNumber, NSelect, NPopconfirm, NGrid, NGi, useMessage,
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
import {
  getSites, createSite, getSiteConfig, putSiteConfig,
  deleteSite, enableSite, disableSite, nginxReload, nginxRestart,
  accessLogsWsUrl, errorLogsWsUrl,
  getNginxProfile, putNginxProfile, probeNginxProfile,
} from '@/api/nginx'
import type { SiteItem, NginxProfile, NginxProfileUpdate } from '@/api/nginx'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiTabs from '@/components/ui/UiTabs.vue'

const route = useRoute()
const auth = useAuthStore()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))
const sites = ref<SiteItem[]>([])
const loading = ref(false)
const reloading = ref(false)

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
    title: '状态', key: 'enabled', width: 90,
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
    message.success('配置已保存（nginx -t 验证通过）')
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

async function loadSites() {
  loading.value = true
  try { sites.value = await getSites(serverId.value) } finally { loading.value = false }
}

// ── Phase Nginx-P3: NginxProfile（多实例 + nginx -V） ────────────────────────
const profile = ref<NginxProfile | null>(null)
const profileSaving = ref(false)
const probing = ref(false)
const profileForm = ref<NginxProfileUpdate>({
  nginx_conf_dir: '', sites_available_dir: '', sites_enabled_dir: '',
  app_locations_dir: '', streams_conf: '', cert_dir: '',
  nginx_conf_path: '', hub_site_name: '', test_cmd: '', reload_cmd: '',
})

function fillForm(p: NginxProfile) {
  profileForm.value = {
    nginx_conf_dir: p.nginx_conf_dir, sites_available_dir: p.sites_available_dir,
    sites_enabled_dir: p.sites_enabled_dir, app_locations_dir: p.app_locations_dir,
    streams_conf: p.streams_conf, cert_dir: p.cert_dir,
    nginx_conf_path: p.nginx_conf_path, hub_site_name: p.hub_site_name,
    test_cmd: p.test_cmd, reload_cmd: p.reload_cmd,
  }
}

async function loadProfile() {
  try {
    const p = await getNginxProfile(serverId.value)
    profile.value = p
    fillForm(p)
  } catch (e: any) { showApiError(e, '加载 nginx profile 失败') }
}

async function saveProfile() {
  profileSaving.value = true
  try {
    const p = await putNginxProfile(serverId.value, profileForm.value)
    profile.value = p
    fillForm(p)
    message.success('已保存')
  } catch (e: any) {
    showApiError(e, '保存失败')
  } finally { profileSaving.value = false }
}

async function doProbe() {
  probing.value = true
  try {
    const p = await probeNginxProfile(serverId.value)
    profile.value = p
    fillForm(p)
    message.success(`探测完成：nginx ${p.version || '<未知>'}`)
  } catch (e: any) {
    showApiError(e, '探测失败')
  } finally { probing.value = false }
}

onMounted(() => { loadSites(); loadProfile() })
onBeforeUnmount(() => { closeLogs(); editorView?.destroy() })
</script>

<style scoped>
.ng-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
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
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
