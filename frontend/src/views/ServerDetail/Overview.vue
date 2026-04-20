<template>
  <div class="overview-page">
    <t-descriptions :column="3" bordered style="margin-bottom:16px">
      <t-descriptions-item label="主机地址">{{ server?.host }}:{{ server?.port }}</t-descriptions-item>
      <t-descriptions-item label="登录用户">{{ server?.username }}</t-descriptions-item>
      <t-descriptions-item label="认证方式">{{ server?.auth_type === 'key' ? 'SSH 密钥' : '密码' }}</t-descriptions-item>
      <t-descriptions-item label="状态">
        <t-tag :theme="statusTheme(server?.status)" variant="light" size="small">{{ server?.status ?? '—' }}</t-tag>
      </t-descriptions-item>
      <t-descriptions-item label="最后检测">{{ server?.last_check_at ?? '—' }}</t-descriptions-item>
      <t-descriptions-item label="备注">{{ server?.remark || '—' }}</t-descriptions-item>
    </t-descriptions>
    <t-space style="margin-bottom:20px">
      <t-button :loading="testing" variant="outline" @click="doTest">连接测试</t-button>
      <t-button :loading="collecting" variant="outline" @click="doCollect">
        <template #icon><refresh-icon /></template>
        采集指标
      </t-button>
    </t-space>

    <div class="gauge-row">
      <t-card shadow="never" class="gauge-card">
        <div class="gauge-label">CPU 使用率</div>
        <t-progress :percentage="+(latestMetric?.cpu ?? 0).toFixed(1)" :color="progressColor(latestMetric?.cpu ?? 0)" :stroke-width="14" />
        <div class="gauge-val">{{ (latestMetric?.cpu ?? 0).toFixed(1) }}%</div>
      </t-card>
      <t-card shadow="never" class="gauge-card">
        <div class="gauge-label">内存使用率</div>
        <t-progress :percentage="+(latestMetric?.mem ?? 0).toFixed(1)" :color="progressColor(latestMetric?.mem ?? 0)" :stroke-width="14" />
        <div class="gauge-val">{{ (latestMetric?.mem ?? 0).toFixed(1) }}%</div>
      </t-card>
      <t-card shadow="never" class="gauge-card">
        <div class="gauge-label">磁盘使用率</div>
        <t-progress :percentage="+(latestMetric?.disk ?? 0).toFixed(1)" :color="progressColor(latestMetric?.disk ?? 0)" :stroke-width="14" />
        <div class="gauge-val">{{ (latestMetric?.disk ?? 0).toFixed(1) }}%</div>
      </t-card>
    </div>

    <div class="stat-row">
      <t-card shadow="never" header="负载 (1min)" class="stat-card">
        <span class="stat-val">{{ latestMetric?.load1?.toFixed(2) ?? '—' }}</span>
      </t-card>
      <t-card shadow="never" header="运行时间" class="stat-card">
        <span class="stat-val">{{ formatUptime(latestMetric?.uptime) }}</span>
      </t-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
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
.overview-page { padding: 4px 0; }
.gauge-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}
.gauge-card { text-align: center; }
.gauge-label { font-size: 13px; color: var(--td-text-color-secondary); margin-bottom: 10px; font-weight: 600; }
.gauge-val { text-align: right; font-size: 13px; color: var(--td-text-color-primary); margin-top: 6px; }
.stat-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}
.stat-val { font-size: 28px; font-weight: 600; color: var(--td-text-color-primary); }
</style>
