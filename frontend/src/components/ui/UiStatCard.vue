<template>
  <UiCard :title="title" :padding="'none'">
    <template v-if="$slots.extra" #extra>
      <slot name="extra" />
    </template>
    <div class="ui-stat__body">
      <div class="ui-stat__value-row">
        <span class="ui-stat__value mono">{{ value }}</span>
        <span v-if="suffix" class="ui-stat__suffix">{{ suffix }}</span>
      </div>
      <p v-if="hint" class="ui-stat__hint">{{ hint }}</p>
      <div v-if="trend != null" class="ui-stat__trend" :class="trendClass">
        <span class="ui-stat__trend-sign">{{ trend >= 0 ? '↑' : '↓' }}</span>
        {{ Math.abs(trend) }}%
        <span v-if="trendLabel" class="ui-stat__trend-label">{{ trendLabel }}</span>
      </div>
      <slot />
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiCard from './UiCard.vue'

interface Props {
  title?: string
  label?: string
  value: string | number
  suffix?: string
  hint?: string
  trend?: number | null
  trendLabel?: string
  /** @deprecated back-compat */
  tone?: string
  /** @deprecated back-compat */
  interactive?: boolean
}
const props = defineProps<Props>()

const trendClass = computed(() => {
  if (props.trend == null) return ''
  return props.trend > 0 ? 'is-up' : props.trend < 0 ? 'is-down' : 'is-flat'
})
</script>

<style scoped>
.ui-stat__body {
  padding: var(--space-5);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.ui-stat__value-row {
  display: flex; align-items: baseline; gap: var(--space-2);
}
.ui-stat__value {
  font-size: var(--fs-3xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.02em;
  line-height: 1.1;
}
.ui-stat__suffix { font-size: var(--fs-sm); color: var(--ui-fg-3); }
.ui-stat__hint { font-size: var(--fs-xs); color: var(--ui-fg-3); }
.ui-stat__trend {
  display: inline-flex; align-items: center; gap: var(--space-1);
  font-size: var(--fs-xs);
  font-weight: var(--fw-medium);
  margin-top: var(--space-1);
}
.ui-stat__trend.is-up   { color: var(--ui-success-fg); }
.ui-stat__trend.is-down { color: var(--ui-danger-fg); }
.ui-stat__trend.is-flat { color: var(--ui-fg-3); }
.ui-stat__trend-sign    { font-weight: var(--fw-bold); }
.ui-stat__trend-label   { color: var(--ui-fg-4); margin-left: var(--space-1); }
</style>
