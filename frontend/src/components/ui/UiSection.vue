<template>
  <UiCard v-bind="$attrs" :title="title" :bordered="bordered" :shadow="shadow" :padded="false">
    <template v-if="$slots.extra" #extra>
      <slot name="extra" />
    </template>
    <div class="ui-section__body" :class="[`ui-section__body--${padding}`]">
      <slot />
    </div>
    <template v-if="$slots.footer" #footer>
      <slot name="footer" />
    </template>
  </UiCard>
</template>

<script setup lang="ts">
import UiCard from './UiCard.vue'

withDefaults(defineProps<{
  title?: string
  bordered?: boolean
  shadow?: boolean
  /** default: 16/24 四周；compact: 12/16；flush: 无内边距 */
  padding?: 'default' | 'compact' | 'flush'
}>(), {
  bordered: true,
  shadow: true,
  padding: 'default',
})

defineOptions({ inheritAttrs: false })
</script>

<style scoped>
.ui-section__body--default { padding: var(--ui-space-4) var(--ui-space-6) var(--ui-space-6); }
.ui-section__body--compact { padding: var(--ui-space-3) var(--ui-space-4); }
.ui-section__body--flush   { padding: 0; }
</style>
