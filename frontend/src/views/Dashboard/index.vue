<template>
  <div class="page-container dashboard">

    <!-- ── 顶部概览统计 ── -->
    <div class="stat-row">
      <stat-card label="服务器总数" :value="total"   :icon="ServerIcon"       color="blue"   />
      <stat-card label="服务器在线" :value="online"  :icon="CheckCircleIcon"  color="green"  />
      <stat-card label="服务器离线" :value="offline" :icon="CloseCircleIcon"  color="red"    />
      <stat-card label="应用总数"   :value="appStore.apps.length" :icon="AppIcon" color="blue" />
      <stat-card label="应用在线"   :value="appsOnline"  :icon="CheckCircleIcon" color="green" />
      <stat-card label="应用异常"   :value="appsOffline" :icon="ErrorCircleIcon" color="red"   />
    </div>

    <!-- ── 服务器状态 ── -->
    <div class="section-block">
      <div class="section-title">
        <div class="section-title-left">
          <server-icon class="section-icon" />
          服务器状态
        </div>
        <div class="section-title-right">
          <t-tag size="small" variant="light" theme="default">每 {{ refreshInterval / 1000 }}s 刷新</t-tag>
          <t-button size="small" variant="outline" :loading="loading" @click="loadOverview">刷新</t-button>
        </div>
      </div>
      <div class="section-body">
        <t-empty v-if="!loading && overview.length === 0" description="暂无服务器" style="padding: var(--sh-space-xl) 0" />
        <div v-else class="server-grid">
          <div
            v-for="item in overview"
            :key="item.id"
            class="server-card"
            :class="{ 'server-card--active': selectedId === item.id }"
            @click="selectServer(item)"
          >
            <div class="server-card-top">
              <div class="server-card-name-row">
                <span class="status-dot" :class="item.status" />
                <span class="server-card-name">{{ item.name }}</span>
              </div>
              <t-tag size="small" :theme="statusTheme(item.status)" variant="light">
                {{ statusText(item.status) }}
              </t-tag>
            </div>
            <div class="server-card-host">{{ item.host }}:{{ item.port }}</div>

            <template v-if="item.metric">
              <div class="metric-row">
                <span class="metric-label">CPU</span>
                <div class="metric-bar-wrap">
                  <div class="metric-bar-inner" :style="{ width: round(item.metric.cpu) + '%', background: barColor(item.metric.cpu) }" />
                </div>
                <span class="metric-val" :style="{ color: barColor(item.metric.cpu) }">{{ round(item.metric.cpu) }}%</span>
              </div>
              <div class="metric-row">
                <span class="metric-label">内存</span>
                <div class="metric-bar-wrap">
                  <div class="metric-bar-inner" :style="{ width: round(item.metric.mem) + '%', background: barColor(item.metric.mem) }" />
                </div>
                <span class="metric-val" :style="{ color: barColor(item.metric.mem) }">{{ round(item.metric.mem) }}%</span>
              </div>
              <div class="metric-row">
                <span class="metric-label">磁盘</span>
                <div class="metric-bar-wrap">
                  <div class="metric-bar-inner" :style="{ width: round(item.metric.disk) + '%', background: barColor(item.metric.disk) }" />
                </div>
                <span class="metric-val" :style="{ color: barColor(item.metric.disk) }">{{ round(item.metric.disk) }}%</span>
              </div>
              <div class="server-card-footer">
                负载 <b>{{ item.metric.load1.toFixed(2) }}</b> · 运行 <b>{{ formatUptime(item.metric.uptime) }}</b>
              </div>
            </template>
            <div v-else class="no-metric">暂无指标数据</div>
          </div>
        </div>
      </div>
    </div>

    <!-- ── 趋势图 ── -->
    <div v-if="selectedId" class="section-block">
      <div class="section-title">
        <div class="section-title-left">
          <chart-icon class="section-icon" />
          {{ selectedName }} — 资源趋势（最近 {{ chartMetrics.length }} 个采样点）
        </div>
      </div>
      <div class="section-body">
        <div ref="chartEl" class="trend-chart" />
      </div>
    </div>

    <!-- ── 应用状态 ── -->
    <div class="section-block">
      <div class="section-title">
        <div class="section-title-left">
          <app-icon class="section-icon" />
          应用状态
        </div>
        <div class="section-title-right">
          <router-link to="/apps/create">
            <t-button size="small" theme="primary">
              <template #icon><add-icon /></template>
              新建应用
            </t-button>
          </router-link>
        </div>
      </div>
      <div class="section-body">
        <t-empty v-if="!appStore.apps.length" description="暂无应用，点击「新建应用」开始" style="padding: var(--sh-space-xl) 0" />
        <div v-else class="app-list">
          <router-link
            v-for="app in appStore.apps"
            :key="app.id"
            :to="`/apps/${app.id}/overview`"
            class="app-row"
          >
            <div class="app-row-left">
              <span class="status-dot" :class="app.status" />
              <div class="app-row-info">
                <span class="app-row-name">{{ app.name }}</span>
                <span class="app-row-desc">{{ app.description || app.domain || '—' }}</span>
              </div>
            </div>
            <div class="app-row-right">
              <span v-if="app.site_name" class="app-row-meta">Nginx: {{ app.site_name }}</span>
              <span v-if="app.container_name" class="app-row-meta">容器: {{ app.container_name }}</span>
              <t-tag :theme="appStatusTheme(app.status)" variant="light" size="small">
                {{ appStatusText(app.status) }}
              </t-tag>
            </div>
          </router-link>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { getOverview, getServerMetrics, type ServerOverview } from '@/api/metrics'
import { useAppStore } from '@/stores/app'
import type { Metric } from '@/types/api'
import StatCard from '@/components/StatCard.vue'
import {
  ServerIcon, CheckCircleIcon, CloseCircleIcon, AppIcon, AddIcon,
  ErrorCircleIcon, ChartIcon,
} from 'tdesign-icons-vue-next'

const appStore = useAppStore()
const overview = ref<ServerOverview[]>([])
const loading = ref(false)
const refreshInterval = 30_000

const total   = computed(() => overview.value.length)
const online  = computed(() => overview.value.filter(s => s.status === 'online').length)
const offline = computed(() => overview.value.filter(s => s.status === 'offline').length)

const appsOnline  = computed(() => appStore.apps.filter(a => a.status === 'online').length)
const appsOffline = computed(() => appStore.apps.filter(a => a.status === 'offline' || a.status === 'error').length)

function appStatusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', error: 'danger', unknown: 'default' } as Record<string,string>)[s] ?? 'default'
}
function appStatusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string,string>)[s] ?? s
}
function statusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', unknown: 'default' } as Record<string,string>)[s] ?? 'default'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string,string>)[s] ?? s
}
function round(n: number) { return Math.round(n) }
function barColor(pct: number) {
  if (pct >= 90) return '#e34d59'
  if (pct >= 70) return '#ed7b2f'
  return '#00a870'
}
function formatUptime(sec: number) {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  return d > 0 ? `${d}天${h}时` : `${h}时`
}

const selectedId = ref<number | null>(null)
const selectedName = ref('')
const chartMetrics = ref<Metric[]>([])
const chartEl = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let timer: ReturnType<typeof setInterval> | null = null

async function loadOverview() {
  loading.value = true
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
    chart = echarts.init(chartEl.value)
    window.addEventListener('resize', () => chart?.resize())
  }
  const metrics = [...chartMetrics.value].reverse()
  const times = metrics.map(m => dayjs(m.created_at).format('HH:mm'))
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'line' } },
    legend: { data: ['CPU', '内存', '磁盘'], top: 4, textStyle: { fontSize: 12 } },
    grid: { left: 48, right: 16, top: 36, bottom: 32 },
    xAxis: { type: 'category', data: times, axisLabel: { fontSize: 11, color: '#888' }, axisLine: { lineStyle: { color: '#e7e7e7' } } },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%', fontSize: 11, color: '#888' }, splitLine: { lineStyle: { color: '#f0f0f0' } } },
    series: [
      { name: 'CPU',  type: 'line', data: metrics.map(m => +m.cpu.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.06 }, color: '#0052d9' },
      { name: '内存', type: 'line', data: metrics.map(m => +m.mem.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.06 }, color: '#00a870' },
      { name: '磁盘', type: 'line', data: metrics.map(m => +m.disk.toFixed(1)), smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.06 }, color: '#ed7b2f' },
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
.dashboard { display: flex; flex-direction: column; gap: var(--sh-space-md); }

/* ── stat row ── */
.stat-row {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--sh-space-md);
}
@media (max-width: 1400px) { .stat-row { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 768px)  { .stat-row { grid-template-columns: repeat(2, 1fr); } }

/* ── section block ── */
.section-block {
  background: var(--sh-card-bg);
  border: var(--sh-card-border);
  border-radius: var(--sh-card-radius);
  box-shadow: var(--sh-card-shadow);
  overflow: hidden;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sh-space-md) var(--sh-space-lg);
  border-bottom: 1px solid var(--sh-border);
  font-size: 14px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.section-title-left {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
}
.section-title-right { display: flex; align-items: center; gap: var(--sh-space-sm); }
.section-icon { font-size: 15px; color: var(--sh-blue); }

.section-body { padding: var(--sh-space-md) var(--sh-space-lg); }

/* ── server grid ── */
.server-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--sh-space-md);
}
@media (max-width: 1400px) { .server-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px)  { .server-grid { grid-template-columns: repeat(2, 1fr); } }

.server-card {
  border: 1px solid var(--sh-border);
  border-radius: 6px;
  padding: var(--sh-space-md);
  cursor: pointer;
  transition: border-color .15s, box-shadow .15s;
  background: #fafafa;
}
.server-card:hover { box-shadow: 0 2px 10px rgba(0,0,0,.08); border-color: #c5cfe8; background: #fff; }
.server-card--active { border-color: var(--sh-blue) !important; background: var(--sh-blue-bg) !important; }

.server-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sh-space-xs);
}
.server-card-name-row { display: flex; align-items: center; gap: var(--sh-space-sm); min-width: 0; }
.server-card-name { font-weight: 600; font-size: 13.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.server-card-host { font-size: 12px; color: var(--sh-text-secondary); margin-bottom: var(--sh-space-sm); }

.metric-row { display: flex; align-items: center; gap: var(--sh-space-sm); margin-bottom: var(--sh-space-sm); }
.metric-label { font-size: 11px; color: var(--sh-text-secondary); width: 26px; flex-shrink: 0; }
.metric-bar-wrap {
  flex: 1;
  height: 4px;
  background: #ebebeb;
  border-radius: 2px;
  overflow: hidden;
}
.metric-bar-inner { height: 100%; border-radius: 2px; transition: width .4s; }
.metric-val { font-size: 11px; width: 34px; text-align: right; font-weight: 600; flex-shrink: 0; }

.server-card-footer {
  font-size: 11px;
  color: var(--sh-text-secondary);
  margin-top: var(--sh-space-sm);
  padding-top: var(--sh-space-sm);
  border-top: 1px solid var(--sh-border);
}
.server-card-footer b { color: var(--sh-text-primary); }
.no-metric { font-size: 12px; color: var(--sh-text-placeholder); padding: var(--sh-space-md) 0; text-align: center; }

/* ── trend chart ── */
.trend-chart {
  width: 100%;
  height: 260px;
}

/* ── app list ── */
.app-list { display: flex; flex-direction: column; gap: 0; }
.app-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sh-space-sm) var(--sh-space-xs);
  border-bottom: 1px solid #f5f5f5;
  text-decoration: none;
  border-radius: 4px;
  transition: background .12s;
}
.app-row:last-child { border-bottom: none; }
.app-row:hover { background: #f7f8fc; }

.app-row-left { display: flex; align-items: center; gap: var(--sh-space-sm); min-width: 0; }
.app-row-info { display: flex; flex-direction: column; gap: var(--sh-space-xs); min-width: 0; }
.app-row-name { font-size: 13.5px; font-weight: 500; color: var(--sh-text-primary); }
.app-row-desc { font-size: 12px; color: var(--sh-text-secondary); }

.app-row-right { display: flex; align-items: center; gap: var(--sh-space-sm); flex-shrink: 0; }
.app-row-meta { font-size: 12px; color: var(--sh-text-secondary); }
</style>
