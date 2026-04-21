<template>
  <component
    :is="tag"
    class="ui-card"
    :class="[
      `ui-card--pad-${padding}`,
      hoverable && 'ui-card--hoverable',
      bordered && 'ui-card--bordered',
      flat && 'ui-card--flat',
    ]"
    :style="customStyle"
  >
    <header v-if="$slots.header || $slots.extra || title" class="ui-card__header">
      <div class="ui-card__title-wrap">
        <h3 v-if="title" class="ui-card__title">{{ title }}</h3>
        <p v-if="description" class="ui-card__desc">{{ description }}</p>
        <slot name="header" />
      </div>
      <div v-if="$slots.extra || $slots.actions" class="ui-card__actions">
        <slot name="extra" />
        <slot name="actions" />
      </div>
    </header>
    <div class="ui-card__body" :class="bodyClass">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="ui-card__footer">
      <slot name="footer" />
    </footer>
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  tag?: string
  title?: string
  description?: string
  padding?: 'none' | 'sm' | 'md' | 'lg'
  padded?: boolean
  bordered?: boolean
  hoverable?: boolean
  flat?: boolean
  shadow?: boolean | string
  bodyClass?: string
  minHeight?: number | string
}

const props = withDefaults(defineProps<Props>(), {
  tag: 'section',
  padding: 'md',
  padded: true,
  bordered: true,
  hoverable: false,
  flat: false,
})

const customStyle = computed(() => {
  const s: Record<string, string> = {}
  if (props.minHeight) s.minHeight = typeof props.minHeight === 'number' ? `${props.minHeight}px` : props.minHeight
  if (props.padded === false) s.padding = '0'
  return Object.keys(s).length ? s : undefined
})
</script>

<style scoped>
.ui-card {
  background: var(--ui-bg-1);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  transition: border-color var(--dur-base) var(--ease),
              box-shadow var(--dur-base) var(--ease);
}
.ui-card--bordered { border: 1px solid var(--ui-border); }
.ui-card--flat { background: transparent; border: none; }
.ui-card--hoverable:hover {
  border-color: var(--ui-border-strong);
  box-shadow: var(--shadow-md);
}

.ui-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-5) var(--space-5) var(--space-3);
}
.ui-card__title-wrap { min-width: 0; flex: 1; }
.ui-card__title {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.01em;
}
.ui-card__desc {
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  margin-top: var(--space-1);
}
.ui-card__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-shrink: 0;
}

.ui-card__body { min-width: 0; flex: 1; }
.ui-card--pad-none .ui-card__body { padding: 0; }
.ui-card--pad-sm   .ui-card__body { padding: var(--space-3) var(--space-4); }
.ui-card--pad-md   .ui-card__body { padding: var(--space-5); }
.ui-card--pad-lg   .ui-card__body { padding: var(--space-6); }

.ui-card__header + .ui-card__body { padding-top: 0; }

.ui-card__footer {
  padding: var(--space-3) var(--space-5);
  border-top: 1px solid var(--ui-border);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
}
</style>
