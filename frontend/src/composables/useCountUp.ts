import { ref, watch, type Ref } from 'vue'

// Usage:
//   const value = useCountUp(() => metric.value, { duration: 600 })
//   <span>{{ value }}</span>
//
// Requests: animate from previous display value to the current target
// using requestAnimationFrame. Keeps integers integer; respects prefers-reduced-motion.

interface Options {
  duration?: number
  decimals?: number
  ease?: (t: number) => number
}

const easeOutCubic = (t: number) => 1 - Math.pow(1 - t, 3)

function prefersReduced(): boolean {
  return typeof window !== 'undefined'
    && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
}

export function useCountUp(target: () => number, opts: Options = {}): Ref<number> {
  const duration = opts.duration ?? 600
  const decimals = opts.decimals ?? 0
  const ease = opts.ease ?? easeOutCubic
  const display = ref(target())
  let raf = 0
  let start = 0
  let from = display.value

  const step = (now: number) => {
    if (!start) start = now
    const progress = Math.min(1, (now - start) / duration)
    const eased = ease(progress)
    const current = from + (target() - from) * eased
    display.value = decimals > 0 ? Number(current.toFixed(decimals)) : Math.round(current)
    if (progress < 1) raf = requestAnimationFrame(step)
  }

  watch(target, () => {
    if (prefersReduced()) { display.value = target(); return }
    cancelAnimationFrame(raf)
    start = 0
    from = display.value
    raf = requestAnimationFrame(step)
  })

  return display
}
