<template>
  <header class="ui-page-header">
    <div class="ui-page-header__top">
      <button v-if="back" class="ui-page-header__back" @click="onBack" aria-label="返回">
        <ArrowLeftIcon :size="14" />
      </button>
      <div class="ui-page-header__main">
        <div class="ui-page-header__breadcrumb" v-if="$slots.breadcrumb">
          <slot name="breadcrumb" />
        </div>
        <div class="ui-page-header__title-row">
          <h1 class="ui-page-header__title">{{ title }}</h1>
          <span v-if="$slots.badge" class="ui-page-header__badge"><slot name="badge" /></span>
        </div>
        <p v-if="description" class="ui-page-header__desc">{{ description }}</p>
        <slot name="desc" />
      </div>
      <div v-if="$slots.actions" class="ui-page-header__actions">
        <slot name="actions" />
      </div>
    </div>
    <div v-if="$slots.tabs" class="ui-page-header__tabs">
      <slot name="tabs" />
    </div>
  </header>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { ArrowLeft as ArrowLeftIcon } from 'lucide-vue-next'

interface Props {
  title: string
  description?: string
  back?: boolean | string
}
const props = defineProps<Props>()
const emit = defineEmits<{ (e: 'back'): void }>()

const router = useRouter()
function onBack() {
  emit('back')
  if (typeof props.back === 'string') router.push(props.back)
  else router.back()
}
</script>

<style scoped>
.ui-page-header {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.ui-page-header__top {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
}
.ui-page-header__back {
  width: 28px; height: 28px;
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-1);
  border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
  display: inline-flex; align-items: center; justify-content: center;
  cursor: pointer;
  margin-top: 2px;
  transition: all var(--dur-fast) var(--ease);
}
.ui-page-header__back:hover { border-color: var(--ui-border-strong); color: var(--ui-fg); }

.ui-page-header__main { flex: 1; min-width: 0; }
.ui-page-header__breadcrumb {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-bottom: var(--space-1);
}
.ui-page-header__title-row {
  display: flex; align-items: center; gap: var(--space-2);
}
.ui-page-header__title {
  font-size: var(--fs-xl);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.015em;
}
.ui-page-header__desc {
  margin-top: var(--space-1);
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
}
.ui-page-header__actions {
  display: flex; align-items: center; gap: var(--space-2);
  flex-shrink: 0;
}
.ui-page-header__tabs { border-bottom: 1px solid var(--ui-border); margin: 0 calc(-1 * var(--space-8)); padding: 0 var(--space-8); }
</style>
