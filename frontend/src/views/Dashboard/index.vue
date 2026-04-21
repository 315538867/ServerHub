<template>
  <div class="page-container dash">
    <!-- 顶部统计 -->
    <div class="dash__stats">
      <UiStatCard label="服务器总数" :value="total" tone="brand" :trend="sparkTotal" />
      <UiStatCard label="服务器在线" :value="online" tone="success" :delta="0" :delta-unit="`% (${onlinePct}%)`" />
      <UiStatCard label="服务器离线" :value="offline" :tone="offline > 0 ? 'danger' : 'brand'" />
      <UiStatCard label="应用总数" :value="appStore.apps.length" tone="brand" />
      <UiStatCard label="应用在线" :value="appsOnline" tone="success" />
      <UiStatCard label="应用异常" :value="appsOffline" :tone="appsOffline > 0 ? 'danger' : 'brand'" />
    </div>

    <!-- 服务器状态 -->
    <UiSection>
      <template #title>
        <server-icon class="dash__tic" /> 服务器状态
      </template>
      <template #extra>
        <UiBadge tone="neutral" variant="soft">每 {{ refreshInterval / 1000 }}s 刷新</UiBadge>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="loadOverview">刷新</UiButton>
      </template>
      <EmptyBlock v-if="!loading && overview.length === 0" title="暂无服务器" description="先去「服务器管理」添加一台" />
      <div v-else class="dash__grid">
        <div
          v-for="(item, idx) in overview"
          :key="item.id"
          class="srv-card"
          :class="{ 'is-active': selectedId === item.id }"
          :style="{ animationDelay: `${idx * 40}ms` }"
          @click="selectServer(item)"
        >
          <div class="srv-card__head">
            <div class="srv-card__name-row">
              <StatusDot :status="item.status" :size="8" pulse />
              <span class="srv-card__name">{{ item.name }}</span>
            </div>
            <UiBadge :tone="statusTone(item.status)" variant="soft">{{ statusText(item.status) }}</UiBadge>
          </div>
          <div class="srv-card__host">
            <code>{{ item.host }}:{{ item.port }}</code>
          </div>

          <template v-if="item.metric">
            <MetricRow label="CPU"  :value="round(item.metric.cpu)" />
            <MetricRow label="内存" :value="round(item.metric.mem)" />
            <MetricRow label="磁盘" :value="round(item.metric.disk)" />
            <div class="srv-card__foot">
              负载 <b>{{ item.metric.load1.toFixed(2) }}</b>
              <span class="srv-card__sep">·</span>
              运行 <b>{{ formatUptime(item.metric.uptime) }}</b>
            </div>
          </template>
          <div v-else class="srv-card__nometric">暂无指标</div>
        </div>
      </div>
    </UiSection>

    <!-- 趋势图 -->
    <UiSection v-if="selectedId">
      <template #title>
        <chart-icon class="dash__tic" /> {{ selectedName }} · 资源趋势
      </template>
      <template #extra>
        <UiBadge tone="brand" variant="soft">{{ chartMetrics.length }} 采样点</UiBadge>
      </template>
      <div ref="chartEl" class="dash__chart" />
    </UiSection>

    <!-- 应用状态 -->
    <UiSection>
      <template #title>
        <app-icon class="dash__tic" /> 应用状态
      </template>
      <template #extra>
        <router-link to="/apps/create">
          <UiButton variant="primary" size="sm">
            <template #icon><add-icon /></template>
            新建应用
          </UiButton>
        </router-link>
      </template>
      <EmptyBlock v-if="!appStore.apps.length" title="暂无应用" description="点击「新建应用」开始部署" />
      <div v-else class="dash__apps">
        <router-link
          v-for="(app, idx) in appStore.apps"
          :key="app.id"
          :to="`/apps/${app.id}/overview`"
          class="app-row"
          :style="{ animationDelay: `${idx * 30}ms` }"
        >
          <div class="app-row__left">
            <StatusDot :status="app.status" :size="8" />
            <div class="app-row__info">
              <span class="app-row__name">{{ app.name }}</span>
              <span class="app-row__desc">{{ app.description || app.domain || '—' }}</span>
            </div>
          </div>
          <div class="app-row__right">
            <code v-if="app.site_name" class="app-row__meta">Nginx · {{ app.site_name }}</code>
            <code v-if="app.container_name" class="app-row__meta">🐳 {{ app.container_name }}</code>
            <UiBadge :tone="appStatusTone(app.status)" variant="soft">{{ appStatusText(app.status) }}</UiBadge>
          </div>
        </router-link>
      </div>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, h } from 'vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { getOverview, getServerMetrics, type ServerOverview } from '@/api/metrics'
import { useAppStore } from '@/stores/app'
import type { Metric } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'
import { useThemeStore } from '@/stores/theme'
import {
  ServerIcon, AppIcon, AddIcon, ChartIcon,
} from 'tdesign-icons-vue-next'

const appStore = useAppStore()
const theme = useThemeStore()
const overview = ref<ServerOverview[]>([])
const loading = ref(false)
const refreshInterval = 30_000

const total   = computed(() => overview.value.length)
const online  = computed(() => overview.value.filter(s => s.status === 'online').length)
const offline = computed(() => overview.value.filter(s => s.status === 'offline').length)
const onlinePct = computed(() => total.value ? Math.round(online.value / total.value * 100) : 0)

const appsOnline  = computed(() => appStore.apps.filter(a => a.status === 'online').length)
const appsOffline = computed(() => appStore.apps.filter(a => a.status === 'offline' || a.status === 'error').length)

const sparkTotal = computed(() => Array.from({ length: 12 }, () => total.value || 1))

function appStatusTone(s: string): any {
  return ({ online: 'success', offline: 'danger', error: 'danger' } as Record<string,string>)[s] ?? 'neutral'
}
function appStatusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string,string>)[s] ?? s
}
function statusTone(s: string): any {
  return ({ online: 'success', offline: 'danger' } as Record<string,string>)[s] ?? 'neutral'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', unknown: '未知' } as Record<string,string>)[s] ?? s
}
function round(n: number) { return Math.round(n) }
function formatUptime(sec: number) {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  return d > 0 ? `${d}天${h}时` : `${h}时`
}

// ─── 内联 MetricRow：动画进度条 + 颜色分层 ───
const MetricRow = (props: { label: string; value: number }) => {
  const color = props.value >= 90 ? 'var(--ui-danger)'
              : props.value >= 70 ? 'var(--ui-warning)'
              : 'var(--ui-success)'
  return h('div', { class: 'mr' }, [
    h('span', { class: 'mr__label' }, props.label),
    h('div', { class: 'mr__track' }, [
      h('div', { class: 'mr__fill', style: { width: `${props.value}%`, background: color } }),
    ]),
    h('span', { class: 'mr__val', style: { color } }, `${props.value}%`),
  ])
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
  const isDark = theme.isDark
  const fgDim = isDark ? '#a0a4b5' : '#8b8f9c'
  const grid = isDark ? '#2a2d3a' : '#edeef2'
  const metrics = [...chartMetrics.value].reverse()
  const times = metrics.map(m => dayjs(m.created_at).format('HH:mm'))
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'line' } },
    legend: { data: ['CPU', '内存', '磁盘'], top: 4, textStyle: { fontSize: 11, color: fgDim } },
    grid: { left: 44, right: 16, top: 32, bottom: 28 },
    xAxis: { type: 'category', data: times, axisLabel: { fontSize: 10, color: fgDim }, axisLine: { lineStyle: { color: grid } } },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%', fontSize: 10, color: fgDim }, splitLine: { lineStyle: { color: grid } } },
    series: [
      { name: 'CPU',  type: 'line', data: metrics.map(m => +m.cpu.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 }, color: '#5E6AD2' },
      { name: '内存', type: 'line', data: metrics.map(m => +m.mem.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 }, color: '#46B1C9' },
      { name: '磁盘', type: 'line', data: metrics.map(m => +m.disk.toFixed(1)), smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 }, color: '#F59E0B' },
    ],
  }, true)
}

watch(chartMetrics, () => { if (chart) renderChart() })
watch(() => theme.isDark, () => { if (chart) renderChart() })

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
.dash {
  display: flex; flex-direction: column;
  gap: var(--ui-space-4);
  padding: var(--ui-space-4) var(--ui-space-5);
}

.dash__stats {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--ui-space-3);
}
@media (max-width: 1400px) { .dash__stats { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 768px)  { .dash__stats { grid-template-columns: repeat(2, 1fr); } }

.dash__tic { font-size: 14px; color: var(--ui-brand); margin-right: 4px; }

.dash__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--ui-space-3);
}

.srv-card {
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  padding: var(--ui-space-3) var(--ui-space-4);
  cursor: pointer;
  transition: border-color var(--ui-dur-fast) var(--ui-ease-standard),
              box-shadow var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard),
              background var(--ui-dur-fast) var(--ui-ease-standard);
  background: var(--ui-bg-surface);
  opacity: 0;
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard) forwards;
}
.srv-card:hover {
  border-color: var(--ui-brand);
  box-shadow: var(--ui-shadow-md), 0 0 0 3px var(--ui-brand-ring);
  transform: translateY(-2px);
}
.srv-card.is-active {
  border-color: var(--ui-brand);
  background: var(--ui-brand-soft);
}

.srv-card__head {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 4px;
}
.srv-card__name-row { display: flex; align-items: center; gap: 8px; min-width: 0; }
.srv-card__name {
  font-weight: var(--ui-fw-semibold); font-size: var(--ui-fs-sm);
  color: var(--ui-fg);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.srv-card__host {
  font-size: var(--ui-fs-xs);
  margin-bottom: 10px;
}
.srv-card__host code {
  font-family: var(--ui-font-mono);
  color: var(--ui-fg-3);
  background: var(--ui-bg-subtle);
  padding: 1px 6px;
  border-radius: var(--ui-radius-sm);
  border: 1px solid var(--ui-border-subtle);
}

.srv-card__foot {
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--ui-border-subtle);
}
.srv-card__foot b { color: var(--ui-fg); font-weight: var(--ui-fw-semibold); }
.srv-card__sep { color: var(--ui-fg-4); margin: 0 6px; }
.srv-card__nometric {
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-placeholder);
  padding: var(--ui-space-3) 0;
  text-align: center;
}

/* MetricRow */
:deep(.mr) {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 6px;
}
:deep(.mr__label) {
  font-size: var(--ui-fs-2xs);
  color: var(--ui-fg-3);
  width: 28px;
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}
:deep(.mr__track) {
  flex: 1;
  height: 4px;
  background: var(--ui-bg-subtle);
  border-radius: 3px;
  overflow: hidden;
  position: relative;
}
:deep(.mr__fill) {
  height: 100%;
  border-radius: 3px;
  transition: width var(--ui-dur-slow) var(--ui-ease-standard);
  box-shadow: 0 0 6px currentColor;
}
:deep(.mr__val) {
  font-size: var(--ui-fs-2xs);
  width: 32px; text-align: right;
  font-weight: var(--ui-fw-semibold);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.dash__chart { width: 100%; height: 260px; }

/* app list */
.dash__apps { display: flex; flex-direction: column; }
.app-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--ui-radius-md);
  text-decoration: none;
  transition: background var(--ui-dur-fast), transform var(--ui-dur-fast);
  opacity: 0;
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard) forwards;
}
.app-row:hover { background: var(--ui-bg-hover); transform: translateX(2px); }

.app-row__left { display: flex; align-items: center; gap: 10px; min-width: 0; }
.app-row__info { display: flex; flex-direction: column; min-width: 0; }
.app-row__name { font-size: var(--ui-fs-sm); font-weight: var(--ui-fw-medium); color: var(--ui-fg); }
.app-row__desc { font-size: var(--ui-fs-xs); color: var(--ui-fg-3); }

.app-row__right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.app-row__meta {
  font-family: var(--ui-font-mono);
  font-size: var(--ui-fs-2xs);
  color: var(--ui-fg-3);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border-subtle);
  padding: 1px 6px;
  border-radius: var(--ui-radius-sm);
}
</style>
