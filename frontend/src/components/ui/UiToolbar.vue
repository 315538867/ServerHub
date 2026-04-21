<template>
  <div class="ui-toolbar" :class="[sticky && 'ui-toolbar--sticky', bordered && 'ui-toolbar--bordered']">
    <div class="ui-toolbar__left">
      <slot name="left" />
    </div>
    <div class="ui-toolbar__right">
      <slot name="right" />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  sticky?: boolean
  bordered?: boolean
}>(), {
  sticky: false,
  bordered: false,
})
</script>

<style scoped>
.ui-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--ui-space-3);
  padding: var(--ui-space-3) 0;
}
.ui-toolbar__left,
.ui-toolbar__right {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  min-width: 0;
}
.ui-toolbar__left { flex: 1; min-width: 0; }
.ui-toolbar--sticky {
  position: sticky; top: 0;
  z-index: var(--ui-z-sticky);
  background: var(--ui-bg-canvas);
}
.ui-toolbar--bordered { border-bottom: 1px solid var(--ui-border); }
</style>
