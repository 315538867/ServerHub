<template>
  <div class="sys-page">
    <UiCard padding="none">
      <div class="sys-head">
        <UiTabs :items="tabItems" :model-value="activeTab" @change="val => { activeTab = String(val); onTabChange(String(val)) }" />
        <UiButton variant="secondary" size="sm" :loading="loading" @click="refreshAll">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </div>
      <div class="sys-body">
        <div v-if="activeTab === 'firewall'">
          <div class="sys-toolbar">
            <div>
              <UiBadge v-if="firewallType" tone="neutral">{{ firewallType }}</UiBadge>
            </div>
            <UiButton variant="primary" size="sm" @click="openAddRule">添加规则</UiButton>
          </div>
          <NDataTable
            :columns="fwColumns"
            :data="firewallRules"
            :loading="loading"
            :row-key="(row: FirewallRule) => row.index"
            size="small"
            :bordered="false"
          />
        </div>

        <div v-else-if="activeTab === 'cron'">
          <div class="sys-toolbar">
            <div />
            <UiButton variant="primary" size="sm" @click="openCronAdd">添加任务</UiButton>
          </div>
          <NDataTable
            :columns="cronColumns"
            :data="cronJobs"
            :loading="loading"
            :row-key="(row: CronJob) => row.index"
            size="small"
            :bordered="false"
          />
        </div>

        <div v-else-if="activeTab === 'processes'">
          <div class="sys-toolbar">
            <div />
            <UiButton variant="secondary" size="sm" :loading="procLoading" @click="loadProcesses">
              <template #icon><RefreshCw :size="14" /></template>
              刷新
            </UiButton>
          </div>
          <NDataTable
            :columns="procColumns"
            :data="processes"
            :loading="procLoading"
            :row-key="(row: ProcessItem) => row.pid"
            size="small"
            :bordered="false"
          />
        </div>

        <div v-else-if="activeTab === 'services'">
          <div class="sys-toolbar">
            <NInput v-model:value="svcFilter" placeholder="过滤服务名" size="small" clearable class="filter-inp">
              <template #prefix><Search :size="14" /></template>
            </NInput>
            <UiButton variant="secondary" size="sm" :loading="svcLoading" @click="loadServices">
              <template #icon><RefreshCw :size="14" /></template>
              刷新
            </UiButton>
          </div>
          <NDataTable
            :columns="svcColumns"
            :data="filteredServices"
            :loading="svcLoading"
            :row-key="(row: ServiceItem) => row.unit"
            size="small"
            :bordered="false"
          />
        </div>
      </div>
    </UiCard>

    <NModal
      v-model:show="ruleVisible"
      preset="card"
      title="添加防火墙规则"
      style="width: 440px"
      :bordered="false"
    >
      <NForm :model="ruleForm" label-placement="left" label-width="80">
        <NFormItem label="端口">
          <NInput v-model:value="ruleForm.port" placeholder="如 80 或 8000:8100" />
        </NFormItem>
        <NFormItem label="协议">
          <NSelect v-model:value="ruleForm.proto" :options="[{label: 'tcp', value: 'tcp'}, {label: 'udp', value: 'udp'}]" />
        </NFormItem>
        <NFormItem label="动作">
          <NSelect v-model:value="ruleForm.action" :options="[{label: '允许 (allow)', value: 'allow'}, {label: '拒绝 (deny)', value: 'deny'}]" />
        </NFormItem>
        <NFormItem label="来源 IP">
          <NInput v-model:value="ruleForm.from" placeholder="留空表示 Anywhere" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="ruleVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmAddRule">添加</UiButton>
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="cronVisible"
      preset="card"
      :title="cronEditIndex === -1 ? '添加 Cron 任务' : '编辑 Cron 任务'"
      style="width: 480px"
      :bordered="false"
    >
      <NForm :model="cronForm" label-placement="left" label-width="100">
        <NFormItem label="Cron 表达式">
          <NInput v-model:value="cronForm.expr" placeholder="*/5 * * * *" />
        </NFormItem>
        <NFormItem label="执行命令">
          <NInput v-model:value="cronForm.cmd" placeholder="/path/to/script.sh" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="cronVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" @click="confirmCron">保存</UiButton>
        </div>
      </template>
    </NModal>

    <NDrawer v-model:show="svcLogsVisible" :width="720" @after-leave="onSvcLogsClosed">
      <NDrawerContent :title="`服务日志 — ${svcLogsName}`" :native-scrollbar="false">
        <div ref="svcLogsEl" class="logs-terminal" />
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  NDataTable, NModal, NDrawer, NDrawerContent, NForm, NFormItem,
  NInput, NSelect, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Search } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import {
  getFirewallRules, addFirewallRule, deleteFirewallRule,
  getCronJobs, addCronJob, updateCronJob, deleteCronJob,
  getProcesses, killProcess,
  getServices, serviceAction, serviceLogsWsUrl,
} from '@/api/system'
import type { FirewallRule, CronJob, ProcessItem, ServiceItem } from '@/api/system'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiTabs from '@/components/ui/UiTabs.vue'

const route = useRoute()
const auth = useAuthStore()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))
const activeTab = ref('firewall')
const loading = ref(false)

const tabItems = [
  { value: 'firewall', label: '防火墙' },
  { value: 'cron', label: 'Cron 任务' },
  { value: 'processes', label: '进程' },
  { value: 'services', label: '系统服务' },
]

const firewallRules = ref<FirewallRule[]>([])
const firewallType = ref('')
const cronJobs = ref<CronJob[]>([])
const processes = ref<ProcessItem[]>([])
const procLoading = ref(false)
const services = ref<ServiceItem[]>([])
const svcLoading = ref(false)
const svcFilter = ref('')
const filteredServices = computed(() =>
  svcFilter.value
    ? services.value.filter(s => s.unit.includes(svcFilter.value) || s.description.includes(svcFilter.value))
    : services.value
)

const fwColumns = computed<DataTableColumns<FirewallRule>>(() => [
  { title: '#', key: 'index', width: 60 },
  { title: '规则', key: 'rule', minWidth: 280, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 90, fixed: 'right' as const,
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => delRule(row),
      positiveText: '删除', negativeText: '取消',
    }, {
      trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
        () => h('span', { class: 'text-danger' }, '删除')),
      default: () => '确认删除该规则？',
    }),
  },
])

const cronColumns = computed<DataTableColumns<CronJob>>(() => [
  { title: 'Cron 表达式', key: 'expr', width: 160 },
  { title: '命令', key: 'cmd', minWidth: 260, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 150, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openCronEdit(row) }, () => '编辑'),
      h(NPopconfirm, {
        onPositiveClick: () => delCron(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除该任务？',
      }),
    ]),
  },
])

const procColumns = computed<DataTableColumns<ProcessItem>>(() => [
  { title: 'PID', key: 'pid', width: 80 },
  { title: '用户', key: 'user', width: 100 },
  { title: 'CPU%', key: 'cpu', width: 80, render: (row) => row.cpu.toFixed(1) + '%' },
  { title: '内存%', key: 'mem', width: 80, render: (row) => row.mem.toFixed(1) + '%' },
  { title: '命令', key: 'command', minWidth: 240, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 90, fixed: 'right' as const,
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => killProc(row),
      positiveText: '终止', negativeText: '取消',
    }, {
      trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
        () => h('span', { class: 'text-danger' }, '终止')),
      default: () => `确认强制终止进程 ${row.pid}？`,
    }),
  },
])

function activeTone(active: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (active === 'active') return 'success'
  if (active === 'failed') return 'danger'
  if (active === 'inactive') return 'neutral'
  return 'warning'
}
function activeText(active: string) {
  return ({ active: '运行中', inactive: '未运行', failed: '失败', activating: '启动中', deactivating: '停止中' } as Record<string, string>)[active] ?? active
}
function loadText(load: string) {
  return ({ loaded: '已加载', 'not-found': '未找到', masked: '已屏蔽', error: '错误' } as Record<string, string>)[load] ?? load
}

const svcColumns = computed<DataTableColumns<ServiceItem>>(() => [
  { title: '服务名', key: 'unit', minWidth: 220, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'active', width: 100,
    render: (row) => h(UiBadge, { tone: activeTone(row.active) }, () => activeText(row.active)),
  },
  {
    title: '自启', key: 'load', width: 90,
    render: (row) => h(UiBadge, { tone: row.load === 'loaded' ? 'success' : 'neutral' }, () => loadText(row.load)),
  },
  { title: '说明', key: 'description', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'ops', width: 260, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => svcAction(row, 'start') }, () => '启动'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => svcAction(row, 'stop') }, () => '停止'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => svcAction(row, 'restart') }, () => '重启'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openSvcLogs(row) }, () => '日志'),
    ]),
  },
])

async function onTabChange(tab: string) {
  if (tab === 'firewall' && firewallRules.value.length === 0) await loadFirewall()
  if (tab === 'cron' && cronJobs.value.length === 0) await loadCron()
  if (tab === 'processes' && processes.value.length === 0) await loadProcesses()
  if (tab === 'services' && services.value.length === 0) await loadServices()
}

async function refreshAll() {
  if (activeTab.value === 'firewall') await loadFirewall()
  else if (activeTab.value === 'cron') await loadCron()
  else if (activeTab.value === 'processes') await loadProcesses()
  else await loadServices()
}

async function loadFirewall() {
  loading.value = true
  try {
    const res = await getFirewallRules(serverId.value)
    firewallType.value = res.type
    firewallRules.value = res.rules
  } finally { loading.value = false }
}

async function loadCron() {
  loading.value = true
  try { cronJobs.value = await getCronJobs(serverId.value) } finally { loading.value = false }
}

async function loadProcesses() {
  procLoading.value = true
  try { processes.value = await getProcesses(serverId.value) } finally { procLoading.value = false }
}

async function loadServices() {
  svcLoading.value = true
  try { services.value = await getServices(serverId.value) } finally { svcLoading.value = false }
}

const ruleVisible = ref(false)
const ruleForm = ref({ port: '', proto: 'tcp', action: 'allow', from: '' })

function openAddRule() { ruleForm.value = { port: '', proto: 'tcp', action: 'allow', from: '' }; ruleVisible.value = true }

async function confirmAddRule() {
  if (!ruleForm.value.port) return
  try {
    await addFirewallRule(serverId.value, ruleForm.value)
    message.success('规则已添加'); ruleVisible.value = false; await loadFirewall()
  } catch { message.error('添加失败') }
}

async function delRule(row: FirewallRule) {
  try { await deleteFirewallRule(serverId.value, String(row.index)); message.success('规则已删除'); await loadFirewall() }
  catch { message.error('删除失败') }
}

const cronVisible = ref(false)
const cronEditIndex = ref(-1)
const cronForm = ref({ expr: '', cmd: '' })

function openCronAdd() { cronEditIndex.value = -1; cronForm.value = { expr: '', cmd: '' }; cronVisible.value = true }

function openCronEdit(row: CronJob) {
  cronEditIndex.value = row.index; cronForm.value = { expr: row.expr, cmd: row.cmd }; cronVisible.value = true
}

async function confirmCron() {
  try {
    if (cronEditIndex.value === -1) await addCronJob(serverId.value, cronForm.value.expr, cronForm.value.cmd)
    else await updateCronJob(serverId.value, cronEditIndex.value, cronForm.value.expr, cronForm.value.cmd)
    message.success('保存成功'); cronVisible.value = false; await loadCron()
  } catch { message.error('保存失败') }
}

async function delCron(row: CronJob) {
  try { await deleteCronJob(serverId.value, row.index); message.success('已删除'); await loadCron() }
  catch { message.error('删除失败') }
}

async function killProc(row: ProcessItem) {
  try { await killProcess(serverId.value, row.pid); message.success(`PID ${row.pid} 已终止`); await loadProcesses() }
  catch { message.error('终止失败') }
}

async function svcAction(row: ServiceItem, action: string) {
  try { await serviceAction(serverId.value, row.unit, action); message.success('操作成功'); await loadServices() }
  catch { message.error('操作失败') }
}

const svcLogsVisible = ref(false)
const svcLogsName = ref('')
const svcLogsEl = ref<HTMLDivElement>()
let svcLogsTerm: Terminal | null = null
let svcLogsWs: WebSocket | null = null

function openSvcLogs(row: ServiceItem) {
  svcLogsName.value = row.unit; svcLogsVisible.value = true
  nextTick(() => initSvcLogs(row.unit))
}

function initSvcLogs(unit: string) {
  if (!svcLogsEl.value) return
  svcLogsTerm?.dispose()
  svcLogsTerm = new Terminal({ theme: { background: '#0A0A0A', foreground: '#E4E4E7' }, convertEol: true, fontSize: 12 })
  const fit = new FitAddon()
  svcLogsTerm.loadAddon(fit); svcLogsTerm.open(svcLogsEl.value); fit.fit()
  svcLogsWs?.close()
  svcLogsWs = new WebSocket(serviceLogsWsUrl(serverId.value, unit, auth.token))
  svcLogsWs.onmessage = (e) => {
    try { const msg = JSON.parse(e.data); if (msg.type === 'output') svcLogsTerm?.writeln(msg.data) } catch { /* ignore */ }
  }
}

function onSvcLogsClosed() { svcLogsWs?.close(); svcLogsWs = null; svcLogsTerm?.dispose(); svcLogsTerm = null }

onMounted(() => loadFirewall())
onBeforeUnmount(() => { svcLogsWs?.close(); svcLogsTerm?.dispose() })
</script>

<style scoped>
.sys-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.sys-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--ui-border);
}

.sys-body { padding: var(--space-4); }

.sys-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}

.filter-inp { width: 220px; }
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

.logs-terminal {
  width: 100%;
  height: calc(100vh - 160px);
  background: #0A0A0A;
  border-radius: var(--radius-sm);
  overflow: hidden;
  padding: var(--space-2);
}

:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
