<template>
  <pre
    ref="el"
    class="ui-log"
    :class="[`ui-log--${tone}`, { 'ui-log--bordered': bordered }]"
    :style="style"
  >{{ content }}</pre>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'

const props = withDefaults(defineProps<{
  content: string
  /** dark: 深色终端风；light: 浅色等宽面板 */
  tone?: 'dark' | 'light'
  maxHeight?: string
  minHeight?: string
  autoScroll?: boolean
  bordered?: boolean
}>(), {
  tone: 'dark',
  maxHeight: 'calc(100vh - 240px)',
  minHeight: '200px',
  autoScroll: true,
  bordered: false,
})

const el = ref<HTMLPreElement>()

const style = computed(() => ({
  maxHeight: props.maxHeight,
  minHeight: props.minHeight,
}))

watch(() => props.content, async () => {
  if (!props.autoScroll) return
  await nextTick()
  if (el.value) el.value.scrollTop = el.value.scrollHeight
})
</script>

<style scoped>
.ui-log {
  margin: 0;
  padding: var(--ui-space-4);
  border-radius: var(--ui-radius-md);
  font-family: var(--ui-font-mono);
  font-size: 12.5px;
  line-height: var(--ui-lh-relaxed);
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
.ui-log--dark  { background: #0f1420; color: #e6e6e6; }
.ui-log--light { background: var(--ui-bg-subtle); color: var(--ui-fg-2); }
.ui-log--bordered.ui-log--light { border: 1px solid var(--ui-border); }
</style>
