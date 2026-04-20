<template>
  <div class="page-container">
    <div class="section-block">
      <div class="section-title">
        <span>系统管理</span>
        <t-button size="small" variant="outline" :loading="loading" @click="refreshAll">
          <template #icon><refresh-icon /></template>
          刷新全部
        </t-button>
      </div>

      <t-tabs :value="activeTab" @change="val => { activeTab = val as string; onTabChange(val as string) }">
        <!-- 防火墙 -->
        <t-tab-panel value="firewall" label="防火墙">
          <div class="tab-toolbar">
            <div class="toolbar-left">
              <t-tag v-if="firewallType" theme="default" variant="light" size="small">{{ firewallType }}</t-tag>
            </div>
            <t-button theme="primary" size="small" @click="openAddRule">添加规则</t-button>
          </div>
          <t-table :data="firewallRules" :columns="fwColumns" :loading="loading" row-key="index" bordered size="small">
            <template #operations="{ row }">
              <t-popconfirm content="确认删除该规则？" @confirm="delRule(row)">
                <t-button theme="danger" size="small" variant="text">删除</t-button>
              </t-popconfirm>
            </template>
          </t-table>
        </t-tab-panel>

        <!-- Cron 任务 -->
        <t-tab-panel value="cron" label="Cron 任务">
          <div class="tab-toolbar">
            <div class="toolbar-left" />
            <t-button theme="primary" size="small" @click="openCronAdd">添加任务</t-button>
          </div>
          <t-table :data="cronJobs" :columns="cronColumns" :loading="loading" row-key="index" bordered size="small">
            <template #operations="{ row }">
              <t-space size="small">
                <t-button size="small" variant="text" @click="openCronEdit(row)">编辑</t-button>
                <t-popconfirm content="确认删除该任务？" @confirm="delCron(row)">
                  <t-button theme="danger" size="small" variant="text">删除</t-button>
                </t-popconfirm>
              </t-space>
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 进程 -->
        <t-tab-panel value="processes" label="进程">
          <div class="tab-toolbar">
            <div class="toolbar-left" />
            <t-button size="small" variant="outline" @click="loadProcesses">
              <template #icon><refresh-icon /></template>
              刷新
            </t-button>
          </div>
          <t-table :data="processes" :columns="procColumns" :loading="procLoading" row-key="pid" bordered size="small">
            <template #cpu="{ row }">{{ row.cpu.toFixed(1) }}%</template>
            <template #mem="{ row }">{{ row.mem.toFixed(1) }}%</template>
            <template #operations="{ row }">
              <t-popconfirm :content="`确认 kill -9 PID ${row.pid}？`" @confirm="killProc(row)">
                <t-button theme="danger" size="small" variant="text">Kill</t-button>
              </t-popconfirm>
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 系统服务 -->
        <t-tab-panel value="services" label="系统服务">
          <div class="tab-toolbar">
            <div class="toolbar-left">
              <t-input v-model="svcFilter" placeholder="过滤服务名" size="small" class="filter-input" clearable />
              <t-button size="small" variant="outline" @click="loadServices">
                <template #icon><refresh-icon /></template>
              </t-button>
            </div>
          </div>
          <t-table :data="filteredServices" :columns="svcColumns" :loading="svcLoading" row-key="unit" bordered size="small">
            <template #active="{ row }">
              <t-tag :theme="activeTheme(row.active)" variant="light" size="small">{{ row.active }}</t-tag>
            </template>
            <template #load="{ row }">
              <t-tag :theme="row.load === 'loaded' ? 'success' : 'default'" variant="light" size="small">{{ row.load }}</t-tag>
            </template>
            <template #operations="{ row }">
              <t-space size="small">
                <t-button theme="success" size="small" variant="text" @click="svcAction(row, 'start')">启动</t-button>
                <t-button theme="warning" size="small" variant="text" @click="svcAction(row, 'stop')">停止</t-button>
                <t-button size="small" variant="text" @click="svcAction(row, 'restart')">重启</t-button>
                <t-button size="small" variant="text" @click="openSvcLogs(row)">日志</t-button>
              </t-space>
            </template>
          </t-table>
        </t-tab-panel>
      </t-tabs>
    </div>

    <!-- 添加防火墙规则对话框 -->
    <t-dialog
      v-model:visible="ruleVisible"
      header="添加防火墙规则"
      width="440px"
      confirm-btn="添加"
      @confirm="confirmAddRule"
    >
      <t-form :data="ruleForm" label-width="80px" colon>
        <t-form-item label="端口">
          <t-input v-model="ruleForm.port" placeholder="如 80 或 8000:8100" />
        </t-form-item>
        <t-form-item label="协议">
          <t-select v-model="ruleForm.proto" class="full-width">
            <t-option label="tcp" value="tcp" />
            <t-option label="udp" value="udp" />
          </t-select>
        </t-form-item>
        <t-form-item label="动作">
          <t-select v-model="ruleForm.action" class="full-width">
            <t-option label="允许 (allow)" value="allow" />
            <t-option label="拒绝 (deny)" value="deny" />
          </t-select>
        </t-form-item>
        <t-form-item label="来源 IP">
          <t-input v-model="ruleForm.from" placeholder="留空表示 Anywhere" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- Cron 对话框 -->
    <t-dialog
      v-model:visible="cronVisible"
      :header="cronEditIndex === -1 ? '添加 Cron 任务' : '编辑 Cron 任务'"
      width="480px"
      confirm-btn="保存"
      @confirm="confirmCron"
    >
      <t-form :data="cronForm" label-width="100px" colon>
        <t-form-item label="Cron 表达式">
          <t-input v-model="cronForm.expr" placeholder="*/5 * * * *" />
        </t-form-item>
        <t-form-item label="执行命令">
          <t-input v-model="cronForm.cmd" placeholder="/path/to/script.sh" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 服务日志抽屉 -->
    <t-drawer v-model:visible="svcLogsVisible" :header="`服务日志 — ${svcLogsName}`" size="60%" @close="onSvcLogsClosed">
      <div ref="svcLogsEl" class="logs-terminal" />
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
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

const route = useRoute()
const auth = useAuthStore()
const serverId = computed(() => Number(route.params.serverId))
const activeTab = ref('firewall')
const loading = ref(false)

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

const fwColumns = [
  { colKey: 'index', title: '#', width: 60 },
  { colKey: 'rule', title: '规则', minWidth: 280, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 80, fixed: 'right' as const },
]
const cronColumns = [
  { colKey: 'expr', title: 'Cron 表达式', width: 160 },
  { colKey: 'cmd', title: '命令', minWidth: 260, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 140, fixed: 'right' as const },
]
const procColumns = [
  { colKey: 'pid', title: 'PID', width: 80 },
  { colKey: 'user', title: '用户', width: 100 },
  { colKey: 'cpu', title: 'CPU%', width: 80 },
  { colKey: 'mem', title: '内存%', width: 80 },
  { colKey: 'command', title: '命令', minWidth: 240, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 80, fixed: 'right' as const },
]
const svcColumns = [
  { colKey: 'unit', title: '服务名', minWidth: 220, ellipsis: true },
  { colKey: 'active', title: '状态', width: 90 },
  { colKey: 'load', title: '自启', width: 70 },
  { colKey: 'description', title: '说明', minWidth: 180, ellipsis: true },
  { colKey: 'operations', title: '操作', width: 260, fixed: 'right' as const },
]

function activeTheme(active: string) {
  return ({ active: 'success', failed: 'danger', inactive: 'default' } as Record<string, string>)[active] ?? 'warning'
}

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
    MessagePlugin.success('规则已添加'); ruleVisible.value = false; await loadFirewall()
  } catch { MessagePlugin.error('添加失败') }
}

async function delRule(row: FirewallRule) {
  try { await deleteFirewallRule(serverId.value, String(row.index)); MessagePlugin.success('规则已删除'); await loadFirewall() }
  catch { MessagePlugin.error('删除失败') }
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
    MessagePlugin.success('保存成功'); cronVisible.value = false; await loadCron()
  } catch { MessagePlugin.error('保存失败') }
}

async function delCron(row: CronJob) {
  try { await deleteCronJob(serverId.value, row.index); MessagePlugin.success('已删除'); await loadCron() }
  catch { MessagePlugin.error('删除失败') }
}

async function killProc(row: ProcessItem) {
  try { await killProcess(serverId.value, row.pid); MessagePlugin.success(`PID ${row.pid} 已终止`); await loadProcesses() }
  catch { MessagePlugin.error('Kill 失败') }
}

async function svcAction(row: ServiceItem, action: string) {
  try { await serviceAction(serverId.value, row.unit, action); MessagePlugin.success('操作成功'); await loadServices() }
  catch { MessagePlugin.error('操作失败') }
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
  svcLogsTerm = new Terminal({ theme: { background: '#1a2332' }, convertEol: true, fontSize: 13 })
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
.filter-input { width: 200px; }
.full-width { width: 100%; }

.logs-terminal {
  width: 100%;
  height: calc(100vh - 120px);
  background: #1a2332;
  border-radius: 4px;
  overflow: hidden;
}

:deep(.t-table) { font-size: 13px; }
:deep(.t-tab-panel) { padding: 0; }
</style>
