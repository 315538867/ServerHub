<template>
  <button
    class="ui-theme-toggle"
    :title="title"
    :aria-label="title"
    @click="cycle"
  >
    <Transition name="ui-theme-icon" mode="out-in">
      <svg v-if="mode === 'light'" key="sun" width="14" height="14" viewBox="0 0 16 16" fill="none">
        <circle cx="8" cy="8" r="3.2" stroke="currentColor" stroke-width="1.5"/>
        <path d="M8 1.5v1.8M8 12.7v1.8M1.5 8h1.8M12.7 8h1.8M3.35 3.35l1.27 1.27M11.38 11.38l1.27 1.27M3.35 12.65l1.27-1.27M11.38 4.62l1.27-1.27" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
      <svg v-else-if="mode === 'dark'" key="moon" width="14" height="14" viewBox="0 0 16 16" fill="none">
        <path d="M13 9.2A5.2 5.2 0 0 1 6.8 3a5.2 5.2 0 1 0 6.2 6.2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
      </svg>
      <svg v-else key="sys" width="14" height="14" viewBox="0 0 16 16" fill="none">
        <rect x="2" y="3" width="12" height="8.5" rx="1.3" stroke="currentColor" stroke-width="1.5"/>
        <path d="M5.5 13.5h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </Transition>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useThemeStore } from '@/stores/theme'

const store = useThemeStore()
const mode = computed(() => store.mode)
const title = computed(() => ({
  light: '主题：浅色（点击切换）',
  dark:  '主题：深色（点击切换）',
  system:'主题：跟随系统（点击切换）',
}[mode.value]))
function cycle() {
  const next = mode.value === 'light' ? 'dark' : mode.value === 'dark' ? 'system' : 'light'
  store.setMode(next)
}
</script>

<style scoped>
.ui-theme-toggle {
  width: 30px; height: 30px;
  display: inline-flex; align-items: center; justify-content: center;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--ui-radius-md);
  color: var(--ui-fg-2);
  cursor: pointer;
  transition: background-color var(--ui-dur-fast) var(--ui-ease-standard),
              color var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-theme-toggle:hover { background: var(--ui-bg-hover); color: var(--ui-fg); }

.ui-theme-icon-enter-active,
.ui-theme-icon-leave-active {
  transition: transform var(--ui-dur-base) var(--ui-ease-emphasized),
              opacity var(--ui-dur-base) var(--ui-ease-standard);
}
.ui-theme-icon-enter-from { opacity: 0; transform: rotate(-60deg) scale(.6); }
.ui-theme-icon-leave-to   { opacity: 0; transform: rotate(60deg) scale(.6); }
</style>
