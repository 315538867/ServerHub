<template>
  <div class="deploy-hero" :class="heroClass">
    <div class="hero-info">
      <div class="hero-status-row">
        <span class="hero-dot" :class="syncDotClass" />
        <div class="hero-status-text">
          <span class="hero-status-label">{{ syncLabel }}</span>
          <span class="hero-status-sub">{{ syncSubText }}</span>
        </div>
      </div>

      <div class="hero-meta">
        <UiBadge v-if="deploy.auto_sync" tone="brand">自动同步 · {{ deploy.sync_interval || 30 }}s</UiBadge>
      </div>
    </div>

    <div class="hero-actions">
      <template v-if="!running">
        <UiButton variant="primary" size="md" @click="$emit('run')">
          <template #icon><Play :size="14" /></template>
          立即部署
        </UiButton>
        <UiButton
          variant="secondary"
          size="md"
          :disabled="!canRollback"
          @click="$emit('rollback')"
        >
          <template #icon><RotateCcw :size="14" /></template>
          回滚
        </UiButton>
      </template>
      <template v-else>
        <div class="hero-running">
          <Loader2 :size="14" class="spin" />
          <span>部署进行中…</span>
        </div>
        <UiButton variant="danger" size="md" @click="$emit('stop')">中止</UiButton>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Play, RotateCcw, Loader2 } from 'lucide-vue-next'
import type { Deploy } from '@/types/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const props = defineProps<{
  deploy: Deploy
  running: boolean
  canRollback?: boolean
}>()

defineEmits<{
  run: []
  rollback: []
  stop: []
}>()

// 'drifted' 分支随 P-D DesiredVersion 漂移检测一起退役 —— 后端 reconciler 早已
// 不写该值,这里一并删掉死分支。错误态仍保留 hero--drift 红色样式以示区别。
const isError = computed(() => props.deploy.sync_status === 'error')

const syncLabel = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return '已同步'
    case 'syncing': return '同步中'
    case 'error': return '同步失败'
    default: return '未同步'
  }
})

const syncSubText = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return '当前版本与期望版本一致'
    case 'syncing': return '正在拉取镜像并重启服务'
    case 'error': return '上次部署失败，请检查日志'
    default: return '尚未运行过部署'
  }
})

const syncDotClass = computed(() => {
  switch (props.deploy.sync_status) {
    case 'synced': return 'dot--ok'
    case 'syncing': return 'dot--info'
    case 'error': return 'dot--err'
    default: return 'dot--idle'
  }
})

const heroClass = computed(() => ({
  'deploy-hero--ok': props.deploy.sync_status === 'synced',
  'deploy-hero--drift': isError.value,
  'deploy-hero--running': props.running,
}))
</script>

<style scoped>
.deploy-hero {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-5);
  padding: var(--space-4) var(--space-5);
  margin-bottom: var(--space-4);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  border-left: 3px solid var(--ui-border);
  border-radius: var(--radius-md);
}
.deploy-hero--ok { border-left-color: var(--ui-success); }
.deploy-hero--drift { border-left-color: var(--ui-warning); }
.deploy-hero--running {
  border-left-color: var(--ui-brand);
  animation: heroPulse 2s ease-in-out infinite;
}
@keyframes heroPulse {
  0%, 100% { box-shadow: 0 4px 16px -8px rgba(62,207,142, 0.2); }
  50%      { box-shadow: 0 4px 24px -4px rgba(62,207,142, 0.4); }
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  flex: 1;
  min-width: 0;
}

.hero-status-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.hero-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot--ok   { background: var(--ui-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--ui-success) 22%, transparent); }
.dot--warn { background: var(--ui-warning); box-shadow: 0 0 0 4px color-mix(in srgb, var(--ui-warning) 22%, transparent); }
.dot--err  { background: var(--ui-danger); box-shadow: 0 0 0 4px color-mix(in srgb, var(--ui-danger) 22%, transparent); }
.dot--info { background: var(--ui-brand); box-shadow: 0 0 0 4px color-mix(in srgb, var(--ui-brand) 22%, transparent); animation: dotBlink 1.2s ease-in-out infinite; }
.dot--idle { background: var(--ui-fg-4); opacity: 0.5; }
@keyframes dotBlink { 50% { opacity: 0.4; } }

.hero-status-text {
  display: flex;
  flex-direction: column;
}
.hero-status-label {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}
.hero-status-sub {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}

.hero-meta {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-top: var(--space-2);
  flex-wrap: wrap;
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.meta-val { color: var(--ui-fg); font-weight: var(--fw-medium); }

.hero-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-shrink: 0;
}
.hero-running {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-3);
  font-size: var(--fs-sm);
  color: var(--ui-brand-fg);
}
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 720px) {
  .deploy-hero {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-3);
  }
}
</style>
