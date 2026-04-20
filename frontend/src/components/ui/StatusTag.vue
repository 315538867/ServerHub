<template>
  <t-tag
    :theme="theme"
    variant="light"
    :size="size"
    class="ui-status-tag"
  >
    <StatusDot v-if="dot" :status="status" :size="6" class="ui-status-tag__dot" />
    <slot>{{ label }}</slot>
  </t-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import StatusDot, { type UiStatus } from './StatusDot.vue'

const props = withDefaults(defineProps<{
  status?: UiStatus | string
  /** 自动根据 status 推导默认文案 */
  label?: string
  /** 是否在 tag 内显示状态点 */
  dot?: boolean
  size?: 'small' | 'medium' | 'large'
  /** 自定义文案映射 */
  labelMap?: Record<string, string>
}>(), {
  status: 'unknown',
  dot: false,
  size: 'small',
})

const DEFAULT_LABELS: Record<string, string> = {
  online: '在线', success: '成功', running: '运行中', ok: '正常',
  offline: '离线', stopped: '已停止',
  error: '错误', failed: '失败', danger: '异常',
  warning: '警告', pending: '等待中', deploying: '部署中',
  unknown: '未知',
}

const theme = computed<'success' | 'danger' | 'warning' | 'default'>(() => {
  const s = (props.status || '').toLowerCase()
  if (['online', 'success', 'running', 'ok'].includes(s)) return 'success'
  if (['error', 'failed', 'danger', 'offline'].includes(s)) return 'danger'
  if (['warning', 'pending', 'deploying'].includes(s)) return 'warning'
  return 'default'
})

const label = computed(() => {
  if (props.label) return props.label
  const s = (props.status || '').toLowerCase()
  return props.labelMap?.[s] ?? DEFAULT_LABELS[s] ?? props.status
})
</script>

<script lang="ts">
export type { UiStatus } from './StatusDot.vue'
</script>

<style scoped>
.ui-status-tag__dot { margin-right: 4px; }
</style>
