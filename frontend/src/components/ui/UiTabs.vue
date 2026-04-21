<template>
  <div class="ui-tabs" :class="[`ui-tabs--${variant}`, `ui-tabs--${size}`]">
    <button
      v-for="(item, idx) in items"
      :key="item.value"
      ref="tabRefs"
      class="ui-tabs__item"
      :class="{ 'is-active': active === item.value }"
      :data-active="active === item.value || undefined"
      @click="select(item.value, idx)"
    >
      <span v-if="item.icon" class="ui-tabs__icon"><component :is="item.icon" /></span>
      <span class="ui-tabs__label">{{ item.label }}</span>
      <span v-if="item.badge !== undefined" class="ui-tabs__badge">{{ item.badge }}</span>
    </button>
    <div
      v-if="variant === 'underline'"
      class="ui-tabs__indicator"
      :style="indicatorStyle"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch, type Component } from 'vue'

interface TabItem {
  value: string
  label: string
  icon?: Component
  badge?: string | number
}

const props = withDefaults(defineProps<{
  items: TabItem[]
  modelValue?: string
  variant?: 'underline' | 'pills'
  size?: 'sm' | 'md'
}>(), {
  variant: 'underline',
  size: 'md',
})

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void; (e: 'change', v: string): void }>()

const active = computed(() => props.modelValue ?? props.items[0]?.value)

const tabRefs = ref<HTMLElement[]>([])
const indicatorStyle = ref<Record<string, string>>({ width: '0px', transform: 'translateX(0)' })

function syncIndicator() {
  if (props.variant !== 'underline') return
  const idx = props.items.findIndex(i => i.value === active.value)
  if (idx < 0) return
  const el = tabRefs.value[idx]
  if (!el) return
  indicatorStyle.value = {
    width: `${el.offsetWidth}px`,
    transform: `translateX(${el.offsetLeft}px)`,
  }
}

function select(value: string, _idx: number) {
  emit('update:modelValue', value)
  emit('change', value)
}

watch(() => active.value, () => nextTick(syncIndicator))
watch(() => props.items.length, () => nextTick(syncIndicator))
onMounted(() => nextTick(syncIndicator))
</script>

<style scoped>
.ui-tabs {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.ui-tabs--underline {
  border-bottom: 1px solid var(--ui-border);
  gap: var(--ui-space-1);
}
.ui-tabs--pills {
  background: var(--ui-bg-subtle);
  padding: 3px;
  border-radius: var(--ui-radius-md);
}

.ui-tabs__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  border: none;
  font-family: inherit;
  color: var(--ui-fg-3);
  cursor: pointer;
  position: relative;
  padding: 0 12px;
  border-radius: var(--ui-radius-sm);
  transition: color var(--ui-dur-fast) var(--ui-ease-standard),
              background-color var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-tabs--sm .ui-tabs__item { height: 28px; font-size: var(--ui-fs-xs); }
.ui-tabs--md .ui-tabs__item { height: 34px; font-size: var(--ui-fs-sm); font-weight: var(--ui-fw-medium); }

.ui-tabs__item:hover { color: var(--ui-fg); }
.ui-tabs__item.is-active { color: var(--ui-fg); }

.ui-tabs--pills .ui-tabs__item.is-active {
  background: var(--ui-bg-surface);
  box-shadow: var(--ui-shadow-xs);
}

.ui-tabs__icon { display: inline-flex; font-size: 14px; }
.ui-tabs__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 16px;
  padding: 0 5px;
  border-radius: var(--ui-radius-pill);
  background: var(--ui-bg-subtle);
  color: var(--ui-fg-3);
  font-size: 10.5px;
  font-weight: var(--ui-fw-medium);
}
.ui-tabs__item.is-active .ui-tabs__badge {
  background: var(--ui-brand-soft);
  color: var(--ui-brand-fg);
}

.ui-tabs__indicator {
  position: absolute;
  left: 0; bottom: -1px;
  height: 2px;
  background: var(--ui-brand);
  border-radius: 2px;
  transition: transform var(--ui-dur-base) var(--ui-ease-emphasized),
              width var(--ui-dur-base) var(--ui-ease-emphasized);
  pointer-events: none;
}
</style>
