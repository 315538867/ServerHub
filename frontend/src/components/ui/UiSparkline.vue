<template>
  <svg
    class="ui-sparkline"
    :viewBox="`0 0 ${width} ${height}`"
    :width="width"
    :height="height"
    preserveAspectRatio="none"
  >
    <defs>
      <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="color" stop-opacity="0.25" />
        <stop offset="100%" :stop-color="color" stop-opacity="0" />
      </linearGradient>
    </defs>
    <path v-if="area" :d="areaPath" :fill="`url(#${gradId})`" />
    <path
      class="ui-sparkline__line"
      :d="linePath"
      :stroke="color"
      stroke-width="1.5"
      fill="none"
      stroke-linejoin="round"
      stroke-linecap="round"
      :style="{ '--ui-draw-len': lineLen }"
    />
    <circle
      v-if="showLast"
      :cx="lastPoint.x" :cy="lastPoint.y" r="2"
      :fill="color" :stroke="'var(--ui-bg-surface)'" stroke-width="1"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(defineProps<{
  data: number[]
  width?: number
  height?: number
  color?: string
  area?: boolean
  showLast?: boolean
}>(), {
  width: 88,
  height: 28,
  color: 'var(--ui-brand)',
  area: true,
  showLast: true,
})

const gradId = 'sp-' + useId()

const points = computed(() => {
  const d = props.data.length > 1 ? props.data : [0, 0]
  const min = Math.min(...d)
  const max = Math.max(...d)
  const range = max - min || 1
  const pad = 2
  const innerW = props.width - pad * 2
  const innerH = props.height - pad * 2
  return d.map((v, i) => ({
    x: pad + (d.length === 1 ? 0 : (i / (d.length - 1)) * innerW),
    y: pad + innerH - ((v - min) / range) * innerH,
  }))
})

const linePath = computed(() => {
  const p = points.value
  if (!p.length) return ''
  return 'M ' + p.map(pt => `${pt.x.toFixed(1)} ${pt.y.toFixed(1)}`).join(' L ')
})

const areaPath = computed(() => {
  const p = points.value
  if (!p.length) return ''
  const base = `M ${p[0].x} ${props.height} `
  const line = p.map(pt => `L ${pt.x.toFixed(1)} ${pt.y.toFixed(1)}`).join(' ')
  return `${base} ${line} L ${p[p.length - 1].x} ${props.height} Z`
})

const lastPoint = computed(() => points.value[points.value.length - 1] || { x: 0, y: 0 })
const lineLen = computed(() => {
  const p = points.value
  if (p.length < 2) return 0
  let len = 0
  for (let i = 1; i < p.length; i++) {
    const dx = p[i].x - p[i - 1].x
    const dy = p[i].y - p[i - 1].y
    len += Math.sqrt(dx * dx + dy * dy)
  }
  return Math.ceil(len)
})
</script>

<style scoped>
.ui-sparkline { display: block; overflow: visible; }
.ui-sparkline__line {
  stroke-dasharray: var(--ui-draw-len);
  stroke-dashoffset: var(--ui-draw-len);
  animation: ui-draw 800ms var(--ui-ease-standard) forwards;
}
</style>
