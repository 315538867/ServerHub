<template>
  <div class="ntf-page">
    <UiSection title="告警规则">
      <template #extra>
        <UiButton variant="primary" size="sm" @click="openCreateRule">
          <template #icon><Plus :size="14" /></template>
          添加规则
        </UiButton>
        <UiButton variant="secondary" size="sm" :loading="rulesLoading" @click="loadRules">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>
      <UiCard padding="none">
        <NDataTable
          :columns="ruleColumns"
          :data="rules"
          :loading="rulesLoading"
          :row-key="(row: AlertRule) => row.id"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>

    <UiSection title="通知渠道">
      <template #extra>
        <UiButton variant="primary" size="sm" @click="openCreateChannel">
          <template #icon><Plus :size="14" /></template>
          添加渠道
        </UiButton>
        <UiButton variant="secondary" size="sm" :loading="channelsLoading" @click="loadChannels">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>
      <UiCard padding="none">
        <NDataTable
          :columns="channelColumns"
          :data="channels"
          :loading="channelsLoading"
          :row-key="(row: NotifyChannel) => row.id"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>

    <UiSection title="告警历史">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="eventsLoading" @click="loadEvents">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
        <NPopconfirm @positive-click="doClearEvents" positive-text="清理" negative-text="取消">
          <template #trigger>
            <UiButton variant="warning" size="sm">清理旧记录</UiButton>
          </template>
          确认清理 30 天前的历史记录？
        </NPopconfirm>
      </template>
      <UiCard padding="none">
        <NDataTable
          :columns="eventColumns"
          :data="events"
          :loading="eventsLoading"
          :row-key="(row: AlertEvent) => row.id"
          size="small"
          :bordered="false"
        />
        <div class="pg-row">
          <NPagination v-model:page="eventsPage" :page-size="50" :item-count="eventsTotal" show-quick-jumper @update:page="loadEvents" />
        </div>
      </UiCard>
    </UiSection>

    <NModal v-model:show="ruleVisible" preset="card" title="添加告警规则" style="width: 480px" :bordered="false">
      <NForm :model="ruleForm" label-placement="left" label-width="100">
        <NFormItem label="服务器">
          <NSelect v-model:value="ruleForm.server_id" placeholder="所有服务器" clearable :options="serverOptions" />
        </NFormItem>
        <NFormItem label="指标">
          <NSelect v-model:value="ruleForm.metric" :options="metricOptions" />
        </NFormItem>
        <template v-if="ruleForm.metric !== 'offline'">
          <NFormItem label="条件">
            <NRadioGroup v-model:value="ruleForm.operator">
              <NRadio value="gt">大于 (&gt;)</NRadio>
              <NRadio value="lt">小于 (&lt;)</NRadio>
            </NRadioGroup>
          </NFormItem>
          <NFormItem label="阈值 (%)">
            <NInputNumber v-model:value="ruleForm.threshold" :min="0" :max="100" style="width: 100%" />
          </NFormItem>
        </template>
        <NFormItem label="持续次数">
          <div style="width: 100%">
            <NInputNumber v-model:value="ruleForm.duration" :min="1" :max="10" style="width: 100%" />
            <div class="form-hint">连续触发 N 次才发告警，防止抖动</div>
          </div>
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="ruleVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="ruleSaving" @click="confirmCreateRule">创建</UiButton>
        </div>
      </template>
    </NModal>

    <NModal v-model:show="channelVisible" preset="card" title="添加通知渠道" style="width: 540px" :bordered="false">
      <NForm :model="channelForm" label-placement="left" label-width="100">
        <NFormItem label="名称">
          <NInput v-model:value="channelForm.name" placeholder="我的企微机器人" />
        </NFormItem>
        <NFormItem label="类型">
          <NSelect v-model:value="channelForm.type" :options="channelTypeOptions" />
        </NFormItem>
        <NFormItem label="Webhook URL">
          <NInput v-model:value="channelForm.url" placeholder="https://qyapi.weixin.qq.com/..." />
        </NFormItem>
        <NFormItem label="消息模板">
          <NInput v-model:value="channelForm.template" type="textarea" :autosize="{ minRows: 3 }"
            placeholder="留空使用默认模板，可用变量: {{.Server}} {{.Metric}} {{.Value}} {{.Time}}" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="channelVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="channelSaving" @click="confirmCreateChannel">创建</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber, NSelect,
  NRadioGroup, NRadio, NSwitch, NPopconfirm, NPagination, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { Plus, RefreshCw } from 'lucide-vue-next'
import dayjs from 'dayjs'
import { getServers } from '@/api/servers'
import {
  listRules, createRule, updateRule, deleteRule,
  listEvents, clearEvents,
  listChannels, createChannel, updateChannel, deleteChannel, testChannel,
} from '@/api/alerts'
import type { AlertRule, AlertEvent, NotifyChannel } from '@/api/alerts'
import type { Server } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const message = useMessage()
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

const serverOptions = computed(() => servers.value.map(s => ({ label: `${s.name} (${s.host})`, value: s.id })))
const metricOptions = [
  { label: 'CPU 使用率', value: 'cpu' },
  { label: '内存使用率', value: 'mem' },
  { label: '磁盘使用率', value: 'disk' },
  { label: '服务器离线', value: 'offline' },
]
const channelTypeOptions = [
  { label: '企业微信机器人', value: 'webhook_wechat' },
  { label: '钉钉机器人', value: 'webhook_dingtalk' },
  { label: '自定义 Webhook', value: 'custom' },
]

const ruleColumns = computed<DataTableColumns<AlertRule>>(() => [
  { title: '服务器', key: 'server', width: 160, render: (row) => row.server_id ? serverName(row.server_id) : '所有服务器' },
  { title: '指标', key: 'metric', width: 90, render: (row) => h(UiBadge, { tone: 'neutral' }, () => metricLabel(row.metric)) },
  {
    title: '条件', key: 'condition', width: 160,
    render: (row) => row.metric !== 'offline' ? `${row.operator === 'gt' ? '>' : '<'} ${row.threshold}%` : '离线',
  },
  { title: '持续次数', key: 'duration', width: 90 },
  {
    title: '状态', key: 'enabled', width: 80,
    render: (row) => h(NSwitch, { value: row.enabled, size: 'small', 'onUpdate:value': (v: boolean) => { row.enabled = v; toggleRule(row) } }),
  },
  {
    title: '操作', key: 'ops', width: 80, fixed: 'right' as const, align: 'center' as const,
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => deleteRuleItem(row),
      positiveText: '删除', negativeText: '取消',
    }, {
      trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
        () => h('span', { class: 'text-danger' }, '删除')),
      default: () => '确认删除此规则？',
    }),
  },
])

const channelColumns = computed<DataTableColumns<NotifyChannel>>(() => [
  { title: '名称', key: 'name', minWidth: 160, ellipsis: { tooltip: true } },
  { title: '类型', key: 'type', width: 140, render: (row) => h(UiBadge, { tone: 'neutral' }, () => channelTypeLabel(row.type)) },
  { title: '消息模板', key: 'template', minWidth: 200, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'enabled', width: 80,
    render: (row) => h(NSwitch, { value: row.enabled, size: 'small', 'onUpdate:value': (v: boolean) => { row.enabled = v; toggleChannel(row) } }),
  },
  {
    title: '操作', key: 'ops', width: 130, fixed: 'right' as const, align: 'center' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => doTestChannel(row) }, () => '测试'),
      h(NPopconfirm, {
        onPositiveClick: () => deleteChannelItem(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除此渠道？',
      }),
    ]),
  },
])

const eventColumns = computed<DataTableColumns<AlertEvent>>(() => [
  { title: '时间', key: 'sent_at', width: 170, render: (row) => formatTime(row.sent_at) },
  { title: '服务器', key: 'server', width: 140, render: (row) => serverName(row.server_id) },
  { title: '消息', key: 'message', minWidth: 300, ellipsis: { tooltip: true } },
  { title: '值', key: 'value', width: 80, render: (row) => row.value ? row.value.toFixed(1) : '—' },
])

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
    message.success('规则已创建')
    ruleVisible.value = false
    await loadRules()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '创建失败') }
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
    message.warning('请填写名称和 URL')
    return
  }
  channelSaving.value = true
  try {
    await createChannel(channelForm.value)
    message.success('渠道已添加')
    channelVisible.value = false
    await loadChannels()
  } catch (e: any) { message.error(e?.response?.data?.msg ?? '创建失败') }
  finally { channelSaving.value = false }
}
async function toggleChannel(row: NotifyChannel) { await updateChannel(row.id, { enabled: row.enabled }) }
async function doTestChannel(row: NotifyChannel) {
  try { await testChannel(row.id); message.success('测试消息已发送') }
  catch (e: any) { message.error(e?.response?.data?.msg ?? '发送失败') }
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
  message.success('已清理')
  await loadEvents()
}

onMounted(async () => {
  servers.value = await getServers()
  await Promise.all([loadRules(), loadChannels(), loadEvents()])
})
</script>

<style scoped>
.ntf-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }
.form-hint { font-size: var(--fs-xs); color: var(--ui-fg-3); margin-top: var(--space-1); }
.pg-row {
  display: flex; justify-content: flex-end;
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--ui-border);
}
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
