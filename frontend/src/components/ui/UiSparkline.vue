<template>
  <svg
    class="ui-sparkline"
    :viewBox="`0 0 ${width} ${height}`"
    :width="width"
    :height="height"
    preserveAspectRatio="none"
  >
    <polyline
      v-if="points.length > 1"
      :points="polylinePoints"
      fill="none"
      :stroke="stroke"
      stroke-width="1.5"
      stroke-linejoin="round"
      stroke-linecap="round"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  points: number[]
  width?: number
  height?: number
  color?: 'brand' | 'success' | 'warning' | 'danger'
}
const props = withDefaults(defineProps<Props>(), {
  width: 80,
  height: 24,
  color: 'brand',
})

const stroke = computed(() => {
  return {
    brand: 'var(--ui-brand)',
    success: 'var(--ui-success)',
    warning: 'var(--ui-warning)',
    danger: 'var(--ui-danger)',
  }[props.color]
})

const polylinePoints = computed(() => {
  if (!props.points.length) return ''
  const min = Math.min(...props.points)
  const max = Math.max(...props.points)
  const range = max - min || 1
  return props.points
    .map((v, i) => {
      const x = (i / (props.points.length - 1 || 1)) * props.width
      const y = props.height - ((v - min) / range) * props.height
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
})
</script>

<style scoped>
.ui-sparkline { display: block; }
</style>
