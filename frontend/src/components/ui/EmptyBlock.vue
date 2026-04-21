<template>
  <div class="ui-empty">
    <div class="ui-empty__icon" aria-hidden="true">
      <svg width="42" height="42" viewBox="0 0 48 48" fill="none">
        <circle cx="24" cy="24" r="20" stroke="currentColor" stroke-width="1.25" stroke-dasharray="3 3" opacity=".5"/>
        <path d="M16 30c2-2 5-3 8-3s6 1 8 3M19 21.5h.01M29 21.5h.01" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/>
      </svg>
    </div>
    <div class="ui-empty__title">{{ title || description }}</div>
    <div v-if="description && title" class="ui-empty__desc">{{ description }}</div>
    <div v-if="$slots.action" class="ui-empty__action">
      <slot name="action" />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ title?: string; description?: string }>(), {
  description: '暂无数据',
})
</script>

<style scoped>
.ui-empty {
  padding: var(--ui-space-8) var(--ui-space-5);
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  text-align: center;
  gap: var(--ui-space-2);
  min-height: 160px;
  color: var(--ui-fg-3);
}
.ui-empty__icon {
  color: var(--ui-fg-4);
  margin-bottom: 4px;
}
.ui-empty__title {
  color: var(--ui-fg-2);
  font-size: var(--ui-fs-sm);
  font-weight: var(--ui-fw-medium);
}
.ui-empty__desc {
  color: var(--ui-fg-3);
  font-size: var(--ui-fs-xs);
}
.ui-empty__action { margin-top: var(--ui-space-2); }
</style>
