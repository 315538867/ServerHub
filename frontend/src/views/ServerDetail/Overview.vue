<template>
  <div class="page-container">
    <!-- 顶部4个指标卡片 -->
    <div class="metrics-row">
      <div class="metric-card section-block">
        <div class="metric-label">CPU 使用率</div>
        <div class="metric-value" :style="{ color: progressColor(latestMetric?.cpu ?? 0) }">
          {{ (latestMetric?.cpu ?? 0).toFixed(1) }}<span class="metric-unit">%</span>
        </div>
        <t-progress
          :percentage="+(latestMetric?.cpu ?? 0).toFixed(1)"
          :color="progressColor(latestMetric?.cpu ?? 0)"
          :stroke-width="6"
          :show-label="false"
        />
      </div>
      <div class="metric-card section-block">
        <div class="metric-label">内存使用率</div>
        <div class="metric-value" :style="{ color: progressColor(latestMetric?.mem ?? 0) }">
          {{ (latestMetric?.mem ?? 0).toFixed(1) }}<span class="metric-unit">%</span>
        </div>
        <t-progress
          :percentage="+(latestMetric?.mem ?? 0).toFixed(1)"
          :color="progressColor(latestMetric?.mem ?? 0)"
          :stroke-width="6"
          :show-label="false"
        />
      </div>
      <div class="metric-card section-block">
        <div class="metric-label">磁盘使用率</div>
        <div class="metric-value" :style="{ color: progressColor(latestMetric?.disk ?? 0) }">
          {{ (latestMetric?.disk ?? 0).toFixed(1) }}<span class="metric-unit">%</span>
        </div>
        <t-progress
          :percentage="+(latestMetric?.disk ?? 0).toFixed(1)"
          :color="progressColor(latestMetric?.disk ?? 0)"
          :stroke-width="6"
          :show-label="false"
        />
      </div>
      <div class="metric-card section-block">
        <div class="metric-label">系统负载</div>
        <div class="metric-value" style="color: var(--sh-text-primary)">
          {{ latestMetric?.load1?.toFixed(2) ?? '—' }}
        </div>
        <div class="metric-sub">运行时间：{{ formatUptime(latestMetric?.uptime) }}</div>
      </div>
    </div>

    <!-- 服务器基本信息 -->
    <div class="section-block">
      <div class="section-title">
        <span class="info-title">
          <server-icon style="color: var(--sh-blue); font-size: 16px" />
          服务器信息
        </span>
        <t-space size="small">
          <t-button size="small" variant="outline" :loading="testing" @click="doTest">连接测试</t-button>
          <t-button size="small" variant="outline" :loading="collecting" @click="doCollect">
            <template #icon><refresh-icon /></template>
            采集指标
          </t-button>
        </t-space>
      </div>
      <div class="info-grid">
        <div class="info-item">
          <span class="info-label">主机地址</span>
          <span class="info-value mono">{{ server?.host }}:{{ server?.port }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">登录用户</span>
          <span class="info-value">{{ server?.username }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">认证方式</span>
          <span class="info-value">{{ server?.auth_type === 'key' ? 'SSH 密钥' : '密码' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">连接状态</span>
          <span class="info-value">
            <t-tag :theme="statusTheme(server?.status)" variant="light" size="small">
              {{ statusText(server?.status) }}
            </t-tag>
          </span>
        </div>
        <div class="info-item">
          <span class="info-label">最后检测</span>
          <span class="info-value time">{{ server?.last_check_at ? dayjs(server.last_check_at).format('MM-DD HH:mm:ss') : '—' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">备注</span>
          <span class="info-value">{{ server?.remark || '—' }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import dayjs from 'dayjs'
import { useServerStore } from '@/stores/server'
import { getServer, testServer, collectMetrics, getMetrics } from '@/api/servers'
import type { Server, Metric } from '@/types/api'

const route = useRoute()
const serverStore = useServerStore()
const serverId = computed(() => Number(route.params.serverId))
const server = ref<Server | null>(null)
const metrics = ref<Metric[]>([])
const latestMetric = computed(() => metrics.value[0] ?? null)
const testing = ref(false)
const collecting = ref(false)

function statusTheme(s?: string) {
  return ({ online: 'success', offline: 'danger', unknown: 'default' } as Record<string, string>)[s ?? ''] ?? 'default'
}
function statusText(s?: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s ?? ''] ?? '—'
}
function progressColor(v: number) {
  if (v >= 90) return '#e34d59'
  if (v >= 70) return '#ed7b2f'
  return '#00a870'
}
function formatUptime(seconds?: number) {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return d > 0 ? `${d}天 ${h}小时` : `${h}小时 ${m}分`
}

async function doTest() {
  testing.value = true
  try {
    const res = await testServer(serverId.value)
    if (res.status === 'ok') { MessagePlugin.success('连接成功'); await serverStore.fetch() }
    else MessagePlugin.error(`连接失败：${res.error ?? '未知错误'}`)
  } catch { MessagePlugin.error('测试失败') }
  finally { testing.value = false }
}

async function doCollect() {
  collecting.value = true
  try {
    await collectMetrics(serverId.value)
    metrics.value = await getMetrics(serverId.value, 1)
    MessagePlugin.success('指标已更新')
  } catch { MessagePlugin.error('采集失败') }
  finally { collecting.value = false }
}

onMounted(async () => {
  server.value = await getServer(serverId.value)
  metrics.value = await getMetrics(serverId.value, 1)
})
</script>

<style scoped>
.metrics-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.metric-card {
  padding: 16px 20px;
}

.metric-label {
  font-size: 13px;
  color: var(--sh-text-secondary);
  margin-bottom: 8px;
}

.metric-value {
  font-size: 28px;
  font-weight: 600;
  line-height: 1;
  margin-bottom: 10px;
}

.metric-unit {
  font-size: 14px;
  font-weight: 400;
  margin-left: 2px;
}

.metric-sub {
  font-size: 12px;
  color: var(--sh-text-secondary);
  margin-top: 8px;
}

.info-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px 24px;
  padding: 16px 20px 20px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: var(--sh-text-secondary);
}

.info-value {
  font-size: 13px;
  color: var(--sh-text-primary);
  font-weight: 500;
}

.info-value.mono {
  font-family: "Cascadia Code", "JetBrains Mono", Menlo, monospace;
  font-size: 12px;
}

.info-value.time {
  font-size: 12px;
  color: var(--sh-text-secondary);
  font-weight: 400;
}
</style>
