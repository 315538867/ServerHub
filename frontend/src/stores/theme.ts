import { defineStore } from 'pinia'

// Usage: import and call useThemeStore(); mode = 'light' | 'dark' | 'system'.
// Persists to localStorage under `serverhub.theme`. Applies [data-theme] on <html>.

export type ThemeMode = 'light' | 'dark' | 'system'
export type ThemeResolved = 'light' | 'dark'

const STORAGE_KEY = 'serverhub.theme'

function systemPrefers(): ThemeResolved {
  if (typeof window === 'undefined' || !window.matchMedia) return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function readStored(): ThemeMode {
  if (typeof localStorage === 'undefined') return 'system'
  const v = localStorage.getItem(STORAGE_KEY) as ThemeMode | null
  if (v === 'light' || v === 'dark' || v === 'system') return v
  return 'system'
}

function applyDom(resolved: ThemeResolved) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.dataset.theme = resolved
  // TDesign Vue Next dark mode trigger (required for .t-input/.t-select/.t-dialog etc.)
  if (resolved === 'dark') root.setAttribute('theme-mode', 'dark')
  else root.removeAttribute('theme-mode')
  root.style.colorScheme = resolved
}

export const useThemeStore = defineStore('theme', {
  state: () => ({
    mode: readStored() as ThemeMode,
    system: systemPrefers() as ThemeResolved,
  }),
  getters: {
    resolved(state): ThemeResolved {
      return state.mode === 'system' ? state.system : state.mode
    },
    isDark(): boolean { return this.resolved === 'dark' },
  },
  actions: {
    setMode(mode: ThemeMode) {
      this.mode = mode
      if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, mode)
      applyDom(this.resolved)
    },
    toggle() {
      const next: ThemeMode = this.resolved === 'dark' ? 'light' : 'dark'
      this.setMode(next)
    },
    init() {
      applyDom(this.resolved)
      if (typeof window !== 'undefined' && window.matchMedia) {
        const mq = window.matchMedia('(prefers-color-scheme: dark)')
        const onChange = (e: MediaQueryListEvent) => {
          this.system = e.matches ? 'dark' : 'light'
          if (this.mode === 'system') applyDom(this.resolved)
        }
        if (mq.addEventListener) mq.addEventListener('change', onChange)
        else mq.addListener(onChange)
      }
    },
  },
})
