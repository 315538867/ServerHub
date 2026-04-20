<template>
  <section class="ui-card" :class="{ 'ui-card--bordered': bordered, 'ui-card--flat': !shadow }">
    <header v-if="title || $slots.title || $slots.extra" class="ui-card__header">
      <div class="ui-card__title">
        <slot name="title">{{ title }}</slot>
      </div>
      <div v-if="$slots.extra" class="ui-card__extra">
        <slot name="extra" />
      </div>
    </header>
    <div class="ui-card__body" :class="{ 'ui-card__body--padded': padded }">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="ui-card__footer">
      <slot name="footer" />
    </footer>
  </section>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  bordered?: boolean
  shadow?: boolean
  padded?: boolean
}>(), {
  bordered: true,
  shadow: true,
  padded: true,
})
</script>

<style scoped>
.ui-card {
  background: var(--ui-bg-elevated);
  border-radius: var(--ui-radius-lg);
  margin-bottom: var(--ui-space-4);
  overflow: hidden;
}
.ui-card:last-child { margin-bottom: 0; }
.ui-card--bordered { border: 1px solid var(--ui-border); }
.ui-card:not(.ui-card--flat) { box-shadow: var(--ui-shadow-xs); }

.ui-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--ui-space-4);
  padding: var(--ui-space-4) var(--ui-space-6);
  border-bottom: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
}
.ui-card__title {
  font-size: var(--ui-fs-md);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  line-height: var(--ui-lh-tight);
  min-width: 0;
}
.ui-card__extra {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  flex-shrink: 0;
}
.ui-card__body--padded { padding: var(--ui-space-4) var(--ui-space-6) var(--ui-space-6); }
.ui-card__footer {
  padding: var(--ui-space-3) var(--ui-space-6);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-subtle);
}
</style>
