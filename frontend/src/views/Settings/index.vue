<template>
  <div class="set-page">
    <UiSection title="系统设置">
      <UiCard padding="lg">
        <NForm :model="settingsForm" label-placement="left" label-width="200">
          <div class="set-group">基础配置</div>
          <NFormItem label="面板名称">
            <NInput v-model:value="settingsForm['panel_name']" placeholder="ServerHub" class="set-inp" />
          </NFormItem>
          <NFormItem label="时区">
            <NInput v-model:value="settingsForm['timezone']" placeholder="Asia/Shanghai" class="set-inp" />
          </NFormItem>
          <NFormItem label="指标采集间隔 (分钟)">
            <NInputNumber v-model:value="settingsForm['metrics_interval']" :min="1" :max="60" class="set-num" />
          </NFormItem>

          <div class="set-group">告警阈值</div>
          <NFormItem label="告警冷却时间 (分钟)">
            <NInputNumber v-model:value="settingsForm['alert_cooldown_min']" :min="5" :max="1440" class="set-num" />
          </NFormItem>
          <NFormItem label="CPU 告警阈值 (%)">
            <NInputNumber v-model:value="settingsForm['alert_cpu_threshold']" :min="1" :max="100" class="set-num" />
          </NFormItem>
          <NFormItem label="内存告警阈值 (%)">
            <NInputNumber v-model:value="settingsForm['alert_mem_threshold']" :min="1" :max="100" class="set-num" />
          </NFormItem>
          <NFormItem label="磁盘告警阈值 (%)">
            <NInputNumber v-model:value="settingsForm['alert_disk_threshold']" :min="1" :max="100" class="set-num" />
          </NFormItem>
          <NFormItem label="SSL 到期预警 (天)">
            <NInputNumber v-model:value="settingsForm['alert_ssl_days']" :min="1" :max="90" class="set-num" />
          </NFormItem>

          <div class="set-group">运维配置</div>
          <NFormItem label="证书自动续签 (天前)">
            <NInputNumber v-model:value="settingsForm['cert_renew_days']" :min="1" :max="60" class="set-num" />
          </NFormItem>
          <NFormItem label="部署日志保留 (天)">
            <NInputNumber v-model:value="settingsForm['deploy_log_keep_days']" :min="1" :max="365" class="set-num" />
          </NFormItem>

          <NFormItem label=" " :show-label="false">
            <UiButton variant="primary" size="md" :loading="savingSettings" @click="saveSettings">保存设置</UiButton>
          </NFormItem>
        </NForm>
      </UiCard>
    </UiSection>

    <UiSection>
      <template #title>
        <span class="set-title">两步验证（TOTP）</span>
        <UiBadge :tone="meUser?.mfa_enabled ? 'success' : 'warning'">
          {{ meUser?.mfa_enabled ? '已启用' : '未启用' }}
        </UiBadge>
      </template>
      <UiCard padding="lg">
        <div v-if="!totpSetupMode">
          <p class="set-desc">两步验证可为您的账号增加额外安全保护。启用后每次登录除密码外还需提供验证码。</p>
          <UiButton v-if="!meUser?.mfa_enabled" variant="primary" size="md" @click="startTotpSetup">启用两步验证</UiButton>
          <UiButton v-else variant="danger" size="md" @click="disableTotp">禁用两步验证</UiButton>
        </div>
        <div v-else class="totp-setup">
          <p class="set-desc">1. 使用 Google Authenticator 或 Authy 扫描下方信息</p>
          <div class="totp-info">
            <div class="totp-row">
              <span class="totp-label">密钥（手动输入）</span>
              <span class="totp-secret">{{ totpSecret }}</span>
            </div>
            <div class="totp-row">
              <span class="totp-label">OTP URI</span>
              <span class="totp-uri">{{ totpUri }}</span>
            </div>
          </div>
          <p class="set-desc">2. 扫描后输入 App 中显示的 6 位验证码以完成绑定</p>
          <div class="totp-actions">
            <NInput v-model:value="confirmCode" placeholder="6 位验证码" :maxlength="6" class="totp-input" />
            <UiButton variant="primary" size="md" :loading="confirmingTotp" @click="confirmTotp">确认绑定</UiButton>
            <UiButton variant="secondary" size="md" @click="totpSetupMode = false">取消</UiButton>
          </div>
        </div>
      </UiCard>
    </UiSection>

    <UiSection title="操作日志">
      <template #extra>
        <NInput v-model:value="auditFilter.username" placeholder="用户名" size="small" clearable class="filter-sm" @blur="loadAudit" />
        <NInput v-model:value="auditFilter.path" placeholder="路径" size="small" clearable class="filter-md" @blur="loadAudit" />
        <NSelect v-model:value="auditFilter.status" placeholder="状态" size="small" clearable :options="statusOptions" class="filter-sm" @update:value="loadAudit" />
        <UiButton :variant="secOnly ? 'primary' : 'secondary'" size="sm" @click="toggleSecOnly">
          仅安全
        </UiButton>
        <UiButton variant="secondary" size="sm" @click="loadAudit">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>
      <UiCard padding="none">
        <NDataTable
          :columns="auditColumns"
          :data="auditLogs"
          :loading="auditLoading"
          :row-key="(row: AuditLog) => row.id"
          size="small"
          :bordered="false"
        />
        <div class="pg-row">
          <NPagination v-model:page="auditPage" :page-size="50" :item-count="auditTotal" show-quick-jumper @update:page="loadAudit" />
        </div>
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  NForm, NFormItem, NInput, NInputNumber, NSelect,
  NDataTable, NPagination, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { getSettings, putSettings, getAuditLogs, type AuditLog } from '@/api/settings'
import { totpSetup, totpConfirm, totpDisable, getMe } from '@/api/auth'
import type { User } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const message = useMessage()

const settingsForm = reactive<Record<string, any>>({
  panel_name: '',
  metrics_interval: 5,
  alert_cooldown_min: 30,
  alert_cpu_threshold: 90,
  alert_mem_threshold: 85,
  alert_disk_threshold: 80,
  alert_ssl_days: 30,
  cert_renew_days: 30,
  deploy_log_keep_days: 30,
  timezone: 'Asia/Shanghai',
})
const savingSettings = ref(false)

async function loadSettings() {
  const data = await getSettings()
  Object.keys(settingsForm).forEach(k => {
    if (data[k] === undefined) return
    const n = Number(data[k])
    settingsForm[k] = isNaN(n) ? data[k] : n
  })
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const payload: Record<string, string> = {}
    Object.entries(settingsForm).forEach(([k, v]) => { payload[k] = String(v) })
    await putSettings(payload)
    message.success('设置已保存')
  } finally {
    savingSettings.value = false
  }
}

const meUser = ref<User | null>(null)
const totpSetupMode = ref(false)
const totpSecret = ref('')
const totpUri = ref('')
const confirmCode = ref('')
const confirmingTotp = ref(false)

async function loadMe() { meUser.value = await getMe() }

async function startTotpSetup() {
  totpSetupMode.value = true
  const data = await totpSetup()
  totpSecret.value = data.secret
  totpUri.value = data.uri
}

async function confirmTotp() {
  if (!confirmCode.value) return
  confirmingTotp.value = true
  try {
    await totpConfirm(totpSecret.value, confirmCode.value)
    message.success('两步验证已启用')
    totpSetupMode.value = false
    await loadMe()
  } catch {
    message.error('验证码错误，请重试')
  } finally {
    confirmingTotp.value = false
  }
}

async function disableTotp() {
  await totpDisable()
  message.success('两步验证已禁用')
  await loadMe()
}

const auditLogs = ref<AuditLog[]>([])
const auditTotal = ref(0)
const auditPage = ref(1)
const auditLoading = ref(false)
const auditFilter = reactive({ username: '', path: '', status: '' })
const secOnly = ref(false)
function toggleSecOnly() {
  secOnly.value = !secOnly.value
  auditFilter.path = secOnly.value ? 'security:' : ''
  auditPage.value = 1
  loadAudit()
}

const statusOptions = [
  { label: '成功 2xx', value: '2' },
  { label: '客户端错误 4xx', value: '4' },
  { label: '服务错误 5xx', value: '5' },
]

function methodTone(m: string): 'success' | 'warning' | 'danger' | 'neutral' {
  return ({ GET: 'neutral', POST: 'success', PUT: 'warning', DELETE: 'danger' } as Record<string, 'success' | 'warning' | 'danger' | 'neutral'>)[m] ?? 'neutral'
}
function statusTone(s: number): 'success' | 'warning' | 'danger' {
  if (s >= 500) return 'danger'
  if (s >= 400) return 'warning'
  return 'success'
}
function fmtTime(t: string) {
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

const auditColumns = computedColumns()
function computedColumns(): DataTableColumns<AuditLog> {
  return [
    { title: '时间', key: 'created_at', width: 170, render: (row) => fmtTime(row.created_at) },
    { title: '用户', key: 'username', width: 100 },
    { title: '方法', key: 'method', width: 80, render: (row) => h(UiBadge, { tone: methodTone(row.method) }, () => row.method) },
    { title: '路径', key: 'path', minWidth: 200, ellipsis: { tooltip: true } },
    { title: '状态', key: 'status', width: 80, render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => String(row.status)) },
    { title: 'IP', key: 'ip', width: 130 },
    { title: '延迟(ms)', key: 'latency_ms', width: 90 },
  ]
}

async function loadAudit() {
  auditLoading.value = true
  const params: Record<string, any> = { page: auditPage.value, size: 50 }
  if (auditFilter.username) params.username = auditFilter.username
  if (auditFilter.path) params.path = auditFilter.path
  if (auditFilter.status) params.status = auditFilter.status
  try {
    const data = await getAuditLogs(params)
    auditLogs.value = data.logs ?? []
    auditTotal.value = data.total ?? 0
  } finally { auditLoading.value = false }
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadMe(), loadAudit()])
})
</script>

<style scoped>
.set-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.set-title { display: inline-flex; align-items: center; gap: var(--space-2); }

.set-group {
  font-size: var(--fs-xs);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: var(--space-3) 0 var(--space-2);
  border-bottom: 1px solid var(--ui-border);
  margin-bottom: var(--space-3);
}
.set-group:first-child { padding-top: 0; }

.set-desc {
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  margin: 0 0 var(--space-4);
  line-height: 1.6;
}

.set-inp { width: 240px; }
.set-num { width: 160px; }

.totp-setup { max-width: 600px; }
.totp-info {
  margin: var(--space-3) 0;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.totp-row {
  display: flex; align-items: flex-start;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--ui-border);
  gap: var(--space-3);
}
.totp-row:last-child { border-bottom: none; }
.totp-label {
  flex-shrink: 0;
  width: 140px;
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.totp-secret {
  font-family: var(--font-mono);
  word-break: break-all;
  color: var(--ui-brand-fg);
}
.totp-uri {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  word-break: break-all;
  color: var(--ui-fg-3);
}
.totp-actions {
  display: flex; align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-2);
}
.totp-input { width: 200px; }

.filter-sm { width: 120px; }
.filter-md { width: 160px; }

.pg-row {
  display: flex; justify-content: flex-end;
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--ui-border);
}
</style>
