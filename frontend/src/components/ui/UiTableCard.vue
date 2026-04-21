<template>
  <UiCard :title="title" :padding="padding">
    <template v-if="$slots.extra" #extra>
      <slot name="extra" />
    </template>
    <slot />
  </UiCard>
</template>

<script setup lang="ts">
import UiCard from './UiCard.vue'

interface Props {
  title?: string
  padding?: 'none' | 'sm' | 'md' | 'lg'
  /** @deprecated back-compat */
  bordered?: boolean
  /** @deprecated back-compat */
  shadow?: boolean
}
withDefaults(defineProps<Props>(), { padding: 'none' })
</script>
