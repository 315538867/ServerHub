<template>
  <button
    :type="nativeType"
    class="ui-icon-btn"
    :class="[`ui-icon-btn--${variant}`, `ui-icon-btn--${size}`, active && 'is-active', disabled && 'is-disabled']"
    :disabled="disabled"
    :title="title"
    :aria-label="ariaLabel || title"
    @click="$emit('click', $event)"
  >
    <slot />
    <span v-if="badge" class="ui-icon-btn__badge">{{ badge }}</span>
  </button>
</template>

<script setup lang="ts">
interface Props {
  variant?: 'ghost' | 'subtle' | 'outline'
  size?: 'sm' | 'md'
  title?: string
  ariaLabel?: string
  disabled?: boolean
  active?: boolean
  badge?: string | number
  nativeType?: 'button' | 'submit'
}
withDefaults(defineProps<Props>(), {
  variant: 'ghost',
  size: 'md',
  nativeType: 'button',
})
defineEmits<{ (e: 'click', ev: MouseEvent): void }>()
</script>

<style scoped>
.ui-icon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  cursor: pointer;
  color: var(--ui-fg-2);
  border: 1px solid transparent;
  background: transparent;
  transition: background-color var(--dur-fast) var(--ease),
              color var(--dur-fast) var(--ease),
              border-color var(--dur-fast) var(--ease);
}
.ui-icon-btn--sm { width: 28px; height: 28px; }
.ui-icon-btn--md { width: 32px; height: 32px; }

.ui-icon-btn:focus-visible { box-shadow: var(--shadow-ring); }
.ui-icon-btn:hover:not(.is-disabled) {
  background: var(--ui-bg-2);
  color: var(--ui-fg);
}
.ui-icon-btn.is-active {
  background: var(--ui-brand-soft);
  color: var(--ui-brand-fg);
}
.ui-icon-btn--subtle  { background: var(--ui-bg-2); }
.ui-icon-btn--outline { border-color: var(--ui-border); }
.ui-icon-btn--outline:hover { border-color: var(--ui-border-strong); }
.ui-icon-btn.is-disabled { opacity: 0.5; cursor: not-allowed; }

.ui-icon-btn__badge {
  position: absolute;
  top: -2px; right: -2px;
  min-width: 16px; height: 16px;
  padding: 0 4px;
  background: var(--ui-danger);
  color: #fff;
  border-radius: var(--radius-pill);
  font-size: 10px;
  font-weight: var(--fw-bold);
  display: flex; align-items: center; justify-content: center;
  border: 2px solid var(--ui-bg-1);
  line-height: 1;
}
</style>
