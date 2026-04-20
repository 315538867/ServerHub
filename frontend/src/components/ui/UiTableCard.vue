<template>
  <UiCard :title="title" :padded="false">
    <template v-if="$slots.extra" #extra>
      <slot name="extra" />
    </template>

    <div v-if="$slots.toolbar" class="ui-tc__toolbar">
      <slot name="toolbar" />
    </div>

    <div class="ui-tc__table">
      <t-table
        v-bind="tableAttrs"
        :data="data"
        :columns="columns"
        :loading="loading"
        :row-key="rowKey"
        :size="size"
        :bordered="bordered"
        :stripe="stripe"
        :empty="empty"
      >
        <template v-for="name in passthroughSlots" #[name]="scope" :key="name">
          <slot :name="name" v-bind="scope" />
        </template>
      </t-table>
    </div>

    <div v-if="$slots.pagination" class="ui-tc__pagination">
      <slot name="pagination" />
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { computed, useSlots } from 'vue'
import UiCard from './UiCard.vue'

const props = withDefaults(defineProps<{
  title?: string
  data: readonly any[]
  columns: readonly any[]
  loading?: boolean
  rowKey?: string
  size?: 'small' | 'medium' | 'large'
  bordered?: boolean
  stripe?: boolean
  empty?: string
  tableAttrs?: Record<string, any>
}>(), {
  size: 'small',
  bordered: false,
  stripe: false,
  rowKey: 'id',
  empty: '暂无数据',
  tableAttrs: () => ({}),
})

const slots = useSlots()

// 所有 "cell-*" slot（形如 #operations / #status）都透传给 t-table
const passthroughSlots = computed(() =>
  Object.keys(slots).filter((n) => n !== 'toolbar' && n !== 'extra' && n !== 'pagination' && n !== 'default'),
)

// 让 v-bind="tableAttrs" 可转发任意额外 t-table 属性
const tableAttrs = computed(() => props.tableAttrs)
</script>

<style scoped>
.ui-tc__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--ui-space-4);
  padding: var(--ui-space-3) var(--ui-space-6);
  border-bottom: 1px solid var(--ui-border-subtle);
  background: var(--ui-bg-elevated);
}
.ui-tc__table {
  padding: var(--ui-space-4) var(--ui-space-6) var(--ui-space-6);
}
.ui-tc__pagination {
  display: flex;
  justify-content: flex-end;
  padding: 0 var(--ui-space-6) var(--ui-space-4);
}
</style>
