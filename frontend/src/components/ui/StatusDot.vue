<template>
  <span
    class="ui-dot"
    :class="[`ui-dot--${resolved}`, { 'ui-dot--pulse': pulse }]"
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
  title?: string
}>(), {
  status: 'unknown',
  size: 8,
  pulse: false,
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
}
.ui-dot--success { background: var(--ui-success); box-shadow: 0 0 0 2px rgba(23,166,115,.18); }
.ui-dot--danger  { background: var(--ui-danger);  box-shadow: 0 0 0 2px rgba(214,69,69,.18); }
.ui-dot--warning { background: var(--ui-warning); box-shadow: 0 0 0 2px rgba(233,138,42,.18); }
.ui-dot--muted   { background: var(--ui-muted); }
.ui-dot--unknown { background: var(--ui-fg-placeholder); }
.ui-dot--pulse { animation: ui-dot-pulse 1.6s infinite var(--ui-ease-standard); }

@keyframes ui-dot-pulse {
  0%,100% { opacity: 1; }
  50%     { opacity: 0.45; }
}
</style>
