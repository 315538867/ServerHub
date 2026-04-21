import { createDiscreteApi, darkTheme, type ConfigProviderProps } from 'naive-ui'
import { computed, h } from 'vue'
import { useThemeStore } from '@/stores/theme'

let cached: ReturnType<typeof createDiscreteApi> | null = null

function ensure() {
  if (cached) return cached
  const themeStore = useThemeStore()
  const configProviderPropsRef = computed<ConfigProviderProps>(() => ({
    theme: themeStore.isDark ? darkTheme : null,
  }))
  cached = createDiscreteApi(['message', 'notification', 'dialog'], { configProviderProps: configProviderPropsRef })
  return cached
}

export function $message() { return ensure().message }
export function $notification() { return ensure().notification }
export function $dialog() { return ensure().dialog }

// Avoid unused import warning in some bundlers
void h
