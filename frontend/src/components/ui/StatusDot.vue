<template>
  <span
    class="ui-dot"
    :class="[`ui-dot--${resolved}`, { 'ui-dot--pulse': pulse, 'ui-dot--ring': ring }]"
    :style="{ '--dot-size': `${size}px` }"
    :title="title"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'

export type UiStatus =
  | 'online' | 'success' | 'running'
  | 'offline' | 'stopped'
  | 'error' | 'failed' | 'danger'
  | 'warning' | 'pending' | 'deploying'
  | 'unknown' | 'muted'

const props = withDefaults(defineProps<{
  status?: UiStatus | string
  size?: number
  pulse?: boolean
  ring?: boolean
  title?: string
}>(), {
  status: 'unknown',
  size: 8,
  pulse: false,
  ring: true,
})

const resolved = computed(() => {
  const s = (props.status || '').toLowerCase()
  if (['online', 'success', 'running', 'ok'].includes(s)) return 'success'
  if (['offline', 'stopped'].includes(s)) return 'muted'
  if (['error', 'failed', 'danger'].includes(s)) return 'danger'
  if (['warning', 'pending', 'deploying'].includes(s)) return 'warning'
  if (['muted'].includes(s)) return 'muted'
  return 'unknown'
})
</script>

<style scoped>
.ui-dot {
  display: inline-block;
  width: var(--dot-size);
  height: var(--dot-size);
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--ui-muted);
  vertical-align: middle;
  position: relative;
}
.ui-dot--success { background: var(--ui-success); }
.ui-dot--danger  { background: var(--ui-danger); }
.ui-dot--warning { background: var(--ui-warning); }
.ui-dot--muted   { background: var(--ui-muted); }
.ui-dot--unknown { background: var(--ui-fg-placeholder); }

.ui-dot--ring.ui-dot--success { box-shadow: 0 0 0 3px rgba(22, 163, 74, .18); }
.ui-dot--ring.ui-dot--danger  { box-shadow: 0 0 0 3px rgba(220, 38, 38, .18); }
.ui-dot--ring.ui-dot--warning { box-shadow: 0 0 0 3px rgba(217, 119, 6, .18); }

.ui-dot--pulse.ui-dot--success { animation: ui-status-pulse 2.4s var(--ui-ease-standard) infinite; }
.ui-dot--pulse.ui-dot--warning { animation: ui-status-pulse 1.6s var(--ui-ease-standard) infinite; }
.ui-dot--pulse.ui-dot--danger  { animation: ui-status-pulse 1.2s var(--ui-ease-standard) infinite; }
</style>
