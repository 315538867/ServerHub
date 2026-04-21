<template>
  <div class="ui-toolbar" :class="[sticky && 'ui-toolbar--sticky']">
    <div class="ui-toolbar__left">
      <slot name="left" />
    </div>
    <div class="ui-toolbar__right">
      <slot name="right" />
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  sticky?: boolean
  /** @deprecated back-compat */
  bordered?: boolean
}
withDefaults(defineProps<Props>(), {})
</script>

<style scoped>
.ui-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3) 0;
}
.ui-toolbar--sticky {
  position: sticky;
  top: 0;
  background: var(--ui-bg);
  z-index: var(--z-sticky);
}
.ui-toolbar__left,
.ui-toolbar__right {
  display: flex; align-items: center; gap: var(--space-2);
  min-width: 0;
}
.ui-toolbar__left { flex: 1; }
</style>
