<template>
  <div class="metrics-card">
    <div class="mc-header">
      <span class="mc-title">实时指标</span>
      <div class="mc-actions">
        <span v-if="lastUpdate" class="mc-time">{{ lastUpdate }}</span>
        <t-button size="small" variant="text" :loading="loading" @click="tick">刷新</t-button>
        <t-switch v-model="autoRefresh" size="small">
          <template #label>自动</template>
        </t-switch>
      </div>
    </div>

    <div v-if="!metrics || !metrics.available" class="mc-empty">
      <t-icon-info-circle class="mc-empty-icon" />
      <div class="mc-empty-text">{{ metrics?.reason || '加载中…' }}</div>
    </div>

    <div v-else class="mc-grid">
      <!-- CPU -->
      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">CPU</span>
          <span class="mc-val">{{ metrics.cpu_percent.toFixed(1) }}<small>%</small></span>
        </div>
        <svg class="mc-spark" viewBox="0 0 120 32" preserveAspectRatio="none">
          <polyline
            v-if="cpuHistory.length > 1"
            :points="sparkPoints(cpuHistory, 100)"
            fill="none"
            stroke="var(--sh-blue)"
            stroke-width="1.5"
          />
          <polyline
            v-if="cpuHistory.length > 1"
            :points="sparkArea(cpuHistory, 100)"
            fill="color-mix(in srgb, var(--sh-blue) 14%, transparent)"
            stroke="none"
          />
        </svg>
      </div>

      <!-- Memory -->
      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">内存</span>
          <span class="mc-val">{{ metrics.mem_percent.toFixed(1) }}<small>%</small></span>
        </div>
        <div class="mc-sub">{{ metrics.mem_usage }}</div>
        <svg class="mc-spark" viewBox="0 0 120 32" preserveAspectRatio="none">
          <polyline
            v-if="memHistory.length > 1"
            :points="sparkPoints(memHistory, 100)"
            fill="none"
            stroke="#67c23a"
            stroke-width="1.5"
          />
          <polyline
            v-if="memHistory.length > 1"
            :points="sparkArea(memHistory, 100)"
            fill="color-mix(in srgb, #67c23a 14%, transparent)"
            stroke="none"
          />
        </svg>
      </div>

      <!-- Network -->
      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">网络 I/O</span>
          <span class="mc-val-text">{{ metrics.net_io }}</span>
        </div>
      </div>

      <!-- Block -->
      <div class="mc-item">
        <div class="mc-item-head">
          <span class="mc-cap">磁盘 I/O</span>
          <span class="mc-val-text">{{ metrics.block_io }}</span>
        </div>
      </div>

      <!-- PIDs -->
      <div class="mc-item mc-item--compact">
        <span class="mc-cap">进程数</span>
        <span class="mc-val">{{ metrics.pids }}</span>
      </div>

      <!-- Container ID -->
      <div class="mc-item mc-item--compact">
        <span class="mc-cap">容器 ID</span>
        <code class="mc-code">{{ metrics.container_id.slice(0, 12) }}</code>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { getAppMetrics, type AppMetrics } from '@/api/application'

const props = defineProps<{ appId: number }>()

const metrics = ref<AppMetrics | null>(null)
const loading = ref(false)
const autoRefresh = ref(true)
const lastUpdate = ref('')

// 60 点环形缓冲
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
    // 静默：容器暂时不可达是正常状态
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

// 绘制 sparkline：将数组归一化到 0-32 Y 范围、X 平均铺满 0-120
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
.metrics-card {
  background: var(--sh-card-bg);
  border: 1px solid var(--sh-border);
  border-radius: 10px;
  padding: var(--sh-space-md) var(--sh-space-lg) var(--sh-space-lg);
}
.mc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sh-space-md);
}
.mc-title { font-size: 14px; font-weight: 600; color: var(--sh-text-primary); }
.mc-actions { display: flex; align-items: center; gap: var(--sh-space-sm); }
.mc-time { font-size: 11px; color: var(--sh-text-secondary); font-variant-numeric: tabular-nums; }

.mc-empty {
  padding: var(--sh-space-lg) 0;
  text-align: center;
  color: var(--sh-text-secondary);
  font-size: 13px;
}
.mc-empty-icon { font-size: 20px; opacity: 0.5; margin-bottom: var(--sh-space-sm); display: inline-block; }

.mc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--sh-space-md);
}
.mc-item {
  display: flex;
  flex-direction: column;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-sm) var(--sh-space-md);
  background: color-mix(in srgb, var(--sh-text-primary) 3%, transparent);
  border-radius: 8px;
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
  gap: var(--sh-space-sm);
}
.mc-cap {
  font-size: 11px;
  color: var(--sh-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.4px;
}
.mc-val {
  font-size: 20px;
  font-weight: 600;
  color: var(--sh-text-primary);
  font-variant-numeric: tabular-nums;
}
.mc-val small { font-size: 12px; opacity: 0.6; margin-left: var(--sh-space-xs); }
.mc-val-text {
  font-size: 13px;
  color: var(--sh-text-primary);
  font-family: var(--sh-font-mono, ui-monospace, SFMono-Regular, monospace);
}
.mc-sub {
  font-size: 11px;
  color: var(--sh-text-secondary);
  font-family: var(--sh-font-mono, ui-monospace, SFMono-Regular, monospace);
}
.mc-spark {
  width: 100%;
  height: 32px;
  display: block;
}
.mc-code {
  font-family: var(--sh-font-mono, ui-monospace, SFMono-Regular, monospace);
  font-size: 12px;
  background: var(--sh-code-bg, rgba(0,0,0,.04));
  padding: 1px 6px;
  border-radius: 3px;
  color: var(--sh-text-primary);
}
</style>
