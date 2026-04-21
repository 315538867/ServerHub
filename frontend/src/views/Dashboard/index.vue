<template>
  <div class="page dash">
    <!-- 顶部统计 -->
    <div class="dash__stats">
      <UiStatCard title="服务器总数" :value="total" />
      <UiStatCard title="服务器在线" :value="online" :hint="`占比 ${onlinePct}%`" />
      <UiStatCard title="服务器离线" :value="offline" />
      <UiStatCard title="应用总数" :value="appStore.apps.length" />
      <UiStatCard title="应用在线" :value="appsOnline" />
      <UiStatCard title="应用异常" :value="appsOffline" />
    </div>

    <!-- 服务器状态 -->
    <UiSection>
      <template #title>
        <span class="dash__sec-title"><Server :size="16" /> 服务器状态</span>
      </template>
      <template #extra>
        <UiBadge tone="neutral">每 {{ refreshInterval / 1000 }}s 刷新</UiBadge>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="loadOverview">刷新</UiButton>
      </template>
      <EmptyBlock v-if="!loading && overview.length === 0" title="暂无服务器" description="先去「服务器管理」添加一台" />
      <div v-else class="dash__grid">
        <div
          v-for="item in overview"
          :key="item.id"
          class="srv-card"
          :class="{ 'is-active': selectedId === item.id }"
          @click="selectServer(item)"
        >
          <div class="srv-card__head">
            <div class="srv-card__name-row">
              <StatusDot :status="item.status" :size="8" :pulse="item.status === 'online'" />
              <span class="srv-card__name">{{ item.name }}</span>
            </div>
            <UiBadge :tone="statusTone(item.status)">{{ statusText(item.status) }}</UiBadge>
          </div>
          <div class="srv-card__host"><code>{{ item.host }}:{{ item.port }}</code></div>

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
        <span class="dash__sec-title"><LineChart :size="16" /> {{ selectedName }} · 资源趋势</span>
      </template>
      <template #extra>
        <UiBadge tone="brand">{{ chartMetrics.length }} 采样点</UiBadge>
      </template>
      <UiCard padding="md">
        <div ref="chartEl" class="dash__chart" />
      </UiCard>
    </UiSection>

    <!-- 应用状态 -->
    <UiSection>
      <template #title>
        <span class="dash__sec-title"><Package :size="16" /> 应用状态</span>
      </template>
      <template #extra>
        <router-link to="/apps/create">
          <UiButton variant="primary" size="sm">
            <template #icon><Plus :size="14" /></template>
            新建应用
          </UiButton>
        </router-link>
      </template>
      <EmptyBlock v-if="!appStore.apps.length" title="暂无应用" description="点击「新建应用」开始部署" />
      <UiCard v-else padding="none">
        <div class="dash__apps">
          <router-link
            v-for="app in appStore.apps"
            :key="app.id"
            :to="`/apps/${app.id}/overview`"
            class="app-row"
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
              <code v-if="app.container_name" class="app-row__meta">{{ app.container_name }}</code>
              <UiBadge :tone="appStatusTone(app.status)">{{ appStatusText(app.status) }}</UiBadge>
            </div>
          </router-link>
        </div>
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, h } from 'vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { Server, LineChart, Package, Plus } from 'lucide-vue-next'
import { getOverview, getServerMetrics, type ServerOverview } from '@/api/metrics'
import { useAppStore } from '@/stores/app'
import type { Metric } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'
import { useThemeStore } from '@/stores/theme'

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
  const fgDim = isDark ? '#71717A' : '#71717A'
  const grid = isDark ? '#27272A' : '#E4E4E7'
  const metrics = [...chartMetrics.value].reverse()
  const times = metrics.map(m => dayjs(m.created_at).format('HH:mm'))
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: ['CPU', '内存', '磁盘'], top: 4, textStyle: { fontSize: 11, color: fgDim } },
    grid: { left: 44, right: 16, top: 32, bottom: 28 },
    xAxis: { type: 'category', data: times, axisLabel: { fontSize: 10, color: fgDim }, axisLine: { lineStyle: { color: grid } } },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { formatter: '{value}%', fontSize: 10, color: fgDim }, splitLine: { lineStyle: { color: grid } } },
    series: [
      { name: 'CPU',  type: 'line', data: metrics.map(m => +m.cpu.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 }, color: '#3ECF8E' },
      { name: '内存', type: 'line', data: metrics.map(m => +m.mem.toFixed(1)),  smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.12 }, color: '#3B82F6' },
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
  gap: var(--space-6);
  padding: var(--space-6);
}

.dash__stats {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--space-3);
}
@media (max-width: 1400px) { .dash__stats { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 768px)  { .dash__stats { grid-template-columns: repeat(2, 1fr); } }

.dash__sec-title {
  display: inline-flex; align-items: center; gap: var(--space-2);
  color: var(--ui-fg);
  font-size: var(--fs-md); font-weight: var(--fw-semibold);
}
.dash__sec-title :deep(svg) { color: var(--ui-brand); }

.dash__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-3);
}

.srv-card {
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  cursor: pointer;
  background: var(--ui-bg-1);
  transition: border-color var(--dur-fast) var(--ease),
              box-shadow var(--dur-fast) var(--ease),
              background var(--dur-fast) var(--ease);
}
.srv-card:hover {
  border-color: var(--ui-border-strong);
  box-shadow: var(--shadow-sm);
}
.srv-card.is-active {
  border-color: var(--ui-brand);
  box-shadow: var(--shadow-ring);
}

.srv-card__head {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-1);
}
.srv-card__name-row { display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
.srv-card__name {
  font-weight: var(--fw-semibold); font-size: var(--fs-sm);
  color: var(--ui-fg);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.srv-card__host {
  font-size: var(--fs-xs);
  margin-bottom: var(--space-3);
}
.srv-card__host code {
  font-family: var(--font-mono);
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--ui-border);
}
.srv-card__foot {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-top: var(--space-2);
  padding-top: var(--space-2);
  border-top: 1px solid var(--ui-border);
}
.srv-card__foot b { color: var(--ui-fg); font-weight: var(--fw-semibold); }
.srv-card__sep { color: var(--ui-fg-4); margin: 0 var(--space-2); }
.srv-card__nometric {
  font-size: var(--fs-xs);
  color: var(--ui-fg-4);
  padding: var(--space-3) 0;
  text-align: center;
}

:deep(.mr) {
  display: flex; align-items: center; gap: var(--space-2);
  margin-bottom: var(--space-1);
}
:deep(.mr__label) {
  font-size: 11px;
  color: var(--ui-fg-3);
  width: 28px;
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}
:deep(.mr__track) {
  flex: 1;
  height: 4px;
  background: var(--ui-bg-2);
  border-radius: 2px;
  overflow: hidden;
}
:deep(.mr__fill) {
  height: 100%;
  border-radius: 2px;
  transition: width var(--dur-slow) var(--ease);
}
:deep(.mr__val) {
  font-size: 11px;
  width: 36px; text-align: right;
  font-weight: var(--fw-semibold);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.dash__chart { width: 100%; height: 280px; }

.dash__apps { display: flex; flex-direction: column; }
.app-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--ui-border);
  text-decoration: none;
  transition: background var(--dur-fast) var(--ease);
}
.app-row:last-child { border-bottom: 0; }
.app-row:hover { background: var(--ui-bg-2); }

.app-row__left { display: flex; align-items: center; gap: var(--space-3); min-width: 0; }
.app-row__info { display: flex; flex-direction: column; min-width: 0; }
.app-row__name { font-size: var(--fs-sm); font-weight: var(--fw-medium); color: var(--ui-fg); }
.app-row__desc { font-size: var(--fs-xs); color: var(--ui-fg-3); }

.app-row__right { display: flex; align-items: center; gap: var(--space-2); flex-shrink: 0; }
.app-row__meta {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
}
</style>
