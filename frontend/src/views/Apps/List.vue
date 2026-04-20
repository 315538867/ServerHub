<template>
  <div class="app-list">
    <div class="page-header">
      <h2 class="page-title">应用列表</h2>
      <t-button theme="primary" @click="$router.push('/apps/create')">新建应用</t-button>
    </div>
    <t-table :data="appStore.apps" :columns="columns" :loading="appStore.loading" row-key="id" stripe>
      <template #name="{ row }">
        <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row.status)" variant="light" size="small">{{ statusText(row.status) }}</t-tag>
      </template>
    </t-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

const columns = [
  { colKey: 'name', title: '应用名称', minWidth: 160 },
  { colKey: 'domain', title: '域名', minWidth: 160 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '更新时间', minWidth: 160 },
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
.app-list {}
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-title { margin: 0; font-size: 18px; font-weight: 600; color: var(--td-text-color-primary); }
</style>
