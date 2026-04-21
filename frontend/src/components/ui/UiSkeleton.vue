<template>
  <div
    class="ui-skeleton"
    :class="[circle && 'ui-skeleton--circle']"
    :style="styleObj"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  width?: number | string
  height?: number | string
  circle?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  width: '100%',
  height: 16,
})
const styleObj = computed(() => ({
  width: typeof props.width === 'number' ? `${props.width}px` : props.width,
  height: typeof props.height === 'number' ? `${props.height}px` : props.height,
}))
</script>

<style scoped>
.ui-skeleton {
  display: block;
  border-radius: 4px;
  background: linear-gradient(
    90deg,
    var(--ui-bg-2) 0%,
    var(--ui-bg-3) 50%,
    var(--ui-bg-2) 100%
  );
  background-size: 200% 100%;
  animation: ui-skeleton-shimmer 1.4s ease-in-out infinite;
}
.ui-skeleton--circle { border-radius: 50%; }

@keyframes ui-skeleton-shimmer {
  0%   { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
</style>
