<template>
  <div v-if="hasContent" class="diff-preview">
    <div v-if="applyResult" class="diff-preview__head">
      <UiBadge :tone="applyResult.rolled_back ? 'danger' : (applyResult.no_op ? 'neutral' : 'success')">
        {{ applyResult.rolled_back ? '已回滚' : (applyResult.no_op ? '无变更' : '已应用') }}
      </UiBadge>
      <span class="diff-preview__muted">audit #{{ applyResult.audit_id }}</span>
    </div>
    <div v-if="changes.length" class="diff-preview__list">
      <div v-for="c in changes" :key="c.path" class="diff-preview__row" :data-kind="c.kind">
        <span class="diff-preview__sign">{{ kindSign(c.kind) }}</span>
        <code class="diff-preview__path">{{ c.path }}</code>
        <span v-if="c.kind === 'update'" class="diff-preview__hash">
          {{ (c.old_hash ?? '').slice(0, 8) }} → {{ (c.new_hash ?? '').slice(0, 8) }}
        </span>
        <span v-else-if="c.kind === 'add'" class="diff-preview__hash">
          {{ (c.new_hash ?? '').slice(0, 8) }}
        </span>
      </div>
    </div>
    <LogOutput
      v-if="showOutput && applyResult?.output"
      :content="applyResult.output"
      tone="dark"
      class="diff-preview__log"
    />
  </div>
</template>

<script setup lang="ts">
// DiffPreview 把 Reconciler 的 changeset + ApplyResult 头 + nginx -t 输出
// 包成一个无业务依赖的纯展示组件,供 ServerDetail/Ingresses 与顶级 Ingresses
// 管理页复用。不发事件,不持本地状态 — 父组件持有 changes 和 applyResult。
import { computed } from 'vue'
import type { ChangeKind, IngressChange, ApplyResult } from '@/api/ingresses'
import UiBadge from '@/components/ui/UiBadge.vue'
import LogOutput from '@/components/ui/LogOutput.vue'

const props = withDefaults(defineProps<{
  changes: IngressChange[]
  applyResult?: ApplyResult | null
  showOutput?: boolean
}>(), { applyResult: null, showOutput: true })

const hasContent = computed(() => props.changes.length > 0 || !!props.applyResult)

function kindSign(k: ChangeKind): string {
  return k === 'add' ? '+' : k === 'delete' ? '-' : '~'
}
</script>

<style scoped>
.diff-preview { display: flex; flex-direction: column; gap: var(--space-2); }
.diff-preview__head { display: flex; align-items: center; gap: var(--space-2); }
.diff-preview__muted { color: var(--ui-fg-3); font-size: var(--fs-xs); }
.diff-preview__list { display: flex; flex-direction: column; gap: 4px; font-size: var(--fs-sm); }
.diff-preview__row { display: flex; align-items: center; gap: var(--space-2); }
.diff-preview__row[data-kind="add"] .diff-preview__sign { color: var(--ui-success-fg); }
.diff-preview__row[data-kind="delete"] .diff-preview__sign { color: var(--ui-danger-fg); }
.diff-preview__row[data-kind="update"] .diff-preview__sign { color: var(--ui-brand); }
.diff-preview__sign { font-family: var(--font-mono); font-weight: var(--fw-semibold); width: 12px; }
.diff-preview__path { font-family: var(--font-mono); }
.diff-preview__hash { font-family: var(--font-mono); font-size: var(--fs-xs); color: var(--ui-fg-4); }
.diff-preview__log { margin-top: var(--space-1); }
</style>
