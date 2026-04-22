<template>
  <div class="page apps-list">
    <UiPageHeader title="应用列表" subtitle="管理所有部署在你服务器上的应用">
      <template #actions>
        <UiButton v-if="selected.length" variant="danger" size="sm" @click="batchDelete">
          删除 {{ selected.length }} 项
        </UiButton>
        <UiButton variant="primary" size="sm" @click="$router.push('/apps/create')">
          <template #icon><Plus :size="14" /></template>
          新建应用
        </UiButton>
      </template>
    </UiPageHeader>

    <UiCard padding="none">
      <div class="filter-bar">
        <NInput
          v-model:value="keyword"
          placeholder="搜索名称 / 域名 / 容器 / 描述…"
          clearable
          size="small"
          class="search-box"
        >
          <template #prefix><Search :size="14" /></template>
        </NInput>
        <NSelect
          v-model:value="filterStatus"
          :options="statusOptions"
          placeholder="所有状态"
          clearable
          size="small"
          class="filter-sel"
        />
        <NSelect
          v-model:value="filterServer"
          :options="serverOptions"
          placeholder="所有服务器"
          clearable
          size="small"
          class="filter-sel"
        />
        <NRadioGroup v-model:value="groupBy" size="small">
          <NRadioButton value="none">不分组</NRadioButton>
          <NRadioButton value="server">按服务器</NRadioButton>
          <NRadioButton value="status">按状态</NRadioButton>
        </NRadioGroup>
        <span class="filter-summary">
          <UiBadge tone="brand">{{ filtered.length }}</UiBadge>
          <span class="filter-summary__total">/ {{ appStore.apps.length }}</span>
        </span>
      </div>

      <div v-if="groupBy === 'none'">
        <NDataTable
          :columns="columns"
          :data="filtered"
          :loading="appStore.loading"
          :row-key="(row: Application) => row.id"
          v-model:checked-row-keys="selected"
          size="small"
          :bordered="false"
        />
      </div>

      <div v-else class="grouped-wrap">
        <EmptyBlock v-if="filtered.length === 0" :description="emptyText" />
        <div v-for="g in grouped" :key="g.key" class="group-block">
          <div class="group-head">
            <span class="group-title">{{ g.label }}</span>
            <UiBadge tone="neutral">{{ g.items.length }}</UiBadge>
          </div>
          <NDataTable
            :columns="groupColumns"
            :data="g.items"
            :row-key="(row: Application) => row.id"
            size="small"
            :bordered="false"
          />
        </div>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NInput, NSelect, NRadioGroup, NRadioButton, NDataTable,
  useMessage, useDialog,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { Plus, Search } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp } from '@/api/application'
import type { Application } from '@/types/api'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const appStore = useAppStore()
const serverStore = useServerStore()

const keyword = ref('')
const filterStatus = ref<string | null>(null)
const filterServer = ref<number | null>(null)
const groupBy = ref<'none' | 'server' | 'status'>('none')
const selected = ref<Array<string | number>>([])

const statusOptions = [
  { label: '在线', value: 'online' },
  { label: '离线', value: 'offline' },
  { label: '错误', value: 'error' },
  { label: '未知', value: 'unknown' },
]
const serverOptions = computed(() =>
  serverStore.servers.map(s => ({ label: s.name, value: s.id }))
)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return appStore.apps.filter((a: Application) => {
    if (filterStatus.value && a.status !== filterStatus.value) return false
    if (filterServer.value && a.server_id !== filterServer.value) return false
    if (kw) {
      const hay = [a.name, a.domain, a.container_name, a.description, a.site_name].filter(Boolean).join(' ').toLowerCase()
      if (!hay.includes(kw)) return false
    }
    return true
  })
})

interface Group { key: string; label: string; items: Application[] }

const grouped = computed<Group[]>(() => {
  const list = filtered.value
  const map = new Map<string, Group>()
  for (const a of list) {
    let key: string, label: string
    if (groupBy.value === 'server') {
      key = String(a.server_id); label = serverName(a.server_id)
    } else if (groupBy.value === 'status') {
      key = a.status || 'unknown'; label = statusText(key)
    } else continue
    let g = map.get(key)
    if (!g) { g = { key, label, items: [] }; map.set(key, g) }
    g.items.push(a)
  }
  return Array.from(map.values()).sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'))
})

const emptyText = computed(() => {
  if (appStore.apps.length === 0) return '暂无应用，点击右上角「新建应用」创建第一个'
  if (keyword.value || filterStatus.value || filterServer.value) return '没有匹配的应用，调整搜索或过滤条件'
  return '暂无数据'
})

function statusTone(s: string): any {
  return ({ online: 'success', offline: 'danger', error: 'danger' } as Record<string, string>)[s] ?? 'neutral'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? s
}
function serverName(sid: number) {
  return serverStore.getById(sid)?.name || `服务器 #${sid}`
}

function renderName(row: Application) {
  return h('div', { class: 'name-cell' }, [
    h(StatusDot, { status: row.status, size: 8 }),
    h('a', {
      class: 'name-link',
      onClick: (e: Event) => { e.preventDefault(); router.push(`/apps/${row.id}/overview`) },
      href: `/apps/${row.id}/overview`,
    }, row.name),
  ])
}
function renderOps(row: Application) {
  return h('div', { class: 'cell-ops' }, [
    h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => router.push(`/apps/${row.id}/overview`) }, () => '查看'),
    h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => router.push(`/apps/${row.id}/deploy`) }, () => '部署'),
  ])
}

const columns = computed<DataTableColumns<Application>>(() => [
  { type: 'selection', width: 40 },
  { title: '应用名称', key: 'name', minWidth: 180, render: renderName },
  { title: '服务器', key: 'server', minWidth: 130, render: (row) => h('span', { class: 'muted-cell' }, serverName(row.server_id)) },
  { title: '域名 / 描述', key: 'domain', minWidth: 200, render: (row) => row.domain || row.description || '—' },
  { title: '状态', key: 'status', width: 100, render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => statusText(row.status)) },
  { title: '更新时间', key: 'updated_at', minWidth: 160, render: (row) => h('span', { class: 'time-cell' }, row.updated_at) },
  { title: '操作', key: 'operations', width: 160, fixed: 'right' as const, render: renderOps },
])

const groupColumns = computed<DataTableColumns<Application>>(() => [
  { title: '应用名称', key: 'name', minWidth: 180, render: renderName },
  { title: '域名 / 描述', key: 'domain', minWidth: 200, render: (row) => row.domain || row.description || '—' },
  { title: '状态', key: 'status', width: 100, render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => statusText(row.status)) },
  { title: '更新时间', key: 'updated_at', minWidth: 160, render: (row) => h('span', { class: 'time-cell' }, row.updated_at) },
  { title: '操作', key: 'operations', width: 160, fixed: 'right' as const, render: renderOps },
])

function batchDelete() {
  const ids = [...selected.value] as number[]
  if (ids.length === 0) return
  dialog.warning({
    title: '批量删除',
    content: `确认删除选中的 ${ids.length} 个应用？此操作不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      let ok = 0, fail = 0
      for (const id of ids) {
        try { await deleteApp(id); ok++ } catch { fail++ }
      }
      message.success(`已删除 ${ok} 个${fail ? `，失败 ${fail} 个` : ''}`)
      selected.value = []
      await appStore.fetch()
    },
  })
}

onMounted(async () => {
  await Promise.all([
    appStore.ensure(),
    serverStore.ensure(),
  ])
})
</script>

<style scoped>
.apps-list {
  padding: var(--space-6);
  display: flex; flex-direction: column;
  gap: var(--space-4);
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  flex-wrap: wrap;
  border-bottom: 1px solid var(--ui-border);
  background: var(--ui-bg);
}
.search-box { width: 280px; max-width: 100%; }
.filter-sel { width: 160px; }
.filter-summary {
  margin-left: auto;
  display: inline-flex; align-items: center; gap: var(--space-1);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  font-variant-numeric: tabular-nums;
}
.filter-summary__total { color: var(--ui-fg-4); }

.grouped-wrap {
  padding: var(--space-3) var(--space-4);
  display: flex; flex-direction: column;
  gap: var(--space-3);
}
.group-block {
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.group-head {
  display: flex; align-items: center; gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--ui-bg-2);
  border-bottom: 1px solid var(--ui-border);
  font-size: var(--fs-sm);
}
.group-title { font-weight: var(--fw-semibold); color: var(--ui-fg); flex: 1; }

:deep(.name-cell) { display: inline-flex; align-items: center; gap: var(--space-2); }
:deep(.name-link) {
  color: var(--ui-brand-fg);
  text-decoration: none;
  font-weight: var(--fw-medium);
}
:deep(.name-link:hover) { color: var(--ui-brand); text-decoration: underline; }
:deep(.muted-cell) { color: var(--ui-fg-3); font-size: var(--fs-xs); }
:deep(.time-cell) { color: var(--ui-fg-3); font-size: var(--fs-xs); font-variant-numeric: tabular-nums; }
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); align-items: center; }
</style>
