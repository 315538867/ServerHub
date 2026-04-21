<template>
  <div class="ui-tabs" :class="[`ui-tabs--${variant}`]">
    <button
      v-for="item in items"
      :key="item.value"
      class="ui-tabs__item"
      :class="{ 'is-active': modelValue === item.value, 'is-disabled': item.disabled }"
      :disabled="item.disabled"
      @click="onClick(item)"
    >
      <span v-if="item.icon" class="ui-tabs__icon"><component :is="item.icon" :size="14" /></span>
      <span>{{ item.label }}</span>
      <span v-if="item.badge != null" class="ui-tabs__badge">{{ item.badge }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'

interface TabItem {
  label: string
  value: string | number
  icon?: Component
  badge?: string | number
  disabled?: boolean
}

interface Props {
  items: TabItem[]
  modelValue: string | number
  variant?: 'line' | 'pill'
  /** @deprecated back-compat */
  size?: string
}
withDefaults(defineProps<Props>(), { variant: 'line' })

const emit = defineEmits<{ (e: 'update:modelValue', v: string | number): void; (e: 'change', v: string | number): void }>()
function onClick(item: TabItem) {
  if (item.disabled) return
  emit('update:modelValue', item.value)
  emit('change', item.value)
}
</script>

<style scoped>
.ui-tabs {
  display: flex;
  align-items: center;
  gap: var(--space-5);
}
.ui-tabs--line { border-bottom: 1px solid var(--ui-border); }

.ui-tabs__item {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  height: 36px;
  padding: 0 var(--space-1);
  background: none;
  border: none;
  cursor: pointer;
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--ui-fg-3);
  transition: color var(--dur-fast) var(--ease);
}
.ui-tabs__item:hover:not(.is-disabled):not(.is-active) { color: var(--ui-fg-2); }
.ui-tabs__item.is-active {
  color: var(--ui-fg);
  font-weight: var(--fw-semibold);
}
.ui-tabs__item.is-active::after {
  content: '';
  position: absolute;
  left: 0; right: 0; bottom: -1px;
  height: 2px;
  background: var(--ui-brand);
  border-radius: 2px 2px 0 0;
}
.ui-tabs__item.is-disabled { opacity: 0.5; cursor: not-allowed; }

.ui-tabs--pill {
  border: none;
  background: var(--ui-bg-2);
  border-radius: var(--radius-sm);
  padding: var(--space-1);
  gap: var(--space-1);
}
.ui-tabs--pill .ui-tabs__item {
  height: 28px;
  padding: 0 var(--space-3);
  border-radius: 4px;
}
.ui-tabs--pill .ui-tabs__item.is-active {
  background: var(--ui-bg-1);
  color: var(--ui-fg);
  box-shadow: var(--shadow-sm);
}
.ui-tabs--pill .ui-tabs__item.is-active::after { display: none; }

.ui-tabs__icon { display: inline-flex; }
.ui-tabs__badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 18px; height: 18px; padding: 0 5px;
  background: var(--ui-bg-2);
  color: var(--ui-fg-3);
  border-radius: var(--radius-pill);
  font-size: 11px; font-weight: var(--fw-medium);
}
.ui-tabs__item.is-active .ui-tabs__badge {
  background: var(--ui-brand-soft);
  color: var(--ui-brand-fg);
}
</style>
