<template>
  <div class="page-container">
    <div class="section-block">
      <div class="section-title">
        <span class="title-text">应用列表</span>
        <t-space size="small">
          <t-button v-if="selected.length" theme="danger" variant="outline" size="small" @click="batchDelete">
            批量删除 ({{ selected.length }})
          </t-button>
          <t-button theme="primary" size="small" @click="$router.push('/apps/create')">新建应用</t-button>
        </t-space>
      </div>

      <!-- 搜索 + 过滤工具栏 -->
      <div class="filter-bar">
        <t-input
          v-model="keyword"
          placeholder="搜索名称 / 域名 / 容器 / 描述…"
          size="small"
          clearable
          class="search-box"
        >
          <template #prefix-icon><t-icon name="search" /></template>
        </t-input>

        <t-select
          v-model="filterStatus"
          :options="statusOptions"
          size="small"
          placeholder="所有状态"
          clearable
          class="filter-sel"
        />

        <t-select
          v-model="filterServer"
          :options="serverOptions"
          size="small"
          placeholder="所有服务器"
          clearable
          class="filter-sel"
        />

        <t-radio-group v-model="groupBy" variant="default-filled" size="small">
          <t-radio-button value="none">不分组</t-radio-button>
          <t-radio-button value="server">按服务器</t-radio-button>
          <t-radio-button value="status">按状态</t-radio-button>
        </t-radio-group>

        <span class="filter-summary">{{ filtered.length }} / {{ appStore.apps.length }}</span>
      </div>

      <!-- 不分组：单表 -->
      <div v-if="groupBy === 'none'" class="table-wrap">
        <t-table
          :data="filtered"
          :columns="columns"
          :loading="appStore.loading"
          :selected-row-keys="selected"
          select-on-row-click
          @select-change="onSelectChange"
          row-key="id"
          stripe
          size="small"
          :empty="emptyText"
        >
          <template #name="{ row }">
            <span class="name-cell">
              <span :class="['status-dot', row.status]" />
              <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
            </span>
          </template>
          <template #server="{ row }">
            <span class="server-cell">{{ serverName(row.server_id) }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="statusTheme(row.status)" variant="light" size="small">{{ statusText(row.status) }}</t-tag>
          </template>
          <template #operations="{ row }">
            <t-button size="small" variant="text" theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">查看</t-button>
            <t-button size="small" variant="text" @click="$router.push(`/apps/${row.id}/deploy`)">部署</t-button>
          </template>
        </t-table>
      </div>

      <!-- 分组：多表 -->
      <div v-else class="grouped-wrap">
        <div v-if="filtered.length === 0" class="grouped-empty">{{ emptyText }}</div>
        <div v-for="g in grouped" :key="g.key" class="group-block">
          <div class="group-head">
            <span class="group-title">{{ g.label }}</span>
            <span class="group-count">{{ g.items.length }}</span>
          </div>
          <t-table
            :data="g.items"
            :columns="groupColumns"
            row-key="id"
            stripe
            size="small"
          >
            <template #name="{ row }">
              <span class="name-cell">
                <span :class="['status-dot', row.status]" />
                <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
              </span>
            </template>
            <template #status="{ row }">
              <t-tag :theme="statusTheme(row.status)" variant="light" size="small">{{ statusText(row.status) }}</t-tag>
            </template>
            <template #operations="{ row }">
              <t-button size="small" variant="text" theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">查看</t-button>
              <t-button size="small" variant="text" @click="$router.push(`/apps/${row.id}/deploy`)">部署</t-button>
            </template>
          </t-table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp } from '@/api/application'
import type { Application } from '@/types/api'

const appStore = useAppStore()
const serverStore = useServerStore()

const keyword = ref('')
const filterStatus = ref<string>('')
const filterServer = ref<number | ''>('')
const groupBy = ref<'none' | 'server' | 'status'>('none')
const selected = ref<Array<string | number>>([])

const statusOptions = [
  { label: '🟢 在线', value: 'online' },
  { label: '🔴 离线', value: 'offline' },
  { label: '⚠️ 错误', value: 'error' },
  { label: '⚪ 未知', value: 'unknown' },
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
    let key: string
    let label: string
    if (groupBy.value === 'server') {
      key = String(a.server_id)
      label = serverName(a.server_id)
    } else if (groupBy.value === 'status') {
      key = a.status || 'unknown'
      label = statusText(key)
    } else {
      continue
    }
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

const columns = [
  { colKey: 'row-select', type: 'multiple' as const, width: 40 },
  { colKey: 'name', title: '应用名称', minWidth: 160 },
  { colKey: 'server', title: '服务器', minWidth: 130 },
  { colKey: 'domain', title: '域名 / 描述', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '更新时间', minWidth: 160 },
  { colKey: 'operations', title: '操作', width: 130, fixed: 'right' as const },
]

const groupColumns = [
  { colKey: 'name', title: '应用名称', minWidth: 160 },
  { colKey: 'domain', title: '域名 / 描述', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'updated_at', title: '更新时间', minWidth: 160 },
  { colKey: 'operations', title: '操作', width: 130, fixed: 'right' as const },
]

function statusTheme(s: string) {
  return ({ online: 'success', offline: 'danger', error: 'danger', unknown: 'default' } as Record<string, string>)[s] ?? 'default'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s] ?? s
}
function serverName(sid: number) {
  return serverStore.getById(sid)?.name || `服务器 #${sid}`
}

function onSelectChange(value: Array<string | number>) {
  selected.value = value
}

function batchDelete() {
  const ids = [...selected.value] as number[]
  if (ids.length === 0) return
  const dlg = DialogPlugin.confirm({
    header: '批量删除',
    body: `确认删除选中的 ${ids.length} 个应用？此操作不可恢复。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      dlg.hide()
      let ok = 0, fail = 0
      for (const id of ids) {
        try { await deleteApp(id); ok++ } catch { fail++ }
      }
      MessagePlugin.success(`已删除 ${ok} 个${fail ? `，失败 ${fail} 个` : ''}`)
      selected.value = []
      await appStore.fetch()
    },
  })
}

onMounted(async () => {
  await Promise.all([
    appStore.fetch(),
    serverStore.servers.length ? Promise.resolve() : serverStore.fetch(),
  ])
})
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-sm) var(--sh-space-lg) var(--sh-space-md);
  flex-wrap: wrap;
  border-bottom: 1px solid var(--sh-border);
}
.search-box { width: 280px; max-width: 100%; }
.filter-sel { width: 160px; }
.filter-summary {
  margin-left: auto;
  font-size: 12px;
  color: var(--sh-text-secondary);
  font-variant-numeric: tabular-nums;
}
.table-wrap {
  padding: 0 var(--sh-space-lg) var(--sh-space-md);
  font-size: 13px;
}
.grouped-wrap {
  padding: var(--sh-space-xs) var(--sh-space-lg) var(--sh-space-md);
  display: flex;
  flex-direction: column;
  gap: var(--sh-space-md);
}
.grouped-empty {
  text-align: center;
  padding: var(--sh-space-xl) 0;
  color: var(--sh-text-secondary);
  font-size: 13px;
}
.group-block {
  border: 1px solid var(--sh-border);
  border-radius: 8px;
  overflow: hidden;
}
.group-head {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-sm) var(--sh-space-md);
  background: color-mix(in srgb, var(--sh-text-primary) 4%, transparent);
  font-size: 13px;
}
.group-title { font-weight: 600; color: var(--sh-text-primary); }
.group-count {
  margin-left: auto;
  font-size: 11px;
  color: var(--sh-text-secondary);
  background: var(--sh-card-bg);
  padding: 1px 8px;
  border-radius: 10px;
}
.name-cell { display: inline-flex; align-items: center; gap: var(--sh-space-sm); }
.server-cell { font-size: 12px; color: var(--sh-text-secondary); }
</style>
