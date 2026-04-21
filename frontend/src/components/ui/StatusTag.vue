<template>
  <UiBadge :tone="tone" :size="size === 'small' ? 'sm' : 'md'" dot>
    <slot>{{ label }}</slot>
  </UiBadge>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiBadge from './UiBadge.vue'

interface Props {
  status?: string
  label?: string
  size?: 'small' | 'medium' | 'large' | 'sm' | 'md'
  /** @deprecated back-compat */
  theme?: string
}
const props = withDefaults(defineProps<Props>(), { size: 'md' })

const tone = computed<'neutral' | 'success' | 'warning' | 'danger' | 'info'>(() => {
  const s = (props.status || props.theme || '').toLowerCase()
  if (['online', 'running', 'active', 'ok', 'success', 'healthy'].includes(s)) return 'success'
  if (['error', 'failed', 'fail', 'danger'].includes(s)) return 'danger'
  if (['warning', 'warn', 'degraded'].includes(s)) return 'warning'
  if (['pending', 'deploying', 'loading', 'info', 'primary'].includes(s)) return 'info'
  return 'neutral'
})
</script>
