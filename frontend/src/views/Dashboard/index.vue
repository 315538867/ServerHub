<template>
  <div class="dashboard">
    <!-- Stat cards -->
    <div class="stat-grid">
      <div class="stat-card stat-card--blue">
        <div class="stat-value">{{ total }}</div>
        <div class="stat-label">总服务器</div>
      </div>
      <div class="stat-card stat-card--green">
        <div class="stat-value">{{ online }}</div>
        <div class="stat-label">服务器在线</div>
      </div>
      <div class="stat-card stat-card--red">
        <div class="stat-value">{{ offline }}</div>
        <div class="stat-label">服务器离线</div>
      </div>
      <div class="stat-card stat-card--gray">
        <div class="stat-value">{{ unknown }}</div>
        <div class="stat-label">服务器未知</div>
      </div>
      <div class="stat-card stat-card--blue">
        <div class="stat-value">{{ appStore.apps.length }}</div>
        <div class="stat-label">总应用</div>
      </div>
      <div class="stat-card stat-card--green">
        <div class="stat-value">{{ appsOnline }}</div>
        <div class="stat-label">应用在线</div>
      </div>
      <div class="stat-card stat-card--red">
        <div class="stat-value">{{ appsOffline }}</div>
        <div class="stat-label">应用异常</div>
      </div>
      <div class="stat-card stat-card--gray">
        <div class="stat-value">{{ appsUnknown }}</div>
        <div class="stat-label">应用未知</div>
      </div>
    </div>

    <!-- Server cards grid -->
    <div class="section-header">
      <span class="section-title">服务器状态</span>
      <t-tag size="small" variant="light">每 {{ refreshInterval / 1000 }}s 刷新</t-tag>
    </div>
    <div v-loading="loading" class="server-grid">
      <div
        v-for="item in overview"
        :key="item.id"
        class="server-card"
        :class="{ 'server-card--active': selectedId === item.id }"
        @click="selectServer(item)"
      >
        <div class="server-card-header">
          <span class="server-name">{{ item.name }}</span>
          <t-tag size="small" :theme="statusTheme(item.status)" variant="light">{{ statusText(item.status) }}</t-tag>
        </div>
        <div class="server-host">{{ item.host }}:{{ item.port }}</div>

        <template v-if="item.metric">
          <div class="metric-row">
            <span class="metric-label">CPU</span>
            <t-progress
              :percentage="round(item.metric.cpu)"
              :color="progressColor(item.metric.cpu)"
              :stroke-width="6"
              class="metric-bar"
              size="small"
              :label="false"
            />
            <span class="metric-val">{{ round(item.metric.cpu) }}%</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">内存</span>
            <t-progress
              :percentage="round(item.metric.mem)"
              :color="progressColor(item.metric.mem)"
              :stroke-width="6"
              class="metric-bar"
              size="small"
              :label="false"
            />
            <span class="metric-val">{{ round(item.metric.mem) }}%</span>
          </div>
          <div class="metric-row">
            <span class="metric-label">磁盘</span>
            <t-progress
              :percentage="round(item.metric.disk)"
              :color="progressColor(item.metric.disk)"
              :stroke-width="6"
              class="metric-bar"
              size="small"
              :label="false"
            />
            <span class="metric-val">{{ round(item.metric.disk) }}%</span>
          </div>
          <div class="server-uptime">负载 {{ item.metric.load1.toFixed(2) }} · 运行 {{ formatUptime(item.metric.uptime) }}</div>
        </template>
        <div v-else class="no-metric">暂无指标数据</div>
      </div>
    </div>
    <t-empty v-if="!loading && overview.length === 0" description="暂无服务器，请先在「服务器管理」中添加" />

    <!-- Trend chart for selected server -->
    <template v-if="selectedId">
      <div class="section-header">
        <span class="section-title">{{ selectedName }} — 趋势图（最近 {{ chartMetrics.length }} 个采样点）</span>
      </div>
      <div ref="chartEl" class="trend-chart" />
    </template>

    <!-- Applications section -->
    <div class="section-header">
      <span class="section-title">应用状态</span>
      <router-link to="/apps/create" class="add-link">+ 新建应用</router-link>
    </div>
    <div v-if="appStore.apps.length" class="app-grid">
      <router-link
        v-for="app in appStore.apps"
        :key="app.id"
        :to="`/apps/${app.id}/overview`"
        class="app-card-link"
      >
        <div class="app-card">
          <div class="app-card-header">
            <span class="server-name">{{ app.name }}</span>
            <t-tag :theme="appStatusTheme(app.status)" variant="light" size="small">{{ appStatusText(app.status) }}</t-tag>
          </div>
          <div class="app-card-desc">{{ app.description || app.domain || '—' }}</div>
          <div class="app-card-meta">
            <span v-if="app.site_name">Nginx: {{ app.site_name }}</span>
            <span v-if="app.container_name"> · 容器: {{ app.container_name }}</span>
          </div>
        </div>
      </router-link>
    </div>
    <t-empty v-else description="暂无应用，点击「新建应用」开始" style="margin-top:20px" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { getOverview, getServerMetrics, type ServerOverview } from '@/api/metrics'
import { useAppStore } from '@/stores/app'
import type { Metric } from '@/types/api'

const appStore = useAppStore()

const overview = ref<ServerOverview[]>([])
const loading = ref(false)
const refreshInterval = 30_000

const total = computed(() => overview.value.length)
const online = computed(() => overview.value.filter(s => s.status === 'online').length)
const offline = computed(() => overview.value.filter(s => s.status === 'offline').length)
const unknown = computed(() => overview.value.filter(s => s.status === 'unknown').length)

const appsOnline = computed(() => appStore.apps.filter(a => a.status === 'online').length)
const appsOffline = computed(() => appStore.apps.filter(a => a.status === 'offline' || a.status === 'error').length)
const appsUnknown = computed(() => appStore.apps.filter(a => a.status === 'unknown').length)

function appStatusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', error: 'danger', unknown: 'default' } as Record<string, string>)[s] ?? 'default'
}
function appStatusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? s
}

const selectedId = ref<number | null>(null)
const selectedName = ref('')
const chartMetrics = ref<Metric[]>([])
const chartEl = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let timer: ReturnType<typeof setInterval> | null = null

function statusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', unknown: 'default' } as Record<string, string>)[s] ?? 'default'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string, string>)[s] ?? s
}
function round(n: number) { return Math.round(n) }
function progressColor(pct: number) {
  if (pct >= 90) return '#e34d59'
  if (pct >= 70) return '#ed7b2f'
  return '#00a870'
}
function formatUptime(sec: number) {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  return d > 0 ? `${d}天${h}时` : `${h}时`
}

async function loadOverview() {
  if (!loading.value) loading.value = true
  try { overview.value = await getOverview() }
  finally { loading.value = false }
}

async function selectServer(item: ServerOverview) {
  selectedId.value = item.id
  selectedName.value = item.name
  chartMetrics.value = await getServerMetrics(item.id, 60)
  await nextTick()
  renderChart()
}

function renderChart() {
  if (!chartEl.value) return
  if (!chart) {
    chart = echarts.init(chartEl.value, 'dark')
    window.addEventListener('resize', () => chart?.resize())
  }
  const metrics = [...chartMetrics.value].reverse()
  const times = metrics.map(m => dayjs(m.created_at).format('HH:mm:ss'))
  const cpu = metrics.map(m => +m.cpu.toFixed(1))
  const mem = metrics.map(m => +m.mem.toFixed(1))
  const disk = metrics.map(m => +m.disk.toFixed(1))

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    legend: { data: ['CPU', '内存', '磁盘'], top: 4 },
    grid: { left: 50, right: 20, top: 40, bottom: 40 },
    xAxis: { type: 'category', data: times, axisLabel: { rotate: 30, fontSize: 11 } },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%' } },
    series: [
      { name: 'CPU', type: 'line', data: cpu, smooth: true, symbol: 'none',
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#0052d9' },
      { name: '内存', type: 'line', data: mem, smooth: true, symbol: 'none',
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#00a870' },
      { name: '磁盘', type: 'line', data: disk, smooth: true, symbol: 'none',
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#ed7b2f' },
    ],
  }, true)
}

watch(chartMetrics, () => { if (chart) renderChart() })

onMounted(async () => {
  await Promise.all([loadOverview(), appStore.fetch()])
  timer = setInterval(loadOverview, refreshInterval)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  chart?.dispose()
})
</script>

<style scoped>
.dashboard {}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}
@media (max-width: 1200px) { .stat-grid { grid-template-columns: repeat(4, 1fr); } }
@media (max-width: 640px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }

.stat-card {
  background: #fff;
  border-radius: 8px;
  padding: 20px 16px;
  text-align: center;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  border-top: 3px solid #0052d9;
}
.stat-card--green { border-top-color: #00a870; }
.stat-card--red   { border-top-color: #e34d59; }
.stat-card--gray  { border-top-color: #8a94a6; }
.stat-card--blue  { border-top-color: #0052d9; }
.stat-value { font-size: 28px; font-weight: 700; color: var(--td-text-color-primary); line-height: 1.2; }
.stat-label { font-size: 12px; color: var(--td-text-color-secondary); margin-top: 4px; }

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 20px 0 12px;
}
.section-title { font-size: 15px; font-weight: 600; color: var(--td-text-color-primary); }

.server-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 4px;
}
@media (max-width: 1200px) { .server-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px) { .server-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) { .server-grid { grid-template-columns: 1fr; } }

.server-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  cursor: pointer;
  border: 2px solid transparent;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  transition: border-color .2s, box-shadow .2s;
}
.server-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,.1); }
.server-card--active { border-color: #0052d9; }
.server-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.server-name { font-weight: 600; font-size: 14px; }
.server-host { font-size: 12px; color: var(--td-text-color-secondary); margin-bottom: 10px; }

.metric-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}
.metric-label { font-size: 12px; color: var(--td-text-color-secondary); width: 28px; flex-shrink: 0; }
.metric-bar { flex: 1; }
.metric-val { font-size: 12px; color: var(--td-text-color-primary); width: 36px; text-align: right; flex-shrink: 0; }
.server-uptime { font-size: 11px; color: var(--td-text-color-placeholder); margin-top: 6px; }
.no-metric { font-size: 12px; color: var(--td-text-color-placeholder); padding: 12px 0; text-align: center; }

.trend-chart {
  width: 100%;
  height: 280px;
  background: #1a2332;
  border-radius: 8px;
  margin-bottom: 20px;
}

.add-link { font-size: 13px; font-weight: 400; color: var(--td-brand-color); text-decoration: none; }
.add-link:hover { text-decoration: underline; }

.app-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
@media (max-width: 1200px) { .app-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px) { .app-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) { .app-grid { grid-template-columns: 1fr; } }

.app-card-link { text-decoration: none; display: block; }
.app-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  border: 2px solid transparent;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  transition: border-color .2s, box-shadow .2s;
}
.app-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,.1); border-color: #0052d9; }
.app-card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.app-card-desc { font-size: 12px; color: var(--td-text-color-secondary); margin-bottom: 6px; }
.app-card-meta { font-size: 11px; color: var(--td-text-color-placeholder); }
</style>
