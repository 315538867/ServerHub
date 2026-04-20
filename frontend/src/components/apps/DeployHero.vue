<template>
  <div class="deploy-hero" :class="heroClass">
    <!-- 左：状态 + 版本 + 最近运行 -->
    <div class="hero-info">
      <div class="hero-status-row">
        <span class="hero-dot" :class="syncDotClass" />
        <div class="hero-status-text">
          <span class="hero-status-label">{{ syncLabel }}</span>
          <span class="hero-status-sub">{{ syncSubText }}</span>
        </div>
      </div>

      <div class="hero-versions" v-if="showVersions">
        <div class="hero-ver-item">
          <span class="ver-cap">期望版本</span>
          <code class="ver-val ver-desired">{{ deploy.desired_version || '—' }}</code>
        </div>
        <span class="ver-arrow" :class="{ 'ver-arrow--drift': isDrift }">→</span>
        <div class="hero-ver-item">
          <span class="ver-cap">当前版本</span>
          <code class="ver-val ver-actual">{{ deploy.actual_version || '—' }}</code>
        </div>
        <div v-if="deploy.previous_version" class="hero-ver-item ver-prev">
          <span class="ver-cap">上次</span>
          <code class="ver-val">{{ deploy.previous_version }}</code>
        </div>
      </div>

      <div class="hero-meta">
        <span v-if="deploy.last_run_at" class="meta-item">
          <span class="meta-cap">最近运行</span>
          <span class="meta-val">{{ formatTimeAgo(deploy.last_run_at) }}</span>
        </span>
        <span v-if="deploy.last_status" class="meta-item">
          <t-tag size="small" variant="light" :theme="lastStatusTheme">{{ lastStatusLabel }}</t-tag>
        </span>
        <span v-if="deploy.auto_sync" class="meta-item">
          <t-tag size="small" variant="outline" theme="primary">自动同步 · {{ deploy.sync_interval || 30 }}s</t-tag>
        </span>
      </div>
    </div>

    <!-- 右：主操作 -->
    <div class="hero-actions">
      <template v-if="!running">
        <t-button theme="primary" size="medium" @click="$emit('run')">
          <template #icon><span class="btn-icon">▶</span></template>
          立即部署
        </t-button>
        <t-button
          size="medium"
          variant="outline"
          :disabled="!deploy.previous_version"
          @click="$emit('rollback')"
        >
          <template #icon><span class="btn-icon">↺</span></template>
          回滚
        </t-button>
      </template>
      <template v-else>
        <div class="hero-running">
          <t-loading size="small" />
          <span>部署进行中…</span>
        </div>
        <t-button theme="danger" variant="outline" size="medium" @click="$emit('stop')">中止</t-button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Deploy } from '@/types/api'

const props = defineProps<{
  deploy: Deploy
  running: boolean
}>()

defineEmits<{
  run: []
  rollback: []
  stop: []
}>()

const showVersions = computed(() =>
  props.deploy.type !== 'native' && (props.deploy.desired_version || props.deploy.actual_version)
)
const isDrift = computed(() => props.deploy.sync_status === 'drifted' || props.deploy.sync_status === 'error')

const syncLabel = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return '已同步'
    case 'drifted': return '版本漂移'
    case 'syncing': return '同步中'
    case 'error': return '同步失败'
    default: return '未同步'
  }
})

const syncSubText = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return '当前版本与期望版本一致'
    case 'drifted': return '检测到版本不匹配，建议执行部署'
    case 'syncing': return '正在拉取镜像并重启服务'
    case 'error': return '上次部署失败，请检查日志'
    default: return '尚未运行过部署'
  }
})

const syncDotClass = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return 'dot--ok'
    case 'drifted': return 'dot--warn'
    case 'syncing': return 'dot--info'
    case 'error': return 'dot--err'
    default: return 'dot--idle'
  }
})

const heroClass = computed(() => ({
  'deploy-hero--ok': props.deploy.sync_status === 'synced',
  'deploy-hero--drift': isDrift.value,
  'deploy-hero--running': props.running,
}))

const lastStatusLabel = computed(() => {
  switch (props.deploy.last_status) {
    case 'success': return '上次成功'
    case 'failed': return '上次失败'
    case 'running': return '执行中'
    default: return props.deploy.last_status || '—'
  }
})
const lastStatusTheme = computed(() => {
  switch (props.deploy.last_status) {
    case 'success': return 'success' as const
    case 'failed': return 'danger' as const
    case 'running': return 'warning' as const
    default: return 'default' as const
  }
})

function formatTimeAgo(iso: string): string {
  const t = new Date(iso).getTime()
  if (!t) return '—'
  const diff = Date.now() - t
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr} 小时前`
  const day = Math.floor(hr / 24)
  if (day < 30) return `${day} 天前`
  return new Date(iso).toLocaleDateString('zh-CN')
}
</script>

<style scoped>
.deploy-hero {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sh-space-lg);
  padding: var(--sh-space-md) var(--sh-space-lg);
  margin-bottom: var(--sh-space-md);
  background: linear-gradient(135deg, var(--sh-card-bg) 0%, color-mix(in srgb, var(--sh-card-bg) 92%, var(--sh-blue) 8%) 100%);
  border: 1px solid var(--sh-border);
  border-radius: 12px;
  box-shadow: 0 4px 16px -8px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(8px);
}
.deploy-hero--ok {
  border-left: 4px solid var(--sh-success, #67c23a);
}
.deploy-hero--drift {
  border-left: 4px solid var(--sh-warning, #e6a23c);
  background: linear-gradient(135deg, var(--sh-card-bg) 0%, color-mix(in srgb, var(--sh-card-bg) 88%, #e6a23c 12%) 100%);
}
.deploy-hero--running {
  border-left: 4px solid var(--sh-blue);
  animation: heroPulse 2s ease-in-out infinite;
}
@keyframes heroPulse {
  0%, 100% { box-shadow: 0 4px 16px -8px rgba(0, 102, 204, 0.2); }
  50%      { box-shadow: 0 4px 24px -4px rgba(0, 102, 204, 0.4); }
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: var(--sh-space-sm);
  flex: 1;
  min-width: 0;
}

.hero-status-row {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
}
.hero-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot--ok   { background: var(--sh-success, #67c23a); box-shadow: 0 0 0 4px color-mix(in srgb, var(--sh-success, #67c23a) 22%, transparent); }
.dot--warn { background: var(--sh-warning, #e6a23c); box-shadow: 0 0 0 4px color-mix(in srgb, var(--sh-warning, #e6a23c) 22%, transparent); }
.dot--err  { background: var(--sh-danger, #f56c6c); box-shadow: 0 0 0 4px color-mix(in srgb, var(--sh-danger, #f56c6c) 22%, transparent); }
.dot--info { background: var(--sh-blue); box-shadow: 0 0 0 4px color-mix(in srgb, var(--sh-blue) 22%, transparent); animation: dotBlink 1.2s ease-in-out infinite; }
.dot--idle { background: var(--sh-text-secondary); opacity: 0.5; }
@keyframes dotBlink { 50% { opacity: 0.4; } }

.hero-status-text { display: flex; flex-direction: column; gap: var(--sh-space-xs); }
.hero-status-label { font-size: 15px; font-weight: 600; color: var(--sh-text-primary); }
.hero-status-sub { font-size: 12px; color: var(--sh-text-secondary); }

.hero-versions {
  display: flex;
  align-items: flex-end;
  gap: var(--sh-space-md);
  margin-top: var(--sh-space-xs);
  flex-wrap: wrap;
}
.hero-ver-item { display: flex; flex-direction: column; gap: var(--sh-space-xs); }
.ver-cap { font-size: 11px; color: var(--sh-text-secondary); text-transform: uppercase; letter-spacing: 0.4px; }
.ver-val {
  font-family: var(--sh-font-mono, ui-monospace, SFMono-Regular, monospace);
  font-size: 13px;
  padding: var(--sh-space-xs) var(--sh-space-sm);
  background: color-mix(in srgb, var(--sh-text-primary) 6%, transparent);
  border-radius: 4px;
  color: var(--sh-text-primary);
}
.ver-desired { color: var(--sh-blue); font-weight: 600; }
.ver-actual  { font-weight: 600; }
.ver-prev .ver-val { opacity: 0.7; font-size: 12px; }
.ver-arrow {
  font-size: 14px;
  color: var(--sh-text-secondary);
  margin-bottom: var(--sh-space-xs);
}
.ver-arrow--drift { color: var(--sh-warning, #e6a23c); font-weight: 700; }

.hero-meta {
  display: flex;
  align-items: center;
  gap: var(--sh-space-md);
  margin-top: var(--sh-space-sm);
  flex-wrap: wrap;
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: var(--sh-space-xs);
  font-size: 12px;
  color: var(--sh-text-secondary);
}
.meta-cap { color: var(--sh-text-secondary); }
.meta-val { color: var(--sh-text-primary); font-weight: 500; }

.hero-actions {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  flex-shrink: 0;
}
.btn-icon {
  display: inline-block;
  font-size: 12px;
  margin-right: var(--sh-space-xs);
}
.hero-running {
  display: inline-flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: 0 var(--sh-space-md);
  font-size: 13px;
  color: var(--sh-blue);
}

@media (max-width: 720px) {
  .deploy-hero {
    flex-direction: column;
    align-items: stretch;
    gap: var(--sh-space-md);
  }
  .hero-actions { justify-content: stretch; }
  .hero-actions :deep(.t-button) { flex: 1; }
}
</style>
