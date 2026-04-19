<template>
  <div class="system-page">
    <div class="page-toolbar">
      <el-button :icon="Refresh" :loading="loading" @click="refreshAll">刷新</el-button>
    </div>

    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="防火墙" name="firewall">
        <div class="tab-toolbar">
          <el-tag type="info" v-if="firewallType">{{ firewallType }}</el-tag>
          <el-button type="primary" size="small" @click="openAddRule">添加规则</el-button>
        </div>
        <el-table :data="firewallRules" v-loading="loading" style="width:100%">
          <el-table-column label="#" prop="index" width="60" />
          <el-table-column label="规则" prop="rule" min-width="280" show-overflow-tooltip />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-popconfirm title="确认删除该规则？" @confirm="delRule(row)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="Cron 任务" name="cron">
        <div class="tab-toolbar">
          <el-button type="primary" size="small" @click="openCronAdd">添加任务</el-button>
        </div>
        <el-table :data="cronJobs" v-loading="loading" style="width:100%">
          <el-table-column label="Cron 表达式" prop="expr" width="160" />
          <el-table-column label="命令" prop="cmd" min-width="260" show-overflow-tooltip />
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="openCronEdit(row)">编辑</el-button>
              <el-popconfirm title="确认删除该任务？" @confirm="delCron(row)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="进程" name="processes">
        <div class="tab-toolbar">
          <el-button size="small" :icon="Refresh" @click="loadProcesses">刷新</el-button>
        </div>
        <el-table :data="processes" v-loading="procLoading" style="width:100%">
          <el-table-column label="PID" prop="pid" width="80" />
          <el-table-column label="用户" prop="user" width="100" />
          <el-table-column label="CPU%" width="80">
            <template #default="{ row }">{{ row.cpu.toFixed(1) }}%</template>
          </el-table-column>
          <el-table-column label="内存%" width="80">
            <template #default="{ row }">{{ row.mem.toFixed(1) }}%</template>
          </el-table-column>
          <el-table-column label="命令" prop="command" min-width="240" show-overflow-tooltip />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-popconfirm :title="`确认 kill -9 PID ${row.pid}？`" @confirm="killProc(row)">
                <template #reference>
                  <el-button size="small" type="danger">Kill</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="系统服务" name="services">
        <div class="tab-toolbar">
          <el-input v-model="svcFilter" placeholder="过滤服务名" style="width:200px" clearable />
          <el-button size="small" :icon="Refresh" @click="loadServices">刷新</el-button>
        </div>
        <el-table :data="filteredServices" v-loading="svcLoading" style="width:100%">
          <el-table-column label="服务名" prop="unit" min-width="220" show-overflow-tooltip />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="activeTag(row.active)" size="small">{{ row.active }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="自启" width="70">
            <template #default="{ row }">
              <el-tag :type="row.load === 'loaded' ? 'success' : 'info'" size="small">{{ row.load }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="说明" prop="description" min-width="180" show-overflow-tooltip />
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="success" @click="svcAction(row, 'start')">启动</el-button>
              <el-button size="small" type="warning" @click="svcAction(row, 'stop')">停止</el-button>
              <el-button size="small" @click="svcAction(row, 'restart')">重启</el-button>
              <el-button size="small" @click="openSvcLogs(row)">日志</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="ruleVisible" title="添加防火墙规则" width="440px">
      <el-form :model="ruleForm" label-width="80px">
        <el-form-item label="端口">
          <el-input v-model="ruleForm.port" placeholder="如 80 或 8000:8100" />
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="ruleForm.proto">
            <el-option label="tcp" value="tcp" />
            <el-option label="udp" value="udp" />
          </el-select>
        </el-form-item>
        <el-form-item label="动作">
          <el-select v-model="ruleForm.action">
            <el-option label="允许 (allow)" value="allow" />
            <el-option label="拒绝 (deny)" value="deny" />
          </el-select>
        </el-form-item>
        <el-form-item label="来源 IP">
          <el-input v-model="ruleForm.from" placeholder="留空表示 Anywhere" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmAddRule">添加</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="cronVisible" :title="cronEditIndex === -1 ? '添加 Cron 任务' : '编辑 Cron 任务'" width="480px">
      <el-form :model="cronForm" label-width="100px">
        <el-form-item label="Cron 表达式">
          <el-input v-model="cronForm.expr" placeholder="*/5 * * * *" />
        </el-form-item>
        <el-form-item label="执行命令">
          <el-input v-model="cronForm.cmd" placeholder="/path/to/script.sh" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cronVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCron">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="svcLogsVisible" :title="`服务日志 — ${svcLogsName}`" size="60%" direction="rtl" @closed="onSvcLogsClosed">
      <div ref="svcLogsEl" class="logs-terminal" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
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

function activeTag(active: string) {
  return ({ active: 'success', failed: 'danger', inactive: 'info' } as Record<string, string>)[active] ?? 'warning'
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
  try { cronJobs.value = await getCronJobs(serverId.value) }
  finally { loading.value = false }
}

async function loadProcesses() {
  procLoading.value = true
  try { processes.value = await getProcesses(serverId.value) }
  finally { procLoading.value = false }
}

async function loadServices() {
  svcLoading.value = true
  try { services.value = await getServices(serverId.value) }
  finally { svcLoading.value = false }
}

const ruleVisible = ref(false)
const ruleForm = ref({ port: '', proto: 'tcp', action: 'allow', from: '' })

function openAddRule() {
  ruleForm.value = { port: '', proto: 'tcp', action: 'allow', from: '' }
  ruleVisible.value = true
}

async function confirmAddRule() {
  if (!ruleForm.value.port) return
  try {
    await addFirewallRule(serverId.value, ruleForm.value)
    ElMessage.success('规则已添加')
    ruleVisible.value = false
    await loadFirewall()
  } catch { ElMessage.error('添加失败') }
}

async function delRule(row: FirewallRule) {
  try {
    await deleteFirewallRule(serverId.value, String(row.index))
    ElMessage.success('规则已删除')
    await loadFirewall()
  } catch { ElMessage.error('删除失败') }
}

const cronVisible = ref(false)
const cronEditIndex = ref(-1)
const cronForm = ref({ expr: '', cmd: '' })

function openCronAdd() { cronEditIndex.value = -1; cronForm.value = { expr: '', cmd: '' }; cronVisible.value = true }

function openCronEdit(row: CronJob) {
  cronEditIndex.value = row.index
  cronForm.value = { expr: row.expr, cmd: row.cmd }
  cronVisible.value = true
}

async function confirmCron() {
  try {
    if (cronEditIndex.value === -1) {
      await addCronJob(serverId.value, cronForm.value.expr, cronForm.value.cmd)
    } else {
      await updateCronJob(serverId.value, cronEditIndex.value, cronForm.value.expr, cronForm.value.cmd)
    }
    ElMessage.success('保存成功')
    cronVisible.value = false
    await loadCron()
  } catch { ElMessage.error('保存失败') }
}

async function delCron(row: CronJob) {
  try {
    await deleteCronJob(serverId.value, row.index)
    ElMessage.success('已删除')
    await loadCron()
  } catch { ElMessage.error('删除失败') }
}

async function killProc(row: ProcessItem) {
  try {
    await killProcess(serverId.value, row.pid)
    ElMessage.success(`PID ${row.pid} 已终止`)
    await loadProcesses()
  } catch { ElMessage.error('Kill 失败') }
}

async function svcAction(row: ServiceItem, action: string) {
  try {
    await serviceAction(serverId.value, row.unit, action)
    ElMessage.success('操作成功')
    await loadServices()
  } catch { ElMessage.error('操作失败') }
}

const svcLogsVisible = ref(false)
const svcLogsName = ref('')
const svcLogsEl = ref<HTMLDivElement>()
let svcLogsTerm: Terminal | null = null
let svcLogsWs: WebSocket | null = null

function openSvcLogs(row: ServiceItem) {
  svcLogsName.value = row.unit
  svcLogsVisible.value = true
  nextTick(() => initSvcLogs(row.unit))
}

function initSvcLogs(unit: string) {
  if (!svcLogsEl.value) return
  svcLogsTerm?.dispose()
  svcLogsTerm = new Terminal({ theme: { background: '#1a1a2e' }, convertEol: true, fontSize: 13 })
  const fit = new FitAddon()
  svcLogsTerm.loadAddon(fit)
  svcLogsTerm.open(svcLogsEl.value)
  fit.fit()
  svcLogsWs?.close()
  svcLogsWs = new WebSocket(serviceLogsWsUrl(serverId.value, unit, auth.token))
  svcLogsWs.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'output') svcLogsTerm?.writeln(msg.data)
    } catch { /* ignore */ }
  }
}

function onSvcLogsClosed() { svcLogsWs?.close(); svcLogsWs = null; svcLogsTerm?.dispose(); svcLogsTerm = null }

onMounted(() => loadFirewall())
onBeforeUnmount(() => { svcLogsWs?.close(); svcLogsTerm?.dispose() })
</script>

<style scoped>
.system-page { padding: 20px; }
.page-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
.tab-toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.logs-terminal { width: 100%; height: calc(100vh - 120px); background: #1a1a2e; border-radius: 4px; overflow: hidden; }
</style>
