<template>
  <button
    :type="nativeType"
    class="ui-btn"
    :class="[
      `ui-btn--${variant}`,
      `ui-btn--${size}`,
      block && 'ui-btn--block',
      loading && 'is-loading',
    ]"
    :disabled="disabled || loading"
    @click="$emit('click', $event)"
  >
    <span v-if="loading" class="ui-btn__spinner" />
    <span v-else-if="$slots.icon || icon" class="ui-btn__icon">
      <slot name="icon"><component :is="icon" v-if="icon" /></slot>
    </span>
    <span v-if="$slots.default" class="ui-btn__label"><slot /></span>
    <span v-if="$slots.suffix" class="ui-btn__suffix"><slot name="suffix" /></span>
  </button>
</template>

<script setup lang="ts">
import type { Component } from 'vue'

withDefaults(defineProps<{
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'soft'
  size?: 'sm' | 'md' | 'lg'
  icon?: Component
  loading?: boolean
  disabled?: boolean
  block?: boolean
  nativeType?: 'button' | 'submit' | 'reset'
}>(), {
  variant: 'secondary',
  size: 'md',
  loading: false,
  disabled: false,
  block: false,
  nativeType: 'button',
})

defineEmits<{ (e: 'click', evt: MouseEvent): void }>()
</script>

<style scoped>
.ui-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-family: inherit;
  font-weight: var(--ui-fw-medium);
  border-radius: var(--ui-radius-md);
  border: 1px solid transparent;
  background: transparent;
  color: var(--ui-fg);
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: background-color var(--ui-dur-fast) var(--ui-ease-standard),
              border-color var(--ui-dur-fast) var(--ui-ease-standard),
              color var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard),
              box-shadow var(--ui-dur-fast) var(--ui-ease-standard);
  position: relative;
  overflow: hidden;
}
.ui-btn:hover:not(:disabled) { transform: translateY(-1px); }
.ui-btn:active:not(:disabled) { transform: translateY(0); }
.ui-btn:disabled, .ui-btn.is-loading { cursor: not-allowed; opacity: .55; }
.ui-btn--block { width: 100%; }

/* Sizes */
.ui-btn--sm { height: 26px; padding: 0 10px; font-size: var(--ui-fs-xs); }
.ui-btn--md { height: 30px; padding: 0 12px; font-size: var(--ui-fs-sm); }
.ui-btn--lg { height: 36px; padding: 0 16px; font-size: var(--ui-fs-md); }

/* Variants */
.ui-btn--primary {
  background: var(--ui-brand);
  color: var(--ui-fg-on-brand);
  border-color: var(--ui-brand);
  box-shadow: 0 1px 0 rgba(0,0,0,.06), inset 0 1px 0 rgba(255,255,255,.12);
}
.ui-btn--primary:hover:not(:disabled) {
  background: var(--ui-brand-hover);
  border-color: var(--ui-brand-hover);
  box-shadow: 0 4px 12px var(--ui-brand-ring);
}

.ui-btn--secondary {
  background: var(--ui-bg-surface);
  color: var(--ui-fg);
  border-color: var(--ui-border);
}
.ui-btn--secondary:hover:not(:disabled) {
  background: var(--ui-bg-hover);
  border-color: var(--ui-border-strong);
}

.ui-btn--ghost {
  background: transparent;
  color: var(--ui-fg-2);
}
.ui-btn--ghost:hover:not(:disabled) {
  background: var(--ui-bg-hover);
  color: var(--ui-fg);
}

.ui-btn--soft {
  background: var(--ui-brand-soft);
  color: var(--ui-brand-fg);
  border-color: transparent;
}
.ui-btn--soft:hover:not(:disabled) {
  background: var(--ui-brand-soft-2);
}

.ui-btn--danger {
  background: var(--ui-danger);
  color: #fff;
  border-color: var(--ui-danger);
}
.ui-btn--danger:hover:not(:disabled) {
  background: var(--p-red-600);
  border-color: var(--p-red-600);
  box-shadow: 0 4px 12px rgba(220, 38, 38, .25);
}

.ui-btn__icon { display: inline-flex; font-size: 14px; }
.ui-btn__suffix { display: inline-flex; opacity: .65; }

.ui-btn__spinner {
  width: 12px; height: 12px;
  border-radius: 50%;
  border: 1.5px solid currentColor;
  border-right-color: transparent;
  animation: ui-spin .8s linear infinite;
}

.ui-btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--ui-brand-ring);
}
</style>
