<template>
  <UiCard padding="md" class="metrics-card">
    <div class="mc-header">
      <span class="mc-title">实时指标</span>
      <div class="mc-actions">
        <span v-if="lastUpdate" class="mc-time">{{ lastUpdate }}</span>
        <UiButton variant="ghost" size="sm" :loading="loading" @click="tick">
          <template #icon><RefreshCw :size="13" /></template>
          刷新
        </UiButton>
        <span class="mc-auto">
          <NSwitch v-model:value="autoRefresh" size="small" />
          <span class="mc-auto-lbl">自动</span>
        </span>
      </div>
    </div>

    <div v-if="!metrics || !metrics.available" class="mc-empty">
      <Info :size="20" class="mc-empty-icon" />
      <div class="mc-empty-text">{{ metrics?.reason || '加载中…' }}</div>
    </div>

    <div v-else class="mc-grid">
      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">CPU</span>
          <span class="mc-val">{{ metrics.cpu_percent.toFixed(1) }}<small>%</small></span>
        </div>
        <svg class="mc-spark" viewBox="0 0 120 32" preserveAspectRatio="none">
          <polyline
            v-if="cpuHistory.length > 1"
            :points="sparkArea(cpuHistory, 100)"
            fill="color-mix(in srgb, var(--ui-brand) 14%, transparent)"
            stroke="none"
          />
          <polyline
            v-if="cpuHistory.length > 1"
            :points="sparkPoints(cpuHistory, 100)"
            fill="none"
            stroke="var(--ui-brand)"
            stroke-width="1.5"
          />
        </svg>
      </div>

      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">内存</span>
          <span class="mc-val">{{ metrics.mem_percent.toFixed(1) }}<small>%</small></span>
        </div>
        <div class="mc-sub">{{ metrics.mem_usage }}</div>
        <svg class="mc-spark" viewBox="0 0 120 32" preserveAspectRatio="none">
          <polyline
            v-if="memHistory.length > 1"
            :points="sparkArea(memHistory, 100)"
            fill="color-mix(in srgb, var(--ui-success) 14%, transparent)"
            stroke="none"
          />
          <polyline
            v-if="memHistory.length > 1"
            :points="sparkPoints(memHistory, 100)"
            fill="none"
            stroke="var(--ui-success)"
            stroke-width="1.5"
          />
        </svg>
      </div>

      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">网络 I/O</span>
          <span class="mc-val-text">{{ metrics.net_io }}</span>
        </div>
      </div>

      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">磁盘 I/O</span>
          <span class="mc-val-text">{{ metrics.block_io }}</span>
        </div>
      </div>

      <div class="mc-item mc-item--compact">
        <span class="mc-cap">进程数</span>
        <span class="mc-val">{{ metrics.pids }}</span>
      </div>

      <div class="mc-item mc-item--compact">
        <span class="mc-cap">容器 ID</span>
        <code class="mc-code">{{ metrics.container_id.slice(0, 12) }}</code>
      </div>
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { NSwitch } from 'naive-ui'
import { RefreshCw, Info } from 'lucide-vue-next'
import { getAppMetrics, type AppMetrics } from '@/api/application'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{ appId: number }>()

const metrics = ref<AppMetrics | null>(null)
const loading = ref(false)
const autoRefresh = ref(true)
const lastUpdate = ref('')

const cpuHistory = ref<number[]>([])
const memHistory = ref<number[]>([])
const MAX_POINTS = 60

let timer: ReturnType<typeof setInterval> | null = null

async function tick() {
  if (!props.appId || loading.value) return
  loading.value = true
  try {
    const m = await getAppMetrics(props.appId)
    metrics.value = m
    if (m.available) {
      cpuHistory.value.push(m.cpu_percent)
      if (cpuHistory.value.length > MAX_POINTS) cpuHistory.value.shift()
      memHistory.value.push(m.mem_percent)
      if (memHistory.value.length > MAX_POINTS) memHistory.value.shift()
      lastUpdate.value = new Date().toLocaleTimeString('zh-CN')
    }
  } catch {
  } finally {
    loading.value = false
  }
}

function startTimer() {
  stopTimer()
  if (autoRefresh.value) timer = setInterval(tick, 5000)
}
function stopTimer() {
  if (timer) { clearInterval(timer); timer = null }
}

watch(autoRefresh, (v) => { v ? startTimer() : stopTimer() })
watch(() => props.appId, () => { cpuHistory.value = []; memHistory.value = []; tick() })

onMounted(() => { tick(); startTimer() })
onBeforeUnmount(stopTimer)

function sparkPoints(data: number[], maxVal: number): string {
  const n = data.length
  if (n < 2) return ''
  const stepX = 120 / (MAX_POINTS - 1)
  return data
    .map((v, i) => {
      const x = (MAX_POINTS - n + i) * stepX
      const y = 32 - Math.max(0, Math.min(v, maxVal)) / maxVal * 30 - 1
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

function sparkArea(data: number[], maxVal: number): string {
  const n = data.length
  if (n < 2) return ''
  const stepX = 120 / (MAX_POINTS - 1)
  const first = (MAX_POINTS - n) * stepX
  const last = (MAX_POINTS - 1) * stepX
  const line = data
    .map((v, i) => {
      const x = (MAX_POINTS - n + i) * stepX
      const y = 32 - Math.max(0, Math.min(v, maxVal)) / maxVal * 30 - 1
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
  return `${first.toFixed(1)},32 ${line} ${last.toFixed(1)},32`
}
</script>

<style scoped>
.metrics-card { display: block; }
.mc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}
.mc-title {
  font-size: var(--fs-sm);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}
.mc-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.mc-time {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  font-variant-numeric: tabular-nums;
}
.mc-auto {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
}
.mc-auto-lbl { font-size: var(--fs-xs); color: var(--ui-fg-3); }

.mc-empty {
  padding: var(--space-5) 0;
  text-align: center;
  color: var(--ui-fg-3);
  font-size: var(--fs-sm);
}
.mc-empty-icon {
  opacity: 0.5;
  margin-bottom: var(--space-2);
  display: inline-block;
}

.mc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--space-3);
}
.mc-item {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--ui-bg-1);
  border-radius: var(--radius-sm);
}
.mc-item--compact {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}
.mc-item-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
}
.mc-cap {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  text-transform: uppercase;
  letter-spacing: 0.4px;
}
.mc-val {
  font-size: var(--fs-2xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  font-variant-numeric: tabular-nums;
}
.mc-val small { font-size: var(--fs-xs); opacity: 0.6; margin-left: 2px; }
.mc-val-text {
  font-size: var(--fs-sm);
  color: var(--ui-fg);
  font-family: var(--font-mono);
}
.mc-sub {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  font-family: var(--font-mono);
}
.mc-spark {
  width: 100%;
  height: 32px;
  display: block;
}
.mc-code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  color: var(--ui-fg);
}
</style>
