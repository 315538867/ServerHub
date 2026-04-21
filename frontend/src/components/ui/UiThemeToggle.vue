<template>
  <button
    class="ui-theme-toggle"
    :title="title"
    :aria-label="title"
    @click="themeStore.toggle()"
  >
    <SunIcon v-if="!themeStore.isDark" :size="16" />
    <MoonIcon v-else :size="16" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Sun as SunIcon, Moon as MoonIcon } from 'lucide-vue-next'
import { useThemeStore } from '@/stores/theme'

const themeStore = useThemeStore()
const title = computed(() => (themeStore.isDark ? '切换为浅色' : '切换为深色'))
</script>

<style scoped>
.ui-theme-toggle {
  width: 32px; height: 32px;
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  background: var(--ui-bg-1);
  color: var(--ui-fg-2);
  cursor: pointer;
  display: inline-flex; align-items: center; justify-content: center;
  transition: all var(--dur-fast) var(--ease);
}
.ui-theme-toggle:hover {
  color: var(--ui-fg);
  border-color: var(--ui-border-strong);
  background: var(--ui-bg-2);
}
.ui-theme-toggle:focus-visible { box-shadow: var(--shadow-ring); }
</style>
