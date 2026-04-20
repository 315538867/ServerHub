<template>
  <div class="settings-page">
    <t-tabs :value="activeTab" @change="activeTab = $event as string">
      <t-tab-panel value="panel" label="面板设置">
        <t-card title="系统设置" style="margin-top:12px">
          <t-form :data="settingsForm" label-width="180px" colon>
            <t-form-item label="面板名称">
              <t-input v-model="settingsForm['panel_name']" placeholder="ServerHub" style="width:240px" />
            </t-form-item>
            <t-form-item label="指标采集间隔 (分钟)">
              <t-input-number v-model="settingsForm['metrics_interval']" :min="1" :max="60" style="width:160px" />
            </t-form-item>
            <t-form-item label="告警冷却时间 (分钟)">
              <t-input-number v-model="settingsForm['alert_cooldown_min']" :min="5" :max="1440" style="width:160px" />
            </t-form-item>
            <t-form-item label="CPU 告警阈值 (%)">
              <t-input-number v-model="settingsForm['alert_cpu_threshold']" :min="1" :max="100" style="width:160px" />
            </t-form-item>
            <t-form-item label="内存告警阈值 (%)">
              <t-input-number v-model="settingsForm['alert_mem_threshold']" :min="1" :max="100" style="width:160px" />
            </t-form-item>
            <t-form-item label="磁盘告警阈值 (%)">
              <t-input-number v-model="settingsForm['alert_disk_threshold']" :min="1" :max="100" style="width:160px" />
            </t-form-item>
            <t-form-item label="SSL 到期预警 (天)">
              <t-input-number v-model="settingsForm['alert_ssl_days']" :min="1" :max="90" style="width:160px" />
            </t-form-item>
            <t-form-item label="证书自动续签 (天前)">
              <t-input-number v-model="settingsForm['cert_renew_days']" :min="1" :max="60" style="width:160px" />
            </t-form-item>
            <t-form-item label="部署日志保留 (天)">
              <t-input-number v-model="settingsForm['deploy_log_keep_days']" :min="1" :max="365" style="width:160px" />
            </t-form-item>
            <t-form-item label="时区">
              <t-input v-model="settingsForm['timezone']" placeholder="Asia/Shanghai" style="width:240px" />
            </t-form-item>
            <t-form-item>
              <t-button theme="primary" :loading="savingSettings" @click="saveSettings">保存设置</t-button>
            </t-form-item>
          </t-form>
        </t-card>
      </t-tab-panel>

      <t-tab-panel value="totp" label="两步验证">
        <t-card title="两步验证（TOTP）" style="margin-top:12px">
          <div v-if="!totpSetupMode">
            <t-alert
              v-if="meUser?.mfa_enabled"
              theme="success"
              message="两步验证已启用"
              style="margin-bottom:16px"
            />
            <t-alert
              v-else
              theme="warning"
              message="两步验证未启用"
              style="margin-bottom:16px"
            />
            <t-button v-if="!meUser?.mfa_enabled" theme="primary" @click="startTotpSetup">
              启用两步验证
            </t-button>
            <t-button v-else theme="danger" @click="disableTotp">
              禁用两步验证
            </t-button>
          </div>
          <div v-else>
            <p class="totp-instruction">1. 使用 Google Authenticator 或 Authy 扫描下方信息</p>
            <t-descriptions :column="1" bordered class="totp-desc" size="small">
              <t-descriptions-item label="密钥（手动输入）">
                <span style="font-family:monospace;word-break:break-all;color:var(--td-brand-color)">{{ totpSecret }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="OTP URI">
                <span style="font-size:11px;word-break:break-all">{{ totpUri }}</span>
              </t-descriptions-item>
            </t-descriptions>
            <p class="totp-instruction">2. 扫描后输入 App 中显示的 6 位验证码以完成绑定</p>
            <t-input v-model="confirmCode" placeholder="6 位验证码" :maxlength="6" style="width:200px;margin-bottom:12px" />
            <br />
            <t-space>
              <t-button theme="primary" :loading="confirmingTotp" @click="confirmTotp">确认绑定</t-button>
              <t-button variant="outline" @click="totpSetupMode = false">取消</t-button>
            </t-space>
          </div>
        </t-card>
      </t-tab-panel>

      <t-tab-panel value="audit" label="审计日志">
        <t-card style="margin-top:12px">
          <template #title>
            <div class="audit-header">
              <span>操作日志</span>
              <t-space size="small">
                <t-input v-model="auditFilter.username" placeholder="用户名" style="width:120px" clearable @change="loadAudit" />
                <t-input v-model="auditFilter.path" placeholder="路径" style="width:160px" clearable @change="loadAudit" />
                <t-select v-model="auditFilter.status" placeholder="状态" style="width:110px" clearable @change="loadAudit">
                  <t-option label="成功 2xx" value="2" />
                  <t-option label="客户端错误 4xx" value="4" />
                  <t-option label="服务错误 5xx" value="5" />
                </t-select>
                <t-button variant="outline" size="small" @click="loadAudit">刷新</t-button>
              </t-space>
            </div>
          </template>
          <t-table :data="auditLogs" :columns="auditColumns" :loading="auditLoading" row-key="id" size="small" stripe>
            <template #created_at="{ row }">{{ fmtTime(row.created_at) }}</template>
            <template #method="{ row }">
              <t-tag :theme="methodTheme(row.method)" variant="light" size="small">{{ row.method }}</t-tag>
            </template>
            <template #status_code="{ row }">
              <t-tag :theme="statusCodeTheme(row.status)" variant="light" size="small">{{ row.status }}</t-tag>
            </template>
          </t-table>
          <div style="margin-top:12px; display:flex; justify-content:flex-end">
            <t-pagination
              v-model:current="auditPage"
              :page-size="50"
              :total="auditTotal"
              show-total
              @change="loadAudit"
            />
          </div>
        </t-card>
      </t-tab-panel>
    </t-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { getSettings, putSettings, getAuditLogs, type AuditLog } from '@/api/settings'
import { totpSetup, totpConfirm, totpDisable, getMe } from '@/api/auth'
import type { User } from '@/types/api'

const activeTab = ref('panel')

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
    MessagePlugin.success('设置已保存')
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
    MessagePlugin.success('两步验证已启用')
    totpSetupMode.value = false
    await loadMe()
  } catch {
    MessagePlugin.error('验证码错误，请重试')
  } finally {
    confirmingTotp.value = false
  }
}

async function disableTotp() {
  await totpDisable()
  MessagePlugin.success('两步验证已禁用')
  await loadMe()
}

const auditLogs = ref<AuditLog[]>([])
const auditTotal = ref(0)
const auditPage = ref(1)
const auditLoading = ref(false)
const auditFilter = reactive({ username: '', path: '', status: '' })

const auditColumns = [
  { colKey: 'created_at', title: '时间', width: 160 },
  { colKey: 'username', title: '用户', width: 100 },
  { colKey: 'method', title: '方法', width: 80 },
  { colKey: 'path', title: '路径', minWidth: 200, ellipsis: true },
  { colKey: 'status_code', title: '状态', width: 80 },
  { colKey: 'ip', title: 'IP', width: 130 },
  { colKey: 'latency_ms', title: '延迟(ms)', width: 90 },
]

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

function fmtTime(t: string) {
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}
function methodTheme(m: string) {
  return ({ GET: 'default', POST: 'success', PUT: 'warning', DELETE: 'danger' } as Record<string, string>)[m] ?? 'default'
}
function statusCodeTheme(s: number) {
  if (s >= 500) return 'danger'
  if (s >= 400) return 'warning'
  return 'success'
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadMe(), loadAudit()])
})
</script>

<style scoped>
.settings-page { padding: 0; }
.audit-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; width: 100%; }
.totp-instruction { color: var(--td-text-color-secondary); margin: 12px 0; }
.totp-desc { margin: 12px 0; }
</style>
