<template>
  <div class="ui-empty" :class="[`ui-empty--${size}`]">
    <div class="ui-empty__icon" aria-hidden="true">
      <component :is="icon" :size="iconSize" />
    </div>
    <h3 v-if="title" class="ui-empty__title">{{ title }}</h3>
    <p v-if="description" class="ui-empty__desc">{{ description }}</p>
    <div v-if="$slots.default || $slots.actions" class="ui-empty__actions">
      <slot name="actions" /><slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue'
import { Inbox } from 'lucide-vue-next'

interface Props {
  title?: string
  description?: string
  icon?: Component
  size?: 'sm' | 'md' | 'lg'
}
const props = withDefaults(defineProps<Props>(), { size: 'md', icon: () => Inbox })

const iconSize = computed(() => (props.size === 'sm' ? 24 : props.size === 'lg' ? 48 : 36))
</script>

<style scoped>
.ui-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--space-10) var(--space-6);
  color: var(--ui-fg-3);
  gap: var(--space-2);
}
.ui-empty--sm { padding: var(--space-6); }
.ui-empty__icon {
  width: 56px; height: 56px;
  display: flex; align-items: center; justify-content: center;
  color: var(--ui-fg-4);
  background: var(--ui-bg-2);
  border-radius: var(--radius-pill);
  margin-bottom: var(--space-2);
}
.ui-empty--sm .ui-empty__icon { width: 40px; height: 40px; }
.ui-empty__title {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
}
.ui-empty__desc {
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  max-width: 320px;
}
.ui-empty__actions {
  display: flex; align-items: center; gap: var(--space-2);
  margin-top: var(--space-2);
}
</style>
