<template>
  <section class="ui-section" :class="{ 'ui-section--flat': flat }">
    <header v-if="title || $slots.extra || $slots.title" class="ui-section__header">
      <div class="ui-section__title-wrap">
        <h2 v-if="title" class="ui-section__title">{{ title }}</h2>
        <slot name="title" />
        <p v-if="description" class="ui-section__desc">{{ description }}</p>
      </div>
      <div v-if="$slots.extra" class="ui-section__extra">
        <slot name="extra" />
      </div>
    </header>
    <div class="ui-section__body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
interface Props {
  title?: string
  description?: string
  flat?: boolean
  /** @deprecated kept for back-compat */
  bordered?: boolean
  /** @deprecated kept for back-compat */
  shadow?: boolean
}
withDefaults(defineProps<Props>(), { flat: false })
</script>

<style scoped>
.ui-section { display: flex; flex-direction: column; gap: var(--space-3); }

.ui-section__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-4);
}
.ui-section__title-wrap { min-width: 0; }
.ui-section__title {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.01em;
}
.ui-section__desc {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-top: var(--space-1);
}
.ui-section__extra { display: flex; align-items: center; gap: var(--space-2); flex-shrink: 0; }

.ui-section__body { min-width: 0; }
</style>
