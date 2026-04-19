<template>
  <div class="dashboard">
    <!-- Stat cards -->
    <el-row :gutter="16" class="stat-row">
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card">
          <div class="stat-value">{{ total }}</div>
          <div class="stat-label">总服务器</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--green">
          <div class="stat-value">{{ online }}</div>
          <div class="stat-label">服务器在线</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--red">
          <div class="stat-value">{{ offline }}</div>
          <div class="stat-label">服务器离线</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--gray">
          <div class="stat-value">{{ unknown }}</div>
          <div class="stat-label">服务器未知</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--blue">
          <div class="stat-value">{{ appStore.apps.length }}</div>
          <div class="stat-label">总应用</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--green">
          <div class="stat-value">{{ appsOnline }}</div>
          <div class="stat-label">应用在线</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--red">
          <div class="stat-value">{{ appsOffline }}</div>
          <div class="stat-label">应用异常</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :xl="3">
        <div class="stat-card stat-card--gray">
          <div class="stat-value">{{ appsUnknown }}</div>
          <div class="stat-label">应用未知</div>
        </div>
      </el-col>
    </el-row>

    <!-- Server cards grid -->
    <div class="section-title">
      服务器状态
      <el-tag size="small" class="refresh-tag">每 {{ refreshInterval / 1000 }}s 刷新</el-tag>
    </div>
    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="item in overview" :key="item.id" :xs="24" :sm="12" :md="8" :xl="6" class="server-col">
        <div
          class="server-card"
          :class="{ 'server-card--active': selectedId === item.id }"
          @click="selectServer(item)"
        >
          <div class="server-card-header">
            <span class="server-name">{{ item.name }}</span>
            <el-tag size="small" :type="statusType(item.status)">{{ statusText(item.status) }}</el-tag>
          </div>
          <div class="server-host">{{ item.host }}:{{ item.port }}</div>

          <template v-if="item.metric">
            <div class="metric-row">
              <span class="metric-label">CPU</span>
              <el-progress
                :percentage="round(item.metric.cpu)"
                :color="progressColor(item.metric.cpu)"
                :stroke-width="6"
                class="metric-bar"
              />
              <span class="metric-val">{{ round(item.metric.cpu) }}%</span>
            </div>
            <div class="metric-row">
              <span class="metric-label">内存</span>
              <el-progress
                :percentage="round(item.metric.mem)"
                :color="progressColor(item.metric.mem)"
                :stroke-width="6"
                class="metric-bar"
              />
              <span class="metric-val">{{ round(item.metric.mem) }}%</span>
            </div>
            <div class="metric-row">
              <span class="metric-label">磁盘</span>
              <el-progress
                :percentage="round(item.metric.disk)"
                :color="progressColor(item.metric.disk)"
                :stroke-width="6"
                class="metric-bar"
              />
              <span class="metric-val">{{ round(item.metric.disk) }}%</span>
            </div>
            <div class="server-uptime">负载 {{ item.metric.load1.toFixed(2) }} · 运行 {{ formatUptime(item.metric.uptime) }}</div>
          </template>
          <div v-else class="no-metric">暂无指标数据</div>
        </div>
      </el-col>

      <el-col v-if="!loading && overview.length === 0" :span="24">
        <el-empty description="暂无服务器，请先在「服务器管理」中添加" />
      </el-col>
    </el-row>

    <!-- Trend chart for selected server -->
    <template v-if="selectedId">
      <div class="section-title">
        {{ selectedName }} — 趋势图（最近 {{ chartMetrics.length }} 个采样点）
      </div>
      <div ref="chartEl" class="trend-chart" />
    </template>

    <!-- Applications section -->
    <div class="section-title">
      应用状态
      <router-link to="/apps/create" class="add-link">+ 新建应用</router-link>
    </div>
    <el-row v-if="appStore.apps.length" :gutter="16">
      <el-col v-for="app in appStore.apps" :key="app.id" :xs="24" :sm="12" :md="8" :xl="6" class="server-col">
        <router-link :to="`/apps/${app.id}/overview`" class="app-card-link">
          <div class="app-card">
            <div class="app-card-header">
              <span class="server-name">{{ app.name }}</span>
              <el-tag :type="appStatusType(app.status)" size="small">{{ appStatusText(app.status) }}</el-tag>
            </div>
            <div class="app-card-desc">{{ app.description || app.domain || '—' }}</div>
            <div class="app-card-meta">
              <span v-if="app.site_name">Nginx: {{ app.site_name }}</span>
              <span v-if="app.container_name"> · 容器: {{ app.container_name }}</span>
            </div>
          </div>
        </router-link>
      </el-col>
    </el-row>
    <el-empty v-else description="暂无应用，点击「新建应用」开始" style="margin-top:20px" />
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

function appStatusType(s: string) {
  return ({ online: 'success', offline: 'danger', error: 'danger', unknown: 'info' } as Record<string, string>)[s] ?? 'info'
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

function statusType(s: string) {
  return { online: 'success', offline: 'danger', unknown: 'info' }[s] ?? 'info'
}
function statusText(s: string) {
  return { online: '在线', offline: '离线', unknown: '未知' }[s] ?? s
}
function round(n: number) { return Math.round(n) }
function progressColor(pct: number) {
  if (pct >= 90) return '#f56c6c'
  if (pct >= 70) return '#e6a23c'
  return '#67c23a'
}
function formatUptime(sec: number) {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  return d > 0 ? `${d}天${h}时` : `${h}时`
}

async function loadOverview() {
  if (!loading.value) loading.value = true
  try {
    overview.value = await getOverview()
  } finally {
    loading.value = false
  }
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
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#409eff' },
      { name: '内存', type: 'line', data: mem, smooth: true, symbol: 'none',
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#67c23a' },
      { name: '磁盘', type: 'line', data: disk, smooth: true, symbol: 'none',
        lineStyle: { width: 2 }, areaStyle: { opacity: 0.08 }, color: '#e6a23c' },
    ],
  }, true)
}

watch(chartMetrics, () => {
  if (chart) renderChart()
})

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
.dashboard { padding: 20px; }

.stat-row { margin-bottom: 20px; }
.stat-card {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  text-align: center;
  box-shadow: 0 1px 4px rgba(0,0,0,.08);
  border-top: 3px solid #409eff;
}
.stat-card--green { border-top-color: #67c23a; }
.stat-card--red   { border-top-color: #f56c6c; }
.stat-card--gray  { border-top-color: #909399; }
.stat-card--blue  { border-top-color: #409eff; }
.stat-value { font-size: 32px; font-weight: 700; color: #303133; line-height: 1.2; }
.stat-label { font-size: 13px; color: #909399; margin-top: 4px; }

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin: 20px 0 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.refresh-tag { font-weight: 400; }

.server-col { margin-bottom: 16px; }
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
.server-card--active { border-color: #409eff; }
.server-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.server-name { font-weight: 600; font-size: 14px; }
.server-host { font-size: 12px; color: #909399; margin-bottom: 10px; }

.metric-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}
.metric-label { font-size: 12px; color: #606266; width: 28px; flex-shrink: 0; }
.metric-bar { flex: 1; }
.metric-val  { font-size: 12px; color: #303133; width: 36px; text-align: right; flex-shrink: 0; }
.server-uptime { font-size: 11px; color: #c0c4cc; margin-top: 6px; }
.no-metric { font-size: 12px; color: #c0c4cc; padding: 12px 0; text-align: center; }

.trend-chart {
  width: 100%;
  height: 280px;
  background: #1a1a2e;
  border-radius: 8px;
  margin-bottom: 20px;
}

.add-link { font-size: 13px; font-weight: 400; color: #409eff; text-decoration: none; margin-left: 8px; }
.add-link:hover { text-decoration: underline; }

.app-card-link { text-decoration: none; display: block; }
.app-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  border: 2px solid transparent;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  transition: border-color .2s, box-shadow .2s;
}
.app-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,.1); border-color: #409eff; }
.app-card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.app-card-desc { font-size: 12px; color: #909399; margin-bottom: 6px; }
.app-card-meta { font-size: 11px; color: #c0c4cc; }
</style>
