<template>
  <div
    class="ui-skeleton"
    :class="[circle && 'ui-skeleton--circle']"
    :style="{ width: typeof width === 'number' ? width + 'px' : width, height: typeof height === 'number' ? height + 'px' : height }"
  />
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  width?: string | number
  height?: string | number
  circle?: boolean
}>(), {
  width: '100%',
  height: 12,
  circle: false,
})
</script>

<style scoped>
.ui-skeleton {
  display: inline-block;
  background: linear-gradient(
    90deg,
    var(--ui-bg-subtle) 0%,
    var(--ui-bg-hover) 50%,
    var(--ui-bg-subtle) 100%
  );
  background-size: 200% 100%;
  animation: ui-shimmer 1.4s linear infinite;
  border-radius: var(--ui-radius-sm);
}
.ui-skeleton--circle { border-radius: 50%; }
</style>
