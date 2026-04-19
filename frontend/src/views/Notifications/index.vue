<template>
  <div class="notifications-page">
    <el-tabs v-model="activeTab">
      <!-- ── Alert Rules ─────────────────────────────────────── -->
      <el-tab-pane label="告警规则" name="rules">
        <div class="tab-toolbar">
          <el-button type="primary" size="small" @click="openCreateRule">添加规则</el-button>
          <el-button size="small" :loading="rulesLoading" @click="loadRules">刷新</el-button>
        </div>
        <el-table :data="rules" v-loading="rulesLoading" size="small">
          <el-table-column label="服务器" width="140">
            <template #default="{ row }">{{ row.server_id ? serverName(row.server_id) : '所有服务器' }}</template>
          </el-table-column>
          <el-table-column label="指标" width="90">
            <template #default="{ row }">
              <el-tag size="small">{{ metricLabel(row.metric) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="条件" width="160">
            <template #default="{ row }">
              {{ row.metric !== 'offline' ? `${row.operator === 'gt' ? '>' : '<'} ${row.threshold}%` : '离线' }}
            </template>
          </el-table-column>
          <el-table-column label="持续次数" width="90" prop="duration" />
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" @change="toggleRule(row)" />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-popconfirm title="确认删除此规则？" @confirm="deleteRuleItem(row)">
                <template #reference>
                  <el-button :icon="Delete" circle size="small" type="danger" plain />
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ── Notify Channels ────────────────────────────────── -->
      <el-tab-pane label="通知渠道" name="channels">
        <div class="tab-toolbar">
          <el-button type="primary" size="small" @click="openCreateChannel">添加渠道</el-button>
          <el-button size="small" :loading="channelsLoading" @click="loadChannels">刷新</el-button>
        </div>
        <el-table :data="channels" v-loading="channelsLoading" size="small">
          <el-table-column label="名称" prop="name" min-width="140" />
          <el-table-column label="类型" width="140">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ channelTypeLabel(row.type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="消息模板" prop="template" min-width="200" show-overflow-tooltip />
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" @change="toggleChannel(row)" />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" align="center">
            <template #default="{ row }">
              <el-button size="small" @click="doTestChannel(row)">测试</el-button>
              <el-popconfirm title="确认删除此渠道？" @confirm="deleteChannelItem(row)">
                <template #reference>
                  <el-button :icon="Delete" circle size="small" type="danger" plain />
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ── Alert Events ───────────────────────────────────── -->
      <el-tab-pane label="告警历史" name="events">
        <div class="tab-toolbar">
          <el-button size="small" :loading="eventsLoading" @click="loadEvents">刷新</el-button>
          <el-popconfirm title="确认清理30天前的历史记录？" @confirm="doClearEvents">
            <template #reference>
              <el-button size="small" type="warning">清理旧记录</el-button>
            </template>
          </el-popconfirm>
        </div>
        <el-table :data="events" v-loading="eventsLoading" size="small">
          <el-table-column label="时间" width="160">
            <template #default="{ row }">{{ formatTime(row.sent_at) }}</template>
          </el-table-column>
          <el-table-column label="服务器" width="130">
            <template #default="{ row }">{{ serverName(row.server_id) }}</template>
          </el-table-column>
          <el-table-column label="消息" prop="message" min-width="300" show-overflow-tooltip />
          <el-table-column label="值" width="80">
            <template #default="{ row }">{{ row.value ? row.value.toFixed(1) : '—' }}</template>
          </el-table-column>
        </el-table>
        <div class="pagination">
          <el-pagination
            v-model:current-page="eventsPage"
            :page-size="50"
            :total="eventsTotal"
            layout="total, prev, pager, next"
            @current-change="loadEvents"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- Add Rule dialog -->
    <el-dialog v-model="ruleVisible" title="添加告警规则" width="440px">
      <el-form :model="ruleForm" label-width="90px" size="small">
        <el-form-item label="服务器">
          <el-select v-model="ruleForm.server_id" placeholder="所有服务器" style="width:100%" clearable>
            <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="指标">
          <el-select v-model="ruleForm.metric" style="width:100%">
            <el-option label="CPU 使用率" value="cpu" />
            <el-option label="内存使用率" value="mem" />
            <el-option label="磁盘使用率" value="disk" />
            <el-option label="服务器离线" value="offline" />
          </el-select>
        </el-form-item>
        <template v-if="ruleForm.metric !== 'offline'">
          <el-form-item label="条件">
            <el-radio-group v-model="ruleForm.operator">
              <el-radio value="gt">大于 (&gt;)</el-radio>
              <el-radio value="lt">小于 (&lt;)</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="阈值 (%)">
            <el-input-number v-model="ruleForm.threshold" :min="0" :max="100" style="width:100%" />
          </el-form-item>
        </template>
        <el-form-item label="持续次数">
          <el-input-number v-model="ruleForm.duration" :min="1" :max="10" style="width:100%" />
          <div class="form-hint">连续触发 N 次才发告警，防止抖动</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleVisible = false">取消</el-button>
        <el-button type="primary" :loading="ruleSaving" @click="confirmCreateRule">创建</el-button>
      </template>
    </el-dialog>

    <!-- Add Channel dialog -->
    <el-dialog v-model="channelVisible" title="添加通知渠道" width="500px">
      <el-form :model="channelForm" label-width="90px" size="small">
        <el-form-item label="名称">
          <el-input v-model="channelForm.name" placeholder="我的企微机器人" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="channelForm.type" style="width:100%">
            <el-option label="企业微信机器人" value="webhook_wechat" />
            <el-option label="钉钉机器人" value="webhook_dingtalk" />
            <el-option label="自定义 Webhook" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item label="Webhook URL">
          <el-input v-model="channelForm.url" placeholder="https://qyapi.weixin.qq.com/..." />
        </el-form-item>
        <el-form-item label="消息模板">
          <el-input v-model="channelForm.template" type="textarea" :rows="3"
            placeholder="留空使用默认模板，可用变量: {{.Server}} {{.Metric}} {{.Value}} {{.Time}}" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelVisible = false">取消</el-button>
        <el-button type="primary" :loading="channelSaving" @click="confirmCreateChannel">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import { getServers } from '@/api/servers'
import {
  listRules, createRule, updateRule, deleteRule,
  listEvents, clearEvents,
  listChannels, createChannel, updateChannel, deleteChannel, testChannel,
} from '@/api/alerts'
import type { AlertRule, AlertEvent, NotifyChannel } from '@/api/alerts'
import type { Server } from '@/types/api'

const activeTab = ref('rules')
const servers = ref<Server[]>([])

function serverName(id: number) {
  return servers.value.find(s => s.id === id)?.name ?? `#${id}`
}
function metricLabel(m: string) {
  return { cpu: 'CPU', mem: '内存', disk: '磁盘', offline: '离线' }[m] ?? m
}
function channelTypeLabel(t: string) {
  return { webhook_wechat: '企业微信', webhook_dingtalk: '钉钉', custom: '自定义' }[t] ?? t
}
function formatTime(t: string) { return dayjs(t).format('MM-DD HH:mm:ss') }

// ── Rules ────────────────────────────────────────────────────────
const rules = ref<AlertRule[]>([])
const rulesLoading = ref(false)
const ruleVisible = ref(false)
const ruleSaving = ref(false)
const ruleForm = ref({ server_id: undefined as number | undefined, metric: 'cpu', operator: 'gt', threshold: 90, duration: 1 })

async function loadRules() {
  rulesLoading.value = true
  try { rules.value = await listRules() } finally { rulesLoading.value = false }
}

function openCreateRule() {
  ruleForm.value = { server_id: undefined, metric: 'cpu', operator: 'gt', threshold: 90, duration: 1 }
  ruleVisible.value = true
}

async function confirmCreateRule() {
  ruleSaving.value = true
  try {
    await createRule({
      server_id: ruleForm.value.server_id ?? 0,
      metric: ruleForm.value.metric,
      operator: ruleForm.value.operator,
      threshold: ruleForm.value.threshold,
      duration: ruleForm.value.duration,
    })
    ElMessage.success('规则已创建')
    ruleVisible.value = false
    await loadRules()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '创建失败') }
  finally { ruleSaving.value = false }
}

async function toggleRule(row: AlertRule) {
  await updateRule(row.id, { enabled: row.enabled })
}

async function deleteRuleItem(row: AlertRule) {
  await deleteRule(row.id)
  await loadRules()
}

// ── Channels ─────────────────────────────────────────────────────
const channels = ref<NotifyChannel[]>([])
const channelsLoading = ref(false)
const channelVisible = ref(false)
const channelSaving = ref(false)
const channelForm = ref({ name: '', type: 'webhook_wechat', url: '', template: '' })

async function loadChannels() {
  channelsLoading.value = true
  try { channels.value = await listChannels() } finally { channelsLoading.value = false }
}

function openCreateChannel() {
  channelForm.value = { name: '', type: 'webhook_wechat', url: '', template: '' }
  channelVisible.value = true
}

async function confirmCreateChannel() {
  if (!channelForm.value.name || !channelForm.value.url) {
    ElMessage.warning('请填写名称和 URL')
    return
  }
  channelSaving.value = true
  try {
    await createChannel(channelForm.value)
    ElMessage.success('渠道已添加')
    channelVisible.value = false
    await loadChannels()
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '创建失败') }
  finally { channelSaving.value = false }
}

async function toggleChannel(row: NotifyChannel) {
  await updateChannel(row.id, { enabled: row.enabled })
}

async function doTestChannel(row: NotifyChannel) {
  try {
    await testChannel(row.id)
    ElMessage.success('测试消息已发送')
  } catch (e: any) { ElMessage.error(e?.response?.data?.msg ?? '发送失败') }
}

async function deleteChannelItem(row: NotifyChannel) {
  await deleteChannel(row.id)
  await loadChannels()
}

// ── Events ────────────────────────────────────────────────────────
const events = ref<AlertEvent[]>([])
const eventsLoading = ref(false)
const eventsPage = ref(1)
const eventsTotal = ref(0)

async function loadEvents() {
  eventsLoading.value = true
  try {
    const res = await listEvents(eventsPage.value, 50)
    events.value = res.events
    eventsTotal.value = res.total
  } finally { eventsLoading.value = false }
}

async function doClearEvents() {
  await clearEvents()
  ElMessage.success('已清理')
  await loadEvents()
}

onMounted(async () => {
  servers.value = await getServers()
  await Promise.all([loadRules(), loadChannels(), loadEvents()])
})
</script>

<style scoped>
.notifications-page { padding: 20px; }
.tab-toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.pagination { margin-top: 12px; display: flex; justify-content: flex-end; }
.form-hint { font-size: 11px; color: #909399; margin-top: 4px; }
</style>
