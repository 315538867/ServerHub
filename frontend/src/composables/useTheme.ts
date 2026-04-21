import { storeToRefs } from 'pinia'
import { useThemeStore } from '@/stores/theme'

// Usage: const { mode, resolved, isDark, setMode, toggle } = useTheme()
export function useTheme() {
  const store = useThemeStore()
  const { mode, system } = storeToRefs(store)
  return {
    mode,
    system,
    resolved: storeToRefs(store).resolved as unknown as { value: 'light' | 'dark' },
    isDark: storeToRefs(store).isDark as unknown as { value: boolean },
    setMode: store.setMode,
    toggle: store.toggle,
  }
}
