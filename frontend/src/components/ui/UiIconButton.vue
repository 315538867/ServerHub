<template>
  <button
    class="ui-icon-btn"
    :class="[`ui-icon-btn--${variant}`, `ui-icon-btn--${size}`, active && 'is-active']"
    :aria-label="ariaLabel"
    :title="title"
    :disabled="disabled"
    @click="$emit('click', $event)"
  >
    <slot />
    <span v-if="badge" class="ui-icon-btn__badge">{{ badge }}</span>
  </button>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  variant?: 'ghost' | 'soft' | 'solid'
  size?: 'sm' | 'md' | 'lg'
  active?: boolean
  disabled?: boolean
  ariaLabel?: string
  title?: string
  badge?: string | number
}>(), {
  variant: 'ghost',
  size: 'md',
  active: false,
  disabled: false,
})

defineEmits<{ (e: 'click', evt: MouseEvent): void }>()
</script>

<style scoped>
.ui-icon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid transparent;
  color: var(--ui-fg-2);
  border-radius: var(--ui-radius-md);
  cursor: pointer;
  transition: background-color var(--ui-dur-fast) var(--ui-ease-standard),
              color var(--ui-dur-fast) var(--ui-ease-standard),
              border-color var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-icon-btn:hover:not(:disabled) { background: var(--ui-bg-hover); color: var(--ui-fg); }
.ui-icon-btn:disabled { opacity: .4; cursor: not-allowed; }
.ui-icon-btn.is-active { background: var(--ui-brand-soft); color: var(--ui-brand-fg); }

.ui-icon-btn--sm { width: 26px; height: 26px; font-size: 14px; }
.ui-icon-btn--md { width: 30px; height: 30px; font-size: 15px; }
.ui-icon-btn--lg { width: 36px; height: 36px; font-size: 16px; }

.ui-icon-btn--soft  { background: var(--ui-bg-subtle); }
.ui-icon-btn--solid { background: var(--ui-brand); color: #fff; }
.ui-icon-btn--solid:hover:not(:disabled) { background: var(--ui-brand-hover); color: #fff; }

.ui-icon-btn__badge {
  position: absolute;
  top: 0; right: 0;
  transform: translate(40%, -40%);
  background: var(--ui-danger);
  color: #fff;
  font-size: 9.5px;
  font-weight: var(--ui-fw-semibold);
  min-width: 14px; height: 14px;
  padding: 0 4px;
  border-radius: var(--ui-radius-pill);
  display: flex; align-items: center; justify-content: center;
  line-height: 1;
  border: 1.5px solid var(--ui-bg-surface);
}
</style>
