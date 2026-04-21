<template>
  <div class="ui-banner" :class="[`ui-banner--${tone}`]">
    <div class="ui-banner__bg" />
    <div class="ui-banner__main">
      <div class="ui-banner__head">
        <div class="ui-banner__status">
          <StatusDot :status="status" />
          <span class="ui-banner__status-label">{{ statusLabel }}</span>
        </div>
        <div v-if="$slots.meta" class="ui-banner__meta"><slot name="meta" /></div>
      </div>
      <h2 class="ui-banner__title">
        <slot name="title">{{ title }}</slot>
      </h2>
      <div v-if="$slots.subtitle || subtitle" class="ui-banner__subtitle">
        <slot name="subtitle">{{ subtitle }}</slot>
      </div>
    </div>
    <div v-if="$slots.actions" class="ui-banner__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import StatusDot from './StatusDot.vue'

withDefaults(defineProps<{
  title?: string
  subtitle?: string
  status?: 'online' | 'offline' | 'error' | 'unknown'
  statusLabel?: string
  tone?: 'brand' | 'neutral'
}>(), {
  status: 'unknown',
  statusLabel: '',
  tone: 'brand',
})
</script>

<style scoped>
.ui-banner {
  position: relative;
  border-radius: var(--ui-radius-xl);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-surface);
  padding: var(--ui-space-5) var(--ui-space-6);
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: var(--ui-space-5);
  overflow: hidden;
  margin-bottom: var(--ui-space-4);
  animation: ui-slide-up var(--ui-dur-slow) var(--ui-ease-standard);
}

.ui-banner__bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: .9;
}
.ui-banner--brand .ui-banner__bg {
  background:
    radial-gradient(120% 120% at 0% 0%, rgba(94, 106, 210, 0.16) 0%, transparent 50%),
    radial-gradient(80% 80% at 100% 100%, rgba(70, 177, 201, 0.10) 0%, transparent 50%);
}
.ui-banner--brand::after {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--ui-brand-grad);
}

.ui-banner__main { position: relative; min-width: 0; flex: 1; }
.ui-banner__head {
  display: flex;
  align-items: center;
  gap: var(--ui-space-3);
  margin-bottom: var(--ui-space-2);
}
.ui-banner__status {
  display: inline-flex; align-items: center; gap: 6px;
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-2);
  font-weight: var(--ui-fw-medium);
  background: var(--ui-bg-subtle);
  padding: 3px 8px 3px 6px;
  border-radius: var(--ui-radius-pill);
  border: 1px solid var(--ui-border);
}
.ui-banner__meta {
  display: inline-flex; align-items: center; gap: var(--ui-space-2);
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
}
.ui-banner__title {
  margin: 0;
  font-size: var(--ui-fs-3xl);
  font-weight: var(--ui-fw-semibold);
  color: var(--ui-fg);
  letter-spacing: var(--ui-tracking-tight);
  line-height: 1.15;
  display: flex; align-items: center; gap: var(--ui-space-2);
}
.ui-banner__subtitle {
  margin-top: 6px;
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
}

.ui-banner__actions {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  flex-shrink: 0;
}
</style>
