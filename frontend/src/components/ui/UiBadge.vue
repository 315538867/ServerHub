<template>
  <span
    class="ui-badge"
    :class="[`ui-badge--${resolvedTone}`, `ui-badge--${size}`, dot && 'ui-badge--with-dot']"
  >
    <span v-if="dot" class="ui-badge__dot" />
    <slot />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  tone?: 'neutral' | 'brand' | 'success' | 'warning' | 'danger' | 'info'
  variant?: 'solid' | 'soft' | 'outline'
  size?: 'sm' | 'md'
  dot?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  tone: 'neutral',
  variant: 'soft',
  size: 'md',
})
const resolvedTone = computed(() => props.tone)
</script>

<style scoped>
.ui-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  border-radius: 4px;
  font-weight: var(--fw-medium);
  white-space: nowrap;
  border: 1px solid transparent;
  line-height: 1;
}
.ui-badge--sm { height: 18px; padding: 0 6px; font-size: 11px; }
.ui-badge--md { height: 22px; padding: 0 8px; font-size: var(--fs-xs); }

.ui-badge__dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.9;
}

.ui-badge--neutral { background: var(--ui-muted-soft); color: var(--ui-fg-2); border-color: var(--ui-border); }
.ui-badge--brand   { background: var(--ui-brand-soft); color: var(--ui-brand-fg); }
.ui-badge--success { background: var(--ui-success-soft); color: var(--ui-success-fg); }
.ui-badge--warning { background: var(--ui-warning-soft); color: var(--ui-warning-fg); }
.ui-badge--danger  { background: var(--ui-danger-soft); color: var(--ui-danger-fg); }
.ui-badge--info    { background: var(--ui-info-soft); color: var(--ui-info-fg); }
</style>
