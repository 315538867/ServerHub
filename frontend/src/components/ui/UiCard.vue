<template>
  <section
    class="ui-card"
    :class="[
      bordered && 'ui-card--bordered',
      !shadow && 'ui-card--flat',
      hover && 'ui-card--hover',
      compact && 'ui-card--compact',
    ]"
  >
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
  hover?: boolean
  compact?: boolean
}>(), {
  bordered: true,
  shadow: true,
  padded: true,
  hover: false,
  compact: false,
})
</script>

<style scoped>
.ui-card {
  background: var(--ui-bg-surface);
  border-radius: var(--ui-radius-lg);
  margin-bottom: var(--ui-space-4);
  overflow: hidden;
  transition: box-shadow var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard),
              border-color var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-card:last-child { margin-bottom: 0; }
.ui-card--bordered { border: 1px solid var(--ui-border); }
.ui-card:not(.ui-card--flat) { box-shadow: var(--ui-shadow-xs); }
.ui-card--hover:hover {
  border-color: var(--ui-border-strong);
  box-shadow: var(--ui-shadow-md);
  transform: translateY(-1px);
}

.ui-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--ui-space-3);
  padding: var(--ui-space-3) var(--ui-space-5);
  border-bottom: 1px solid var(--ui-border);
  background: var(--ui-bg-surface);
  min-height: 44px;
}
.ui-card--compact .ui-card__header {
  min-height: 36px;
  padding: var(--ui-space-2) var(--ui-space-4);
}
.ui-card__title {
  font-size: var(--ui-fs-md);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  line-height: var(--ui-lh-tight);
  letter-spacing: var(--ui-tracking-tight);
  min-width: 0;
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
}
.ui-card__extra {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  flex-shrink: 0;
}
.ui-card__body--padded { padding: var(--ui-space-4) var(--ui-space-5); }
.ui-card--compact .ui-card__body--padded { padding: var(--ui-space-3) var(--ui-space-4); }
.ui-card__footer {
  padding: var(--ui-space-3) var(--ui-space-5);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-subtle);
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
}
</style>
