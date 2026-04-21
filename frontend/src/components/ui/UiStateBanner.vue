<template>
  <div class="ui-banner" :class="[`ui-banner--${tone}`]">
    <div class="ui-banner__icon">
      <component :is="icon" :size="18" />
    </div>
    <div class="ui-banner__main">
      <div class="ui-banner__head">
        <h3 v-if="title" class="ui-banner__title">{{ title }}</h3>
        <slot name="title" />
      </div>
      <p v-if="description" class="ui-banner__desc">{{ description }}</p>
      <slot />
    </div>
    <div v-if="$slots.actions" class="ui-banner__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Info, CheckCircle2, AlertTriangle, XCircle } from 'lucide-vue-next'

interface Props {
  tone?: 'info' | 'success' | 'warning' | 'danger'
  title?: string
  description?: string
}
const props = withDefaults(defineProps<Props>(), { tone: 'info' })

const icon = computed(() => ({
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  danger: XCircle,
}[props.tone]))
</script>

<style scoped>
.ui-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  background: var(--ui-bg-1);
}
.ui-banner__icon { flex-shrink: 0; margin-top: 2px; }
.ui-banner__main { flex: 1; min-width: 0; }
.ui-banner__title { font-size: var(--fs-sm); font-weight: var(--fw-semibold); color: var(--ui-fg); }
.ui-banner__desc  { font-size: var(--fs-sm); color: var(--ui-fg-2); margin-top: var(--space-1); line-height: var(--lh-relaxed); }
.ui-banner__actions { display: flex; align-items: center; gap: var(--space-2); flex-shrink: 0; }

.ui-banner--info    { background: var(--ui-info-soft); border-color: transparent; color: var(--ui-info-fg); }
.ui-banner--success { background: var(--ui-success-soft); border-color: transparent; color: var(--ui-success-fg); }
.ui-banner--warning { background: var(--ui-warning-soft); border-color: transparent; color: var(--ui-warning-fg); }
.ui-banner--danger  { background: var(--ui-danger-soft); border-color: transparent; color: var(--ui-danger-fg); }
</style>
