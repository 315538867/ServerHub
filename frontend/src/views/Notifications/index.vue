<template>
  <div class="page-container notifications-page">
    <!-- 告警规则 -->
    <div class="section-block">
      <div class="section-title">
        <span>告警规则</span>
        <div class="title-actions">
          <t-button theme="primary" size="small" @click="openCreateRule">
            <template #icon><add-icon /></template>
            添加规则
          </t-button>
          <t-button variant="outline" size="small" :loading="rulesLoading" @click="loadRules">刷新</t-button>
        </div>
      </div>
      <div class="block-body">
        <t-table :data="rules" :columns="ruleColumns" :loading="rulesLoading" row-key="id" size="small" stripe>
          <template #server="{ row }">{{ row.server_id ? serverName(row.server_id) : '所有服务器' }}</template>
          <template #metric="{ row }">
            <t-tag theme="default" variant="light" size="small">{{ metricLabel(row.metric) }}</t-tag>
          </template>
          <template #condition="{ row }">
            {{ row.metric !== 'offline' ? `${row.operator === 'gt' ? '>' : '<'} ${row.threshold}%` : '离线' }}
          </template>
          <template #enabled="{ row }">
            <t-switch v-model="row.enabled" size="small" @change="toggleRule(row)" />
          </template>
          <template #operations="{ row }">
            <t-popconfirm content="确认删除此规则？" @confirm="deleteRuleItem(row)">
              <t-link theme="danger">删除</t-link>
            </t-popconfirm>
          </template>
        </t-table>
      </div>
    </div>

    <!-- 通知渠道 -->
    <div class="section-block">
      <div class="section-title">
        <span>通知渠道</span>
        <div class="title-actions">
          <t-button theme="primary" size="small" @click="openCreateChannel">
            <template #icon><add-icon /></template>
            添加渠道
          </t-button>
          <t-button variant="outline" size="small" :loading="channelsLoading" @click="loadChannels">刷新</t-button>
        </div>
      </div>
      <div class="block-body">
        <t-table :data="channels" :columns="channelColumns" :loading="channelsLoading" row-key="id" size="small" stripe>
          <template #type="{ row }">
            <t-tag theme="default" variant="light" size="small">{{ channelTypeLabel(row.type) }}</t-tag>
          </template>
          <template #enabled="{ row }">
            <t-switch v-model="row.enabled" size="small" @change="toggleChannel(row)" />
          </template>
          <template #operations="{ row }">
            <t-space size="small">
              <t-link theme="primary" @click="doTestChannel(row)">测试</t-link>
              <t-popconfirm content="确认删除此渠道？" @confirm="deleteChannelItem(row)">
                <t-link theme="danger">删除</t-link>
              </t-popconfirm>
            </t-space>
          </template>
        </t-table>
      </div>
    </div>

    <!-- 告警历史 -->
    <div class="section-block">
      <div class="section-title">
        <span>告警历史</span>
        <div class="title-actions">
          <t-button variant="outline" size="small" :loading="eventsLoading" @click="loadEvents">刷新</t-button>
          <t-popconfirm content="确认清理30天前的历史记录？" @confirm="doClearEvents">
          <t-button theme="warning" variant="outline" size="small">清理旧记录</t-button>
          </t-popconfirm>
        </div>
      </div>
      <div class="block-body">
        <t-table :data="events" :columns="eventColumns" :loading="eventsLoading" row-key="id" size="small" stripe>
          <template #sent_at="{ row }">{{ formatTime(row.sent_at) }}</template>
          <template #server="{ row }">{{ serverName(row.server_id) }}</template>
          <template #value="{ row }">{{ row.value ? row.value.toFixed(1) : '—' }}</template>
        </t-table>
        <div class="pagination-row">
          <t-pagination
            v-model:current="eventsPage"
            :page-size="50"
            :total="eventsTotal"
            show-total
            @change="loadEvents"
          />
        </div>
      </div>
    </div>

    <!-- 添加规则弹窗 -->
    <t-dialog
      v-model:visible="ruleVisible"
      header="添加告警规则"
      width="440px"
      :confirm-btn="{ content: '创建', loading: ruleSaving }"
      @confirm="confirmCreateRule"
    >
      <t-form :data="ruleForm" label-width="90px" size="small" colon>
        <t-form-item label="服务器">
          <t-select v-model="ruleForm.server_id" placeholder="所有服务器" clearable class="full-width">
            <t-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.host})`" :value="s.id" />
          </t-select>
        </t-form-item>
        <t-form-item label="指标">
          <t-select v-model="ruleForm.metric" class="full-width">
            <t-option label="CPU 使用率" value="cpu" />
            <t-option label="内存使用率" value="mem" />
            <t-option label="磁盘使用率" value="disk" />
            <t-option label="服务器离线" value="offline" />
          </t-select>
        </t-form-item>
        <template v-if="ruleForm.metric !== 'offline'">
          <t-form-item label="条件">
            <t-radio-group v-model="ruleForm.operator">
              <t-radio value="gt">大于 (&gt;)</t-radio>
              <t-radio value="lt">小于 (&lt;)</t-radio>
            </t-radio-group>
          </t-form-item>
          <t-form-item label="阈值 (%)">
            <t-input-number v-model="ruleForm.threshold" :min="0" :max="100" class="full-width" />
          </t-form-item>
        </template>
        <t-form-item label="持续次数">
          <t-input-number v-model="ruleForm.duration" :min="1" :max="10" class="full-width" />
          <div class="form-hint">连续触发 N 次才发告警，防止抖动</div>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 添加渠道弹窗 -->
    <t-dialog
      v-model:visible="channelVisible"
      header="添加通知渠道"
      width="500px"
      :confirm-btn="{ content: '创建', loading: channelSaving }"
      @confirm="confirmCreateChannel"
    >
      <t-form :data="channelForm" label-width="100px" size="small" colon>
        <t-form-item label="名称">
          <t-input v-model="channelForm.name" placeholder="我的企微机器人" />
        </t-form-item>
        <t-form-item label="类型">
          <t-select v-model="channelForm.type" class="full-width">
            <t-option label="企业微信机器人" value="webhook_wechat" />
            <t-option label="钉钉机器人" value="webhook_dingtalk" />
            <t-option label="自定义 Webhook" value="custom" />
          </t-select>
        </t-form-item>
        <t-form-item label="Webhook URL">
          <t-input v-model="channelForm.url" placeholder="https://qyapi.weixin.qq.com/..." />
        </t-form-item>
        <t-form-item label="消息模板">
          <t-textarea v-model="channelForm.template" :autosize="{ minRows: 3 }"
            placeholder="留空使用默认模板，可用变量: {{.Server}} {{.Metric}} {{.Value}} {{.Time}}" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { AddIcon } from 'tdesign-icons-vue-next'
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
  return ({ cpu: 'CPU', mem: '内存', disk: '磁盘', offline: '离线' } as Record<string, string>)[m] ?? m
}
function channelTypeLabel(t: string) {
  return ({ webhook_wechat: '企业微信', webhook_dingtalk: '钉钉', custom: '自定义' } as Record<string, string>)[t] ?? t
}
function formatTime(t: string) { return dayjs(t).format('MM-DD HH:mm:ss') }

const ruleColumns = [
  { colKey: 'server', title: '服务器', width: 140 },
  { colKey: 'metric', title: '指标', width: 90 },
  { colKey: 'condition', title: '条件', width: 160 },
  { colKey: 'duration', title: '持续次数', width: 90 },
  { colKey: 'enabled', title: '状态', width: 80 },
  { colKey: 'operations', title: '操作', width: 80, align: 'center' as const },
]
const channelColumns = [
  { colKey: 'name', title: '名称', minWidth: 140 },
  { colKey: 'type', title: '类型', width: 140 },
  { colKey: 'template', title: '消息模板', minWidth: 200, ellipsis: true },
  { colKey: 'enabled', title: '状态', width: 80 },
  { colKey: 'operations', title: '操作', width: 120, align: 'center' as const },
]
const eventColumns = [
  { colKey: 'sent_at', title: '时间', width: 160 },
  { colKey: 'server', title: '服务器', width: 130 },
  { colKey: 'message', title: '消息', minWidth: 300, ellipsis: true },
  { colKey: 'value', title: '值', width: 80 },
]

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
    MessagePlugin.success('规则已创建')
    ruleVisible.value = false
    await loadRules()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '创建失败') }
  finally { ruleSaving.value = false }
}
async function toggleRule(row: AlertRule) { await updateRule(row.id, { enabled: row.enabled }) }
async function deleteRuleItem(row: AlertRule) { await deleteRule(row.id); await loadRules() }

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
    MessagePlugin.warning('请填写名称和 URL')
    return
  }
  channelSaving.value = true
  try {
    await createChannel(channelForm.value)
    MessagePlugin.success('渠道已添加')
    channelVisible.value = false
    await loadChannels()
  } catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '创建失败') }
  finally { channelSaving.value = false }
}
async function toggleChannel(row: NotifyChannel) { await updateChannel(row.id, { enabled: row.enabled }) }
async function doTestChannel(row: NotifyChannel) {
  try { await testChannel(row.id); MessagePlugin.success('测试消息已发送') }
  catch (e: any) { MessagePlugin.error(e?.response?.data?.msg ?? '发送失败') }
}
async function deleteChannelItem(row: NotifyChannel) { await deleteChannel(row.id); await loadChannels() }

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
  MessagePlugin.success('已清理')
  await loadEvents()
}

onMounted(async () => {
  servers.value = await getServers()
  await Promise.all([loadRules(), loadChannels(), loadEvents()])
})
</script>

<style scoped>
.notifications-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.block-body {
  padding: 16px 20px;
}

.title-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.full-width { width: 100%; }

.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.form-hint {
  font-size: 11px;
  color: var(--sh-text-secondary);
  margin-top: 4px;
}
</style>
