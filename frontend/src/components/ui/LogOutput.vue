<template>
  <pre
    ref="el"
    class="ui-log"
    :class="[`ui-log--${tone}`, { 'ui-log--bordered': bordered }]"
  ><slot /></pre>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

interface Props {
  tone?: 'dark' | 'neutral'
  bordered?: boolean
  autoScroll?: boolean
  content?: string
}
const props = withDefaults(defineProps<Props>(), {
  tone: 'dark',
  bordered: true,
  autoScroll: true,
})

const el = ref<HTMLElement | null>(null)
watch(() => props.content, async () => {
  if (!props.autoScroll) return
  await nextTick()
  if (el.value) el.value.scrollTop = el.value.scrollHeight
})
</script>

<style scoped>
.ui-log {
  margin: 0;
  padding: var(--space-4);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: var(--lh-relaxed);
  white-space: pre-wrap;
  word-break: break-all;
  border-radius: var(--radius-md);
  overflow-y: auto;
  min-height: 200px;
  max-height: calc(100vh - 240px);
}
.ui-log--dark {
  background: var(--ui-code-bg);
  color: var(--ui-code-fg);
}
.ui-log--neutral {
  background: var(--ui-bg-2);
  color: var(--ui-fg-2);
}
.ui-log--bordered { border: 1px solid var(--ui-border); }
</style>
