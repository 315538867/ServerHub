<template>
  <div class="ph-wrap">
    <div class="ph-left">
      <h1 class="ph-title">{{ title }}</h1>
      <p v-if="subtitle" class="ph-subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="ph-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<style scoped>
.ph-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 16px;
  background: var(--sh-card-bg);
  border-bottom: 1px solid var(--sh-border);
  margin-bottom: 0;
}
.ph-left { min-width: 0; }
.ph-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--sh-text-primary);
  line-height: 1.3;
}
.ph-subtitle {
  margin: 3px 0 0;
  font-size: 13px;
  color: var(--sh-text-secondary);
  line-height: 1.5;
}
.ph-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
</style>
