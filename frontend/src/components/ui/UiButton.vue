<template>
  <button
    :type="nativeType"
    class="ui-btn"
    :class="[
      `ui-btn--${resolvedVariant}`,
      `ui-btn--${size}`,
      block && 'ui-btn--block',
      loading && 'is-loading',
      (disabled || loading) && 'is-disabled',
    ]"
    :disabled="disabled || loading"
    @click="onClick"
  >
    <span v-if="$slots.icon || loading" class="ui-btn__icon">
      <span v-if="loading" class="ui-btn__spinner" />
      <slot v-else name="icon" />
    </span>
    <span v-if="$slots.default" class="ui-btn__label"><slot /></span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'link' | 'default' | 'text' | 'outline' | 'base'
  theme?: 'primary' | 'default' | 'danger' | 'warning' | 'success'
  size?: 'sm' | 'md' | 'lg' | 'small' | 'medium' | 'large'
  nativeType?: 'button' | 'submit' | 'reset'
  loading?: boolean
  disabled?: boolean
  block?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  variant: 'secondary',
  size: 'md',
  nativeType: 'button',
})

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>()
function onClick(ev: MouseEvent) {
  if (props.disabled || props.loading) return
  emit('click', ev)
}

const resolvedVariant = computed(() => {
  if (props.theme === 'danger') return 'danger'
  if (props.theme === 'primary' && props.variant !== 'text' && props.variant !== 'ghost') return 'primary'
  if (props.variant === 'default') return 'secondary'
  if (props.variant === 'text') return 'ghost'
  if (props.variant === 'outline') return 'secondary'
  if (props.variant === 'base') return 'primary'
  return props.variant
})

const size = computed(() => {
  if (props.size === 'small') return 'sm'
  if (props.size === 'medium') return 'md'
  if (props.size === 'large') return 'lg'
  return props.size
})
</script>

<style scoped>
.ui-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  font-family: inherit;
  font-weight: var(--fw-medium);
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  cursor: pointer;
  white-space: nowrap;
  user-select: none;
  transition: background-color var(--dur-fast) var(--ease),
              border-color var(--dur-fast) var(--ease),
              color var(--dur-fast) var(--ease),
              box-shadow var(--dur-fast) var(--ease);
}
.ui-btn:focus-visible { box-shadow: var(--shadow-ring); }

.ui-btn--sm { height: var(--control-h-sm); padding: 0 var(--space-3); font-size: var(--fs-xs); }
.ui-btn--md { height: var(--control-h-md); padding: 0 var(--space-4); font-size: var(--fs-sm); }
.ui-btn--lg { height: var(--control-h-lg); padding: 0 var(--space-5); font-size: var(--fs-md); }
.ui-btn--block { width: 100%; }

.ui-btn--primary {
  background: var(--ui-brand);
  color: var(--ui-fg-on-brand);
  border-color: var(--ui-brand);
}
.ui-btn--primary:hover:not(.is-disabled) {
  background: var(--ui-brand-hover);
  border-color: var(--ui-brand-hover);
}
.ui-btn--primary:active:not(.is-disabled) {
  background: var(--ui-brand-active);
  border-color: var(--ui-brand-active);
}

.ui-btn--secondary {
  background: var(--ui-bg-1);
  color: var(--ui-fg);
  border-color: var(--ui-border);
}
.ui-btn--secondary:hover:not(.is-disabled) {
  background: var(--ui-bg-2);
  border-color: var(--ui-border-strong);
}

.ui-btn--ghost {
  background: transparent;
  color: var(--ui-fg-2);
}
.ui-btn--ghost:hover:not(.is-disabled) {
  background: var(--ui-bg-2);
  color: var(--ui-fg);
}

.ui-btn--danger {
  background: var(--ui-danger);
  color: #fff;
  border-color: var(--ui-danger);
}
.ui-btn--danger:hover:not(.is-disabled) {
  background: var(--p-red-600);
  border-color: var(--p-red-600);
}

.ui-btn--link {
  background: transparent;
  color: var(--ui-brand-fg);
  height: auto;
  padding: 0;
}
.ui-btn--link:hover:not(.is-disabled) { text-decoration: underline; }

.ui-btn.is-disabled { cursor: not-allowed; opacity: 0.5; }
.ui-btn__icon { display: inline-flex; align-items: center; flex-shrink: 0; }

.ui-btn__spinner {
  width: 12px; height: 12px;
  border: 1.5px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: ui-btn-spin 0.6s linear infinite;
}
@keyframes ui-btn-spin { to { transform: rotate(360deg); } }
</style>
