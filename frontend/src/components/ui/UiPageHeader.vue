<template>
  <header class="ui-page-header" :class="{ 'ui-page-header--has-back': back }">
    <div class="ui-page-header__main">
      <button v-if="back" class="ui-page-header__back" @click="onBack" :aria-label="'返回'">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
          <path d="M10 12L6 8L10 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
      <div class="ui-page-header__text">
        <div class="ui-page-header__eyebrow" v-if="$slots.eyebrow || eyebrow">
          <slot name="eyebrow">{{ eyebrow }}</slot>
        </div>
        <h1 class="ui-page-header__title">
          <slot name="title">{{ title }}</slot>
          <span v-if="$slots.titleTag" class="ui-page-header__title-tag"><slot name="titleTag" /></span>
        </h1>
        <p class="ui-page-header__subtitle" v-if="$slots.subtitle || subtitle">
          <slot name="subtitle">{{ subtitle }}</slot>
        </p>
      </div>
    </div>
    <div v-if="$slots.actions" class="ui-page-header__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const props = withDefaults(defineProps<{
  title?: string
  subtitle?: string
  eyebrow?: string
  back?: boolean
  backTo?: string
}>(), {
  back: false,
})

const emit = defineEmits<{ (e: 'back'): void }>()
const router = useRouter()
function onBack() {
  emit('back')
  if (props.backTo) { router.push(props.backTo); return }
  if (window.history.length > 1) router.back()
}
</script>

<style scoped>
.ui-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--ui-space-4);
  padding: var(--ui-space-5) 0 var(--ui-space-4);
  margin-bottom: var(--ui-space-2);
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard);
}
.ui-page-header__main {
  display: flex; align-items: flex-start; gap: var(--ui-space-3);
  flex: 1; min-width: 0;
}
.ui-page-header__back {
  width: 28px; height: 28px;
  display: inline-flex; align-items: center; justify-content: center;
  background: var(--ui-bg-surface);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  color: var(--ui-fg-2);
  cursor: pointer;
  margin-top: 2px;
  flex-shrink: 0;
  transition: background-color var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard);
}
.ui-page-header__back:hover { background: var(--ui-bg-hover); transform: translateX(-1px); }

.ui-page-header__text { min-width: 0; flex: 1; }

.ui-page-header__eyebrow {
  font-size: var(--ui-fs-2xs);
  font-weight: var(--ui-fw-medium);
  color: var(--ui-fg-3);
  letter-spacing: var(--ui-tracking-wide);
  text-transform: uppercase;
  margin-bottom: 4px;
}

.ui-page-header__title {
  margin: 0;
  font-size: var(--ui-fs-3xl);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  letter-spacing: var(--ui-tracking-tight);
  line-height: var(--ui-lh-tight);
  display: flex; align-items: center; gap: var(--ui-space-2);
}
.ui-page-header__title-tag { display: inline-flex; }

.ui-page-header__subtitle {
  margin: 4px 0 0;
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
  line-height: var(--ui-lh-normal);
  max-width: 640px;
}

.ui-page-header__actions {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  flex-shrink: 0;
}
</style>
