<template>
  <div class="page-container">
    <div class="section-block">
      <div class="section-title">
        <span class="title-text">应用列表</span>
        <t-button theme="primary" size="small" @click="$router.push('/apps/create')">新建应用</t-button>
      </div>
      <div class="table-wrap">
        <t-table
          :data="appStore.apps"
          :columns="columns"
          :loading="appStore.loading"
          row-key="id"
          stripe
          size="small"
        >
          <template #name="{ row }">
            <span class="name-cell">
              <span :class="['status-dot', `status-dot--${row.status}`]" />
              <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
            </span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="statusTheme(row.status)" variant="light" size="small">{{ statusText(row.status) }}</t-tag>
          </template>
          <template #operations="{ row }">
            <t-button size="small" variant="text" theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">查看详情</t-button>
          </template>
        </t-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

const columns = [
  { colKey: 'name', title: '应用名称', minWidth: 160 },
  { colKey: 'domain', title: '域名 / 描述', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '更新时间', minWidth: 160 },
  { colKey: 'operations', title: '操作', width: 100, fixed: 'right' as const },
]

function statusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', error: 'danger', unknown: 'default' } as Record<string, string>)[s] ?? 'default'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? s
}

onMounted(() => appStore.fetch())
</script>

<style scoped>
.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--sh-text-primary);
}
.table-wrap {
  padding: 0 20px 16px;
  font-size: 13px;
}
:deep(.t-table th) {
  background: #FAFAFA;
  font-size: 12px;
  color: var(--sh-text-secondary);
  font-weight: 500;
}
:deep(.t-table td) {
  font-size: 13px;
}
.name-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}
.status-dot--online { background: var(--sh-green); }
.status-dot--offline { background: var(--sh-text-secondary); }
.status-dot--error { background: var(--sh-red); }
.status-dot--unknown { background: var(--sh-text-secondary); }
</style>
