<template>
  <div class="ov-page">
    <div class="metrics-row">
      <UiStatCard title="CPU 使用率" :value="(latestMetric?.cpu ?? 0).toFixed(1)" suffix="%">
        <NProgress type="line" :percentage="+(latestMetric?.cpu ?? 0).toFixed(1)" :show-indicator="false" :height="4" :color="progressColor(latestMetric?.cpu ?? 0)" class="ov-bar" />
      </UiStatCard>
      <UiStatCard title="内存使用率" :value="(latestMetric?.mem ?? 0).toFixed(1)" suffix="%">
        <NProgress type="line" :percentage="+(latestMetric?.mem ?? 0).toFixed(1)" :show-indicator="false" :height="4" :color="progressColor(latestMetric?.mem ?? 0)" class="ov-bar" />
      </UiStatCard>
      <UiStatCard title="磁盘使用率" :value="(latestMetric?.disk ?? 0).toFixed(1)" suffix="%">
        <NProgress type="line" :percentage="+(latestMetric?.disk ?? 0).toFixed(1)" :show-indicator="false" :height="4" :color="progressColor(latestMetric?.disk ?? 0)" class="ov-bar" />
      </UiStatCard>
      <UiStatCard title="系统负载" :value="latestMetric?.load1?.toFixed(2) ?? '—'" :hint="`运行时间：${formatUptime(latestMetric?.uptime)}`" />
    </div>

    <UiSection>
      <template #title>
        <span class="ov-title"><Server :size="16" /> 服务器信息</span>
      </template>
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="testing" @click="doTest">连接测试</UiButton>
        <UiButton variant="secondary" size="sm" :loading="collecting" @click="doCollect">
          <template #icon><RefreshCw :size="14" /></template>
          采集指标
        </UiButton>
      </template>
      <UiCard padding="md">
        <div class="ov-grid">
          <div class="ov-cell"><span class="lbl">主机地址</span><code class="mono">{{ server?.host }}:{{ server?.port }}</code></div>
          <div class="ov-cell"><span class="lbl">登录用户</span><span class="val">{{ server?.username }}</span></div>
          <div class="ov-cell"><span class="lbl">认证方式</span><span class="val">{{ server?.auth_type === 'key' ? 'SSH 密钥' : '密码' }}</span></div>
          <div class="ov-cell"><span class="lbl">连接状态</span><UiBadge :tone="statusTone(server?.status)">{{ statusText(server?.status) }}</UiBadge></div>
          <div class="ov-cell"><span class="lbl">最后检测</span><span class="val time">{{ server?.last_check_at ? dayjs(server.last_check_at).format('MM-DD HH:mm:ss') : '—' }}</span></div>
          <div class="ov-cell"><span class="lbl">备注</span><span class="val">{{ server?.remark || '—' }}</span></div>
        </div>
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { NProgress, useMessage } from 'naive-ui'
import { RefreshCw, Server } from 'lucide-vue-next'
import dayjs from 'dayjs'
import { useServerStore } from '@/stores/server'
import { getServer, testServer, collectMetrics, getMetrics } from '@/api/servers'
import type { Server as ServerT, Metric } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'

const route = useRoute()
const serverStore = useServerStore()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))
const server = ref<ServerT | null>(null)
const metrics = ref<Metric[]>([])
const latestMetric = computed(() => metrics.value[0] ?? null)
const testing = ref(false)
const collecting = ref(false)

function statusTone(s?: string): 'success' | 'danger' | 'neutral' {
  if (s === 'online') return 'success'
  if (s === 'offline') return 'danger'
  return 'neutral'
}
function statusText(s?: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s ?? ''] ?? '—'
}
function progressColor(v: number) {
  const css = getComputedStyle(document.documentElement)
  if (v >= 90) return css.getPropertyValue('--ui-danger').trim() || '#EF4444'
  if (v >= 70) return css.getPropertyValue('--ui-warning').trim() || '#F59E0B'
  return css.getPropertyValue('--ui-brand').trim() || '#3ECF8E'
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
    if (res.status === 'ok') { message.success('连接成功'); await serverStore.fetch() }
    else message.error(`连接失败：${res.error ?? '未知错误'}`)
  } catch { message.error('测试失败') }
  finally { testing.value = false }
}

async function doCollect() {
  collecting.value = true
  try {
    await collectMetrics(serverId.value)
    metrics.value = await getMetrics(serverId.value, 1)
    message.success('指标已更新')
  } catch { message.error('采集失败') }
  finally { collecting.value = false }
}

onMounted(async () => {
  server.value = await getServer(serverId.value)
  metrics.value = await getMetrics(serverId.value, 1)
})
</script>

<style scoped>
.ov-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.metrics-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
}
@media (max-width: 1080px) { .metrics-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 560px)  { .metrics-row { grid-template-columns: 1fr; } }

.ov-sub { font-size: var(--fs-xs); color: var(--ui-fg-3); margin-top: var(--space-2); }
.ov-bar { margin-top: var(--space-2); }

.ov-title { display: inline-flex; align-items: center; gap: var(--space-2); color: var(--ui-fg); }

.ov-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-3) var(--space-6);
}
@media (max-width: 720px) { .ov-grid { grid-template-columns: 1fr; } }

.ov-cell { display: flex; align-items: center; gap: var(--space-3); min-width: 0; }
.ov-cell .lbl { flex-shrink: 0; width: 80px; font-size: var(--fs-xs); color: var(--ui-fg-3); }
.ov-cell .val {
  font-size: var(--fs-sm); color: var(--ui-fg);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;
}
.ov-cell .val.time { font-size: var(--fs-xs); color: var(--ui-fg-3); }

.mono {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
}
</style>
