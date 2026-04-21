<template>
  <div class="ui-stat" :class="[`ui-stat--${tone}`, interactive && 'ui-stat--interactive']">
    <div class="ui-stat__top">
      <span class="ui-stat__label">{{ label }}</span>
      <span v-if="$slots.icon" class="ui-stat__icon"><slot name="icon" /></span>
    </div>
    <div class="ui-stat__value-row">
      <div class="ui-stat__value tabular">
        <span class="ui-stat__value-num">{{ display }}</span>
        <span v-if="unit" class="ui-stat__value-unit">{{ unit }}</span>
      </div>
      <UiSparkline
        v-if="trend && trend.length > 1"
        :data="trend"
        :color="toneColor"
        :width="96"
        :height="28"
      />
    </div>
    <div class="ui-stat__foot" v-if="delta !== undefined || $slots.foot">
      <span v-if="delta !== undefined" class="ui-stat__delta" :class="deltaClass">
        <svg v-if="delta > 0" width="10" height="10" viewBox="0 0 10 10"><path d="M5 2 L8 7 L2 7 Z" fill="currentColor"/></svg>
        <svg v-else-if="delta < 0" width="10" height="10" viewBox="0 0 10 10"><path d="M5 8 L8 3 L2 3 Z" fill="currentColor"/></svg>
        <span class="tabular">{{ Math.abs(delta) }}{{ deltaUnit ?? '%' }}</span>
      </span>
      <slot name="foot" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, toRef } from 'vue'
import UiSparkline from './UiSparkline.vue'
import { useCountUp } from '@/composables/useCountUp'

const props = withDefaults(defineProps<{
  label: string
  value: number | string
  unit?: string
  delta?: number
  deltaUnit?: string
  tone?: 'neutral' | 'brand' | 'success' | 'warning' | 'danger' | 'info'
  trend?: number[]
  interactive?: boolean
  decimals?: number
  /** disable countUp — use when value is a non-numeric string like "正常" */
  animate?: boolean
}>(), {
  tone: 'neutral',
  interactive: false,
  decimals: 0,
  animate: true,
})

const numeric = computed(() => typeof props.value === 'number' ? props.value : NaN)
const shouldAnimate = computed(() => props.animate && !Number.isNaN(numeric.value))
const counted = useCountUp(() => Number.isNaN(numeric.value) ? 0 : numeric.value, { decimals: props.decimals })

const display = computed(() => {
  if (typeof props.value === 'string') return props.value
  return shouldAnimate.value ? counted.value : numeric.value
})

const toneColor = computed(() => ({
  neutral: 'var(--ui-fg-3)',
  brand:   'var(--ui-brand)',
  success: 'var(--ui-success)',
  warning: 'var(--ui-warning)',
  danger:  'var(--ui-danger)',
  info:    'var(--ui-info)',
}[props.tone]))

const deltaClass = computed(() => {
  if (props.delta === undefined || props.delta === 0) return 'ui-stat__delta--flat'
  return props.delta > 0 ? 'ui-stat__delta--up' : 'ui-stat__delta--down'
})

// Suppress unused-ref warning in templates
void toRef(props, 'tone')
</script>

<style scoped>
.ui-stat {
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  padding: var(--ui-space-3) var(--ui-space-4);
  display: flex;
  flex-direction: column;
  gap: 6px;
  position: relative;
  overflow: hidden;
  transition: box-shadow var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard),
              border-color var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-stat--interactive { cursor: pointer; }
.ui-stat--interactive:hover {
  transform: translateY(-2px);
  box-shadow: var(--ui-shadow-md);
  border-color: var(--ui-border-strong);
}
.ui-stat::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: currentColor;
  opacity: .0;
  transition: opacity var(--ui-dur-base) var(--ui-ease-standard);
}
.ui-stat--brand { color: var(--ui-brand); }
.ui-stat--success { color: var(--ui-success); }
.ui-stat--warning { color: var(--ui-warning); }
.ui-stat--danger  { color: var(--ui-danger); }
.ui-stat--info    { color: var(--ui-info); }
.ui-stat--brand::before,
.ui-stat--success::before,
.ui-stat--warning::before,
.ui-stat--danger::before,
.ui-stat--info::before { opacity: .9; }

.ui-stat__top {
  display: flex; align-items: center; justify-content: space-between;
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  font-weight: var(--ui-fw-medium);
  letter-spacing: 0.01em;
}
.ui-stat__icon {
  color: inherit;
  opacity: .9;
  display: inline-flex;
  font-size: 14px;
}
.ui-stat__value-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--ui-space-3);
}
.ui-stat__value {
  color: var(--ui-fg);
  font-size: var(--ui-fs-4xl);
  font-weight: var(--ui-fw-semibold);
  line-height: 1.1;
  letter-spacing: var(--ui-tracking-tight);
  display: flex; align-items: baseline; gap: 4px;
}
.ui-stat__value-unit {
  font-size: var(--ui-fs-sm);
  font-weight: var(--ui-fw-regular);
  color: var(--ui-fg-3);
}
.ui-stat__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  gap: var(--ui-space-2);
}
.ui-stat__delta {
  display: inline-flex; align-items: center; gap: 3px;
  font-weight: var(--ui-fw-medium);
}
.ui-stat__delta--up   { color: var(--ui-success-fg); }
.ui-stat__delta--down { color: var(--ui-danger-fg); }
.ui-stat__delta--flat { color: var(--ui-fg-4); }
</style>
