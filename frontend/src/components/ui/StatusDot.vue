<template>
  <span
    class="ui-dot"
    :class="[`ui-dot--${resolved}`, { 'ui-dot--pulse': pulse, 'ui-dot--ring': ring }]"
    :style="{ '--dot-size': `${size}px` }"
    :title="label"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'

export type UiStatus = 'online' | 'offline' | 'error' | 'warning' | 'unknown' | 'pending' | 'running' | 'stopped'

interface Props {
  status?: UiStatus | string
  size?: number
  pulse?: boolean
  ring?: boolean
  label?: string
}
const props = withDefaults(defineProps<Props>(), {
  status: 'unknown',
  size: 8,
  pulse: false,
  ring: true,
})

const resolved = computed(() => {
  const s = (props.status || '').toLowerCase()
  if (['online', 'running', 'active', 'ok', 'success', 'healthy'].includes(s)) return 'online'
  if (['offline', 'stopped', 'inactive'].includes(s)) return 'offline'
  if (['error', 'failed', 'fail', 'danger'].includes(s)) return 'error'
  if (['warning', 'warn', 'degraded', 'lagging'].includes(s)) return 'warning'
  if (['pending', 'deploying', 'loading', 'starting', 'syncing'].includes(s)) return 'pending'
  return 'unknown'
})
</script>

<style scoped>
.ui-dot {
  display: inline-block;
  width: var(--dot-size, 8px);
  height: var(--dot-size, 8px);
  border-radius: 50%;
  background: var(--ui-fg-4);
  flex-shrink: 0;
  position: relative;
}
.ui-dot--online  { background: var(--ui-success); }
.ui-dot--offline { background: var(--ui-fg-4); }
.ui-dot--error   { background: var(--ui-danger); }
.ui-dot--warning { background: var(--ui-warning); }
.ui-dot--pending { background: var(--ui-info); }
.ui-dot--unknown { background: var(--ui-fg-4); }

.ui-dot--ring::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.18;
  pointer-events: none;
}
.ui-dot--online.ui-dot--ring::after { color: var(--ui-success); }
.ui-dot--error.ui-dot--ring::after  { color: var(--ui-danger); }
.ui-dot--pending.ui-dot--ring::after{ color: var(--ui-info); }

.ui-dot--pulse { animation: ui-dot-pulse 2.2s ease-in-out infinite; }
@keyframes ui-dot-pulse {
  0%, 100% { box-shadow: 0 0 0 0 currentColor; }
  50%      { box-shadow: 0 0 0 4px transparent; }
}
</style>
