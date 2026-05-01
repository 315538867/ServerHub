<template>
  <div class="page dash">
    <!-- 项目卡片（Primary） -->
    <UiSection>
      <template #title>
        <span class="dash__sec-title"><Package :size="16" /> 项目</span>
      </template>
      <template #extra>
        <UiBadge tone="neutral">{{ appStore.apps.length }} 个项目</UiBadge>
        <UiBadge tone="success">{{ appsOnline }} 运行中</UiBadge>
        <UiBadge v-if="appsOffline" tone="danger">{{ appsOffline }} 异常</UiBadge>
        <router-link to="/apps/create">
          <UiButton variant="primary" size="sm">
            <template #icon><Plus :size="14" /></template>
            新建项目
          </UiButton>
        </router-link>
      </template>
      <EmptyBlock v-if="!appStore.apps.length" title="暂无项目" description="点击「新建项目」开始部署第一个项目" />
      <div v-else class="dash__projects">
        <div
          v-for="app in appStore.apps"
          :key="app.id"
          class="proj-card"
        >
          <div class="proj-card__head">
            <div class="proj-card__title-row">
              <StatusDot :status="app.status" :size="9" :pulse="app.status === 'running'" />
              <router-link :to="`/apps/${app.id}/overview`" class="proj-card__name">{{ app.name }}</router-link>
              <UiBadge :tone="appStatusTone(app.status)" size="sm">{{ appStatusText(app.status) }}</UiBadge>
            </div>
            <div class="proj-card__desc">{{ app.description || '—' }}</div>
          </div>

          <!-- access_url -->
          <div v-if="app.access_url" class="proj-card__url">
            <Globe :size="12" />
            <code>{{ app.access_url }}</code>
          </div>

          <div class="proj-card__meta">
            <span class="proj-meta">
              <Server :size="11" />
              {{ serverMap[app.server_id]?.name || `#${app.server_id}` }}
            </span>
            <span class="proj-meta">
              <Route :size="11" />
              {{ app.ingress_count ?? 0 }} Ingress
            </span>
            <span class="proj-meta">
              <Package :size="11" />
              {{ app.service_count ?? 0 }} Service
            </span>
            <span v-if="app.container_name" class="proj-meta">
              <Container :size="11" />
              {{ app.container_name }}
            </span>
          </div>

          <div class="proj-card__actions">
            <router-link v-if="app.access_url" :to="`/apps/${app.id}/overview`" class="proj-act">
              <Globe :size="13" /> 概览
            </router-link>
            <router-link :to="`/apps/${app.id}/releases`" class="proj-act">
              <Rocket :size="13" /> Releases
            </router-link>
            <router-link v-if="app.expose_mode !== 'none'" :to="`/apps/${app.id}/network/ingresses`" class="proj-act">
              <Route :size="13" /> 流量
            </router-link>
            <router-link :to="`/apps/${app.id}/ops/terminal`" class="proj-act">
              <Terminal :size="13" /> 终端
            </router-link>
          </div>
        </div>
      </div>
    </UiSection>

    <!-- 服务器状态 -->
    <UiSection>
      <template #title>
        <span class="dash__sec-title"><Server :size="16" /> 服务器状态</span>
      </template>
      <template #extra>
        <UiBadge tone="neutral">{{ total }} 台 · 在线 {{ online }}{{ onlinePct > 0 ? ` (${onlinePct}%)` : '' }}</UiBadge>
        <UiButton v-if="showServerCards" size="sm" variant="secondary" @click="showServerCards = false">收起</UiButton>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="loadOverview">刷新</UiButton>
      </template>
      <EmptyBlock v-if="!loading && overview.length === 0" title="暂无服务器" description="先去「服务器管理」添加一台" />
      <div v-else-if="showServerCards" class="dash__grid">
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
      <!-- 紧凑模式：仅展示服务器简表 -->
      <div v-else class="dash__srv-compact">
        <div
          v-for="item in overview"
          :key="item.id"
          class="srv-compact-row"
          @click="showServerCards = true; selectServer(item)"
        >
          <StatusDot :status="item.status" :size="7" :pulse="item.status === 'online'" />
          <span class="srv-compact-name">{{ item.name }}</span>
          <code class="srv-compact-host">{{ item.host }}</code>
          <UiBadge :tone="statusTone(item.status)" size="sm">{{ statusText(item.status) }}</UiBadge>
          <span v-if="item.metric" class="srv-compact-metrics">
            CPU {{ round(item.metric.cpu) }}% · 内存 {{ round(item.metric.mem) }}%
          </span>
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

    <!-- ServerHub 自身资源（折叠） -->
    <details v-if="self" class="dash__self-details">
      <summary class="dash__self-summary">
        <Zap :size="13" /> ServerHub 自身资源
        <span class="dash__self-summary-vals">
          {{ cpuText }} CPU · {{ memText }} RAM · {{ uptimeText }}
        </span>
      </summary>
      <div class="dash__self">
        <UiStatCard title="CPU" :value="cpuText" :hint="`${self.num_cpu} 核`">
          <UiSparkline :points="self.history.cpu" :width="120" :height="28" color="brand" />
        </UiStatCard>
        <UiStatCard title="内存" :value="memText" :hint="`堆 ${memSysText}`">
          <UiSparkline :points="self.history.mem" :width="120" :height="28" color="success" />
        </UiStatCard>
        <UiStatCard title="Goroutine" :value="self.goroutines" :hint="`连接 ${self.connections}`" />
        <UiStatCard title="运行时长" :value="uptimeText" hint="自服务启动" />
      </div>
    </details>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, h } from 'vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import { Server, LineChart, Package, Plus, Globe, Route, Container, Terminal, Rocket, Zap } from 'lucide-vue-next'
import { getOverview, getServerMetrics, type ServerOverview } from '@/api/metrics'
import { getSelfMetrics, type SelfMetrics } from '@/api/system'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import type { Metric } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSparkline from '@/components/ui/UiSparkline.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'
import { useThemeStore } from '@/stores/theme'

const appStore = useAppStore()
const serverStore = useServerStore()
const theme = useThemeStore()

const serverMap = computed(() => {
  const m: Record<number, typeof serverStore.servers[number]> = {}
  for (const s of serverStore.servers) m[s.id] = s
  return m
})
const overview = ref<ServerOverview[]>([])
const showServerCards = ref(false)
const loading = ref(false)
const refreshInterval = 30_000

const total   = computed(() => overview.value.length)
const online  = computed(() => overview.value.filter(s => s.status === 'online').length)
const offline = computed(() => overview.value.filter(s => s.status === 'offline').length)
const onlinePct = computed(() => total.value ? Math.round(online.value / total.value * 100) : 0)
// R3 起 app.status 枚举: running | syncing | error | unknown
const appsOnline  = computed(() => appStore.apps.filter(a => a.status === 'running').length)
const appsOffline = computed(() => appStore.apps.filter(a => a.status === 'error').length)

function appStatusTone(s: string): any {
  return ({ running: 'success', syncing: 'warning', error: 'danger' } as Record<string,string>)[s] ?? 'neutral'
}
function appStatusText(s: string) {
  return ({ running: '运行中', syncing: '同步中', error: '错误', unknown: '未知' } as Record<string,string>)[s] ?? s
}
// R3 起 server.status 枚举: online | lagging | offline | unknown
function statusTone(s: string): any {
  return ({ online: 'success', lagging: 'warning', offline: 'danger' } as Record<string,string>)[s] ?? 'neutral'
}
function statusText(s: string) {
  return ({ online: '在线', lagging: '心跳延迟', offline: '离线', unknown: '未知' } as Record<string,string>)[s] ?? s
}
function round(n: number) { return Math.round(n) }
function formatUptime(sec: number) {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}天${h}时`
  if (h > 0) return `${h}时${m}分`
  return `${m}分`
}
function formatBytes(b: number) {
  if (b >= 1024 ** 3) return `${(b / 1024 ** 3).toFixed(1)} GB`
  if (b >= 1024 ** 2) return `${(b / 1024 ** 2).toFixed(0)} MB`
  if (b >= 1024) return `${(b / 1024).toFixed(0)} KB`
  return `${b} B`
}

const self = ref<SelfMetrics | null>(null)
let selfTimer: ReturnType<typeof setInterval> | null = null
const cpuText  = computed(() => self.value ? `${self.value.cpu_percent.toFixed(1)}%` : '—')
const memText  = computed(() => self.value ? formatBytes(self.value.mem_rss) : '—')
const memSysText = computed(() => self.value ? formatBytes(self.value.mem_sys) : '—')
const uptimeText = computed(() => self.value ? formatUptime(self.value.uptime) : '—')
async function loadSelf() {
  try { self.value = await getSelfMetrics() } catch { /* ignore */ }
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
  await Promise.all([loadOverview(), appStore.ensure(), loadSelf()])
  timer = setInterval(loadOverview, refreshInterval)
  selfTimer = setInterval(loadSelf, 15000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (selfTimer) clearInterval(selfTimer)
  chart?.dispose()
})
</script>

<style scoped>
.dash {
  display: flex; flex-direction: column;
  gap: var(--space-6);
  padding: var(--space-6);
}

.dash__self-details {
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  background: var(--ui-bg-1);
  overflow: hidden;
}
.dash__self-summary {
  display: flex; align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;
  font-size: var(--fs-sm);
  color: var(--ui-fg-2);
  font-weight: var(--fw-medium);
  user-select: none;
  transition: background var(--dur-fast) var(--ease);
}
.dash__self-summary:hover { background: var(--ui-bg-2); }
.dash__self-summary-vals {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  font-weight: 400;
}
.dash__self details[open] > .dash__self-summary { border-bottom: 1px solid var(--ui-border); }
.dash__self details[open] .dash__self { padding: var(--space-3) var(--space-4); }

.dash__self {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
}
@media (max-width: 1024px) { .dash__self { grid-template-columns: repeat(2, 1fr); } }

.dash__stats { display: none; }

.dash__srv-compact {
  display: flex; flex-direction: column;
  gap: 2px;
  padding: var(--space-2);
}
.srv-compact-row {
  display: flex; align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease);
  font-size: var(--fs-sm);
}
.srv-compact-row:hover { background: var(--ui-bg-2); }
.srv-compact-name {
  font-weight: var(--fw-medium);
  color: var(--ui-fg);
  min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.srv-compact-host {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
}
.srv-compact-metrics {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}

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

.dash__projects {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: var(--space-3);
  padding: var(--space-3);
}
@media (max-width: 768px) { .dash__projects { grid-template-columns: 1fr; } }

.proj-card {
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  background: var(--ui-bg-1);
  padding: var(--space-4);
  display: flex; flex-direction: column;
  gap: var(--space-3);
  transition: border-color var(--dur-fast) var(--ease), box-shadow var(--dur-fast) var(--ease);
}
.proj-card:hover {
  border-color: var(--ui-border-strong);
  box-shadow: var(--shadow-sm);
}

.proj-card__head { display: flex; flex-direction: column; gap: var(--space-1); }
.proj-card__title-row {
  display: flex; align-items: center; gap: var(--space-2);
}
.proj-card__name {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  text-decoration: none;
  transition: color var(--dur-fast) var(--ease);
}
.proj-card__name:hover { color: var(--ui-brand); }
.proj-card__desc {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  padding-left: calc(var(--space-2) + 9px);
}

.proj-card__url {
  display: flex; align-items: center; gap: var(--space-2);
  font-size: var(--fs-xs);
  color: var(--ui-brand-fg);
  padding: var(--space-1) var(--space-2);
  background: var(--ui-brand-soft);
  border-radius: var(--radius-sm);
  border: 1px solid color-mix(in srgb, var(--ui-brand) 20%, transparent);
}
.proj-card__url code {
  font-family: var(--font-mono);
  color: var(--ui-brand-fg);
  font-weight: var(--fw-medium);
}

.proj-card__meta {
  display: flex; flex-wrap: wrap; gap: var(--space-2);
}
.proj-meta {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  padding: 2px 8px;
}

.proj-card__actions {
  display: flex; gap: var(--space-1);
  padding-top: var(--space-2);
  border-top: 1px solid var(--ui-border);
}
.proj-act {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
  text-decoration: none;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  transition: all var(--dur-fast) var(--ease);
}
.proj-act:hover {
  background: var(--ui-bg-2);
  border-color: var(--ui-border);
  color: var(--ui-fg);
}
</style>
