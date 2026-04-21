<template>
  <div class="page-container apps-list">
    <UiPageHeader title="应用列表" subtitle="管理所有部署在你服务器上的应用">
      <template #actions>
        <UiButton v-if="selected.length" variant="danger" size="sm" @click="batchDelete">
          删除 {{ selected.length }} 项
        </UiButton>
        <UiButton variant="primary" size="sm" @click="$router.push('/apps/create')">
          新建应用
        </UiButton>
      </template>
    </UiPageHeader>

    <UiSection padding="flush">
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

        <span class="filter-summary">
          <UiBadge tone="brand" variant="soft">{{ filtered.length }}</UiBadge>
          / {{ appStore.apps.length }}
        </span>
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
              <StatusDot :status="row.status" :size="8" />
              <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
            </span>
          </template>
          <template #server="{ row }">
            <span class="server-cell">{{ serverName(row.server_id) }}</span>
          </template>
          <template #status="{ row }">
            <UiBadge :tone="statusTone(row.status)" variant="soft">{{ statusText(row.status) }}</UiBadge>
          </template>
          <template #operations="{ row }">
            <t-button size="small" variant="text" theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">查看</t-button>
            <t-button size="small" variant="text" @click="$router.push(`/apps/${row.id}/deploy`)">部署</t-button>
          </template>
        </t-table>
      </div>

      <!-- 分组：多表 -->
      <div v-else class="grouped-wrap">
        <EmptyBlock v-if="filtered.length === 0" :description="emptyText" />
        <div v-for="(g, gi) in grouped" :key="g.key" class="group-block" :style="{ animationDelay: `${gi * 60}ms` }">
          <div class="group-head">
            <span class="group-title">{{ g.label }}</span>
            <UiBadge tone="neutral" variant="soft">{{ g.items.length }}</UiBadge>
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
                <StatusDot :status="row.status" :size="8" />
                <t-link theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">{{ row.name }}</t-link>
              </span>
            </template>
            <template #status="{ row }">
              <UiBadge :tone="statusTone(row.status)" variant="soft">{{ statusText(row.status) }}</UiBadge>
            </template>
            <template #operations="{ row }">
              <t-button size="small" variant="text" theme="primary" @click="$router.push(`/apps/${row.id}/overview`)">查看</t-button>
              <t-button size="small" variant="text" @click="$router.push(`/apps/${row.id}/deploy`)">部署</t-button>
            </template>
          </t-table>
        </div>
      </div>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp } from '@/api/application'
import type { Application } from '@/types/api'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSection from '@/components/ui/UiSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

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

function statusTone(s: string): any {
  return ({ online: 'success', offline: 'danger', error: 'danger' } as Record<string, string>)[s] ?? 'neutral'
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
.apps-list { padding: var(--ui-space-4) var(--ui-space-5); }

.filter-bar {
  display: flex;
  align-items: center;
  gap: var(--ui-space-2);
  padding: var(--ui-space-3) var(--ui-space-5);
  flex-wrap: wrap;
  border-bottom: 1px solid var(--ui-border-subtle);
  background: var(--ui-bg-subtle);
}
.search-box { width: 280px; max-width: 100%; }
.filter-sel { width: 160px; }
.filter-summary {
  margin-left: auto;
  display: inline-flex; align-items: center; gap: 4px;
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
  font-variant-numeric: tabular-nums;
}
.table-wrap {
  padding: 0 var(--ui-space-5) var(--ui-space-3);
  font-size: var(--ui-fs-sm);
}
.grouped-wrap {
  padding: var(--ui-space-3) var(--ui-space-5);
  display: flex; flex-direction: column;
  gap: var(--ui-space-3);
}
.group-block {
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  overflow: hidden;
  opacity: 0;
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard) forwards;
}
.group-head {
  display: flex; align-items: center; gap: var(--ui-space-2);
  padding: var(--ui-space-2) var(--ui-space-4);
  background: var(--ui-bg-subtle);
  border-bottom: 1px solid var(--ui-border-subtle);
  font-size: var(--ui-fs-sm);
}
.group-title { font-weight: var(--ui-fw-semibold); color: var(--ui-fg); flex: 1; }

.name-cell { display: inline-flex; align-items: center; gap: var(--ui-space-2); }
.server-cell { font-size: var(--ui-fs-xs); color: var(--ui-fg-3); }
</style>
