<template>
  <div class="settings-page">
    <el-tabs v-model="activeTab">
      <!-- Panel Settings -->
      <el-tab-pane label="面板设置" name="panel">
        <el-card>
          <template #header>系统设置</template>
          <el-form :model="settingsForm" label-width="160px">
            <el-form-item label="面板名称">
              <el-input v-model="settingsForm['panel_name']" placeholder="ServerHub" />
            </el-form-item>
            <el-form-item label="指标采集间隔 (分钟)">
              <el-input-number v-model.number="settingsForm['metrics_interval']" :min="1" :max="60" />
            </el-form-item>
            <el-form-item label="告警冷却时间 (分钟)">
              <el-input-number v-model.number="settingsForm['alert_cooldown_min']" :min="5" :max="1440" />
            </el-form-item>
            <el-form-item label="CPU 告警阈值 (%)">
              <el-input-number v-model.number="settingsForm['alert_cpu_threshold']" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="内存告警阈值 (%)">
              <el-input-number v-model.number="settingsForm['alert_mem_threshold']" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="磁盘告警阈值 (%)">
              <el-input-number v-model.number="settingsForm['alert_disk_threshold']" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="SSL 到期预警 (天)">
              <el-input-number v-model.number="settingsForm['alert_ssl_days']" :min="1" :max="90" />
            </el-form-item>
            <el-form-item label="证书自动续签 (天前)">
              <el-input-number v-model.number="settingsForm['cert_renew_days']" :min="1" :max="60" />
            </el-form-item>
            <el-form-item label="部署日志保留 (天)">
              <el-input-number v-model.number="settingsForm['deploy_log_keep_days']" :min="1" :max="365" />
            </el-form-item>
            <el-form-item label="时区">
              <el-input v-model="settingsForm['timezone']" placeholder="Asia/Shanghai" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="savingSettings" @click="saveSettings">保存设置</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- TOTP 2FA -->
      <el-tab-pane label="两步验证" name="totp">
        <el-card>
          <template #header>两步验证（TOTP）</template>
          <div v-if="!totpSetupMode">
            <el-alert
              v-if="meUser?.mfa_enabled"
              title="两步验证已启用"
              type="success"
              show-icon
              :closable="false"
              style="margin-bottom:16px"
            />
            <el-alert
              v-else
              title="两步验证未启用"
              type="warning"
              show-icon
              :closable="false"
              style="margin-bottom:16px"
            />
            <el-button v-if="!meUser?.mfa_enabled" type="primary" @click="startTotpSetup">
              启用两步验证
            </el-button>
            <el-button v-else type="danger" @click="disableTotp">
              禁用两步验证
            </el-button>
          </div>

          <div v-else>
            <p class="totp-instruction">
              1. 使用 Google Authenticator 或 Authy 扫描下方二维码
            </p>
            <div class="qr-wrapper">
              <el-descriptions :column="1" border size="small">
                <el-descriptions-item label="密钥（手动输入）">
                  <el-text type="primary" style="font-family:monospace;word-break:break-all">{{ totpSecret }}</el-text>
                </el-descriptions-item>
                <el-descriptions-item label="OTP URI">
                  <el-text style="font-size:11px;word-break:break-all">{{ totpUri }}</el-text>
                </el-descriptions-item>
              </el-descriptions>
            </div>
            <p class="totp-instruction">
              2. 扫描后输入 App 中显示的 6 位验证码以完成绑定
            </p>
            <el-input
              v-model="confirmCode"
              placeholder="6 位验证码"
              maxlength="6"
              style="width:200px;margin-bottom:12px"
            />
            <br />
            <el-button type="primary" :loading="confirmingTotp" @click="confirmTotp">确认绑定</el-button>
            <el-button @click="totpSetupMode = false">取消</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Audit Logs -->
      <el-tab-pane label="审计日志" name="audit">
        <el-card>
          <template #header>
            <div class="audit-header">
              <span>操作日志</span>
              <div class="audit-filters">
                <el-input v-model="auditFilter.username" placeholder="用户名" style="width:120px" clearable @change="loadAudit" />
                <el-input v-model="auditFilter.path" placeholder="路径" style="width:160px" clearable @change="loadAudit" />
                <el-select v-model="auditFilter.status" placeholder="状态" style="width:100px" clearable @change="loadAudit">
                  <el-option label="成功 2xx" value="2" />
                  <el-option label="客户端错误 4xx" value="4" />
                  <el-option label="服务错误 5xx" value="5" />
                </el-select>
                <el-button @click="loadAudit">刷新</el-button>
              </div>
            </div>
          </template>
          <el-table :data="auditLogs" stripe size="small">
            <el-table-column prop="created_at" label="时间" width="160">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="username" label="用户" width="100" />
            <el-table-column prop="method" label="方法" width="70">
              <template #default="{ row }">
                <el-tag size="small" :type="methodType(row.method)">{{ row.method }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="path" label="路径" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="70">
              <template #default="{ row }">
                <el-tag size="small" :type="statusType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="ip" label="IP" width="130" />
            <el-table-column prop="latency_ms" label="延迟(ms)" width="90" />
          </el-table>
          <el-pagination
            v-model:current-page="auditPage"
            :page-size="50"
            :total="auditTotal"
            layout="total, prev, pager, next"
            style="margin-top:12px"
            @current-change="loadAudit"
          />
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSettings, putSettings, getAuditLogs, type AuditLog } from '@/api/settings'
import { totpSetup, totpConfirm, totpDisable, getMe } from '@/api/auth'
import type { User } from '@/types/api'

const activeTab = ref('panel')

// ─── Settings ───────────────────────────────────────────────
const settingsForm = reactive<Record<string, any>>({
  'panel_name': '',
  'metrics_interval': 5,
  'alert_cooldown_min': 30,
  'alert_cpu_threshold': 90,
  'alert_mem_threshold': 85,
  'alert_disk_threshold': 80,
  'alert_ssl_days': 30,
  'cert_renew_days': 30,
  'deploy_log_keep_days': 30,
  'timezone': 'Asia/Shanghai',
})
const savingSettings = ref(false)

async function loadSettings() {
  const data = await getSettings()
  Object.keys(settingsForm).forEach(k => {
    if (data[k] === undefined) return
    const v = data[k]
    const n = Number(v)
    settingsForm[k] = isNaN(n) ? v : n
  })
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const payload: Record<string, string> = {}
    Object.entries(settingsForm).forEach(([k, v]) => { payload[k] = String(v) })
    await putSettings(payload)
    ElMessage.success('设置已保存')
  } finally {
    savingSettings.value = false
  }
}

// ─── TOTP ───────────────────────────────────────────────────
const meUser = ref<User | null>(null)
const totpSetupMode = ref(false)
const totpSecret = ref('')
const totpUri = ref('')
const confirmCode = ref('')
const confirmingTotp = ref(false)

async function loadMe() {
  meUser.value = await getMe()
}

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
    ElMessage.success('两步验证已启用')
    totpSetupMode.value = false
    await loadMe()
  } catch {
    ElMessage.error('验证码错误，请重试')
  } finally {
    confirmingTotp.value = false
  }
}

async function disableTotp() {
  await totpDisable()
  ElMessage.success('两步验证已禁用')
  await loadMe()
}

// ─── Audit ──────────────────────────────────────────────────
const auditLogs = ref<AuditLog[]>([])
const auditTotal = ref(0)
const auditPage = ref(1)
const auditFilter = reactive({ username: '', path: '', status: '' })

async function loadAudit() {
  const params: Record<string, any> = { page: auditPage.value, size: 50 }
  if (auditFilter.username) params.username = auditFilter.username
  if (auditFilter.path) params.path = auditFilter.path
  if (auditFilter.status) params.status = auditFilter.status
  const data = await getAuditLogs(params)
  auditLogs.value = data.logs ?? []
  auditTotal.value = data.total ?? 0
}

function fmtTime(t: string) {
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

function methodType(m: string) {
  const map: Record<string, string> = { GET: '', POST: 'success', PUT: 'warning', DELETE: 'danger' }
  return (map[m] ?? 'info') as any
}

function statusType(s: number) {
  if (s >= 500) return 'danger'
  if (s >= 400) return 'warning'
  return 'success'
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadMe(), loadAudit()])
})
</script>

<style scoped>
.settings-page { padding: 16px; }
.audit-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.audit-filters { display: flex; gap: 8px; flex-wrap: wrap; }
.totp-instruction { color: var(--el-text-color-regular); margin: 12px 0; }
.qr-wrapper { margin: 12px 0; }
</style>
