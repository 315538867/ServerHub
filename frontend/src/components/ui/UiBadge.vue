<template>
  <span
    class="ui-badge"
    :class="[`ui-badge--${tone}`, `ui-badge--${variant}`, dot && 'ui-badge--with-dot']"
  >
    <span v-if="dot" class="ui-badge__dot" />
    <slot />
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  tone?: 'neutral' | 'brand' | 'success' | 'warning' | 'danger' | 'info'
  variant?: 'soft' | 'solid' | 'outline'
  dot?: boolean
}>(), {
  tone: 'neutral',
  variant: 'soft',
  dot: false,
})
</script>

<style scoped>
.ui-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 20px;
  padding: 0 7px;
  border-radius: var(--ui-radius-sm);
  font-size: var(--ui-fs-2xs);
  font-weight: var(--ui-fw-medium);
  line-height: 1;
  letter-spacing: 0.01em;
  border: 1px solid transparent;
  white-space: nowrap;
}
.ui-badge__dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}

/* Soft */
.ui-badge--soft.ui-badge--neutral { background: var(--ui-muted-soft); color: var(--ui-fg-2); }
.ui-badge--soft.ui-badge--brand   { background: var(--ui-brand-soft); color: var(--ui-brand-fg); }
.ui-badge--soft.ui-badge--success { background: var(--ui-success-soft); color: var(--ui-success-fg); }
.ui-badge--soft.ui-badge--warning { background: var(--ui-warning-soft); color: var(--ui-warning-fg); }
.ui-badge--soft.ui-badge--danger  { background: var(--ui-danger-soft);  color: var(--ui-danger-fg); }
.ui-badge--soft.ui-badge--info    { background: var(--ui-info-soft);    color: var(--ui-info-fg); }

/* Solid */
.ui-badge--solid { color: #fff; }
.ui-badge--solid.ui-badge--neutral { background: var(--ui-muted); }
.ui-badge--solid.ui-badge--brand   { background: var(--ui-brand); }
.ui-badge--solid.ui-badge--success { background: var(--ui-success); }
.ui-badge--solid.ui-badge--warning { background: var(--ui-warning); }
.ui-badge--solid.ui-badge--danger  { background: var(--ui-danger); }
.ui-badge--solid.ui-badge--info    { background: var(--ui-info); }

/* Outline */
.ui-badge--outline { background: transparent; }
.ui-badge--outline.ui-badge--neutral { border-color: var(--ui-border); color: var(--ui-fg-2); }
.ui-badge--outline.ui-badge--brand   { border-color: var(--ui-brand); color: var(--ui-brand); }
.ui-badge--outline.ui-badge--success { border-color: var(--ui-success); color: var(--ui-success); }
.ui-badge--outline.ui-badge--warning { border-color: var(--ui-warning); color: var(--ui-warning); }
.ui-badge--outline.ui-badge--danger  { border-color: var(--ui-danger); color: var(--ui-danger); }
.ui-badge--outline.ui-badge--info    { border-color: var(--ui-info);   color: var(--ui-info); }
</style>
