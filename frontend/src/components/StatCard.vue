<template>
  <div class="sc-card">
    <div class="sc-icon-wrap" :style="{ background: iconBg }">
      <component :is="icon" class="sc-icon" :style="{ color: iconColor }" />
    </div>
    <div class="sc-body">
      <div class="sc-value">{{ value }}</div>
      <div class="sc-label">{{ label }}</div>
    </div>
    <div v-if="trend !== undefined" class="sc-trend" :class="trend > 0 ? 'up' : trend < 0 ? 'down' : 'flat'">
      <arrow-up-icon v-if="trend > 0" />
      <arrow-down-icon v-else-if="trend < 0" />
      <span>{{ Math.abs(trend) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { ArrowUpIcon, ArrowDownIcon } from 'tdesign-icons-vue-next'

const props = defineProps<{
  label: string
  value: string | number
  icon: Component
  color?: 'blue' | 'green' | 'orange' | 'red' | 'gray'
  trend?: number
}>()

const colorMap = {
  blue:   { bg: '#e8f0fd', icon: '#0052d9' },
  green:  { bg: '#e8faf4', icon: '#00a870' },
  orange: { bg: '#fdf3e8', icon: '#ed7b2f' },
  red:    { bg: '#fdecea', icon: '#e34d59' },
  gray:   { bg: '#f2f3f5', icon: '#8a94a6' },
}

const c = colorMap[props.color ?? 'blue']
const iconBg    = c.bg
const iconColor = c.icon
</script>

<style scoped>
.sc-card {
  background: var(--sh-card-bg);
  border: var(--sh-card-border);
  border-radius: var(--sh-card-radius);
  box-shadow: var(--sh-card-shadow);
  padding: var(--sh-space-lg);
  display: flex;
  align-items: center;
  gap: var(--sh-space-md);
  transition: box-shadow .2s;
}
.sc-card:hover { box-shadow: var(--sh-card-shadow-hover); }

.sc-icon-wrap {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.sc-icon { font-size: 22px; }

.sc-body { flex: 1; min-width: 0; }
.sc-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--sh-text-primary);
  line-height: 1.1;
}
.sc-label {
  font-size: 13px;
  color: var(--sh-text-secondary);
  margin-top: var(--sh-space-xs);
}

.sc-trend {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: var(--sh-space-xs);
  font-weight: 500;
}
.sc-trend.up   { color: var(--sh-green); }
.sc-trend.down { color: var(--sh-red); }
.sc-trend.flat { color: var(--sh-gray); }
</style>
