<template>
  <div class="igtop">
    <UiSection title="Ingress 总览（全部 Edge）">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="loadAll">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>

      <UiCard v-if="!loading && servers.length === 0" padding="md">
        <div class="igtop__empty">
          <p>当前没有任何服务器。先去
            <RouterLink class="igtop__link" to="/servers">服务器管理</RouterLink>
            添加 Edge,再回来配置 Ingress。
          </p>
        </div>
      </UiCard>

      <UiCard v-else padding="none">
        <NDataTable
          :columns="columns"
          :data="rows"
          :loading="loading"
          :row-key="(row: Row) => `${row.edge_server_id}:${row.id}`"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>

    <UiSection v-if="!loading" title="按 Edge 汇总">
      <UiCard padding="none">
        <NDataTable
          :columns="summaryColumns"
          :data="summaryRows"
          :row-key="(row: SummaryRow) => row.edge_server_id"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, h, computed, onMounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { NDataTable, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { listIngresses, type Ingress } from '@/api/ingresses'
import { useServerStore } from '@/stores/server'
import { showApiError } from '@/utils/errors'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

// 顶级 /ingresses 是 "跨 Edge 总览" 视图,不做编辑 — 编辑在每个 Edge 各自的
// /servers/:id/ingresses 内,因为 apply 必须按 Edge 走(每台 nginx 独立 reload)。
// 这里只负责让运维一眼看清: 哪个 Edge 上哪些域名,有没有 broken / drift 需要回头处理。

interface Row extends Ingress { edge_server_name: string }
interface SummaryRow {
  edge_server_id: number
  edge_server_name: string
  total: number
  applied: number
  pending: number
  drift: number
  broken: number
}

const router = useRouter()
const serverStore = useServerStore()

const loading = ref(false)
const ingresses = ref<Ingress[]>([])

const servers = computed(() => serverStore.servers)

const rows = computed<Row[]>(() => {
  return ingresses.value
    .map((ig) => ({
      ...ig,
      edge_server_name: serverStore.getById(ig.edge_server_id)?.name || `#${ig.edge_server_id}`,
    }))
    .sort((a, b) => {
      if (a.edge_server_name === b.edge_server_name) return a.domain.localeCompare(b.domain)
      return a.edge_server_name.localeCompare(b.edge_server_name)
    })
})

const summaryRows = computed<SummaryRow[]>(() => {
  const map = new Map<number, SummaryRow>()
  for (const ig of ingresses.value) {
    let row = map.get(ig.edge_server_id)
    if (!row) {
      row = {
        edge_server_id: ig.edge_server_id,
        edge_server_name: serverStore.getById(ig.edge_server_id)?.name || `#${ig.edge_server_id}`,
        total: 0, applied: 0, pending: 0, drift: 0, broken: 0,
      }
      map.set(ig.edge_server_id, row)
    }
    row.total++
    if (ig.status === 'applied') row.applied++
    else if (ig.status === 'pending') row.pending++
    else if (ig.status === 'drift') row.drift++
    else if (ig.status === 'broken' || ig.status === 'failed') row.broken++
  }
  return Array.from(map.values()).sort((a, b) => a.edge_server_name.localeCompare(b.edge_server_name))
})

async function loadAll() {
  loading.value = true
  try {
    await serverStore.ensure()
    ingresses.value = await listIngresses()
  } catch (e: any) {
    showApiError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

function statusTone(s: string): 'success' | 'neutral' | 'warning' | 'danger' {
  switch (s) {
    case 'applied': return 'success'
    case 'pending':
    case 'drift': return 'warning'
    case 'broken':
    case 'failed': return 'danger'
    default: return 'neutral'
  }
}
function statusLabel(s: string): string {
  switch (s) {
    case 'applied': return '已生效'
    case 'pending': return '待应用'
    case 'drift': return '漂移'
    case 'broken': return '异常'
    case 'failed': return '失败'
    default: return s || '—'
  }
}

const columns: DataTableColumns<Row> = [
  {
    title: '所在 Edge', key: 'edge_server_name', width: 180,
    render: (row) => h('a', {
      class: 'igtop__link',
      onClick: () => router.push(`/servers/${row.edge_server_id}/ingresses`),
    }, row.edge_server_name),
  },
  { title: '域名', key: 'domain', minWidth: 200, ellipsis: { tooltip: true } },
  {
    title: '匹配', key: 'match_kind', width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.match_kind === 'domain' ? 'info' : 'default' },
      { default: () => row.match_kind === 'domain' ? '域名' : '路径' }),
  },
  {
    title: 'TLS', key: 'cert_id', width: 110,
    render: (row) => row.cert_id
      ? h(NTag, { size: 'small', type: row.force_https ? 'error' : 'success' },
          { default: () => row.force_https ? 'HTTPS强制' : 'HTTPS' })
      : '—',
  },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => statusLabel(row.status)),
  },
  {
    title: '最近应用', key: 'last_applied_at', width: 180,
    render: (row) => row.last_applied_at ? new Date(row.last_applied_at).toLocaleString() : '—',
  },
  {
    title: '操作', key: 'ops', width: 100, fixed: 'right' as const,
    render: (row) => h(UiButton, {
      variant: 'ghost', size: 'sm',
      onClick: () => router.push(`/servers/${row.edge_server_id}/ingresses`),
    }, () => '管理'),
  },
]

const summaryColumns: DataTableColumns<SummaryRow> = [
  {
    title: 'Edge', key: 'edge_server_name', minWidth: 180,
    render: (row) => h('a', {
      class: 'igtop__link',
      onClick: () => router.push(`/servers/${row.edge_server_id}/ingresses`),
    }, row.edge_server_name),
  },
  { title: '总数', key: 'total', width: 80 },
  {
    title: '已生效', key: 'applied', width: 100,
    render: (row) => h(UiBadge, { tone: row.applied > 0 ? 'success' : 'neutral' }, () => `${row.applied}`),
  },
  {
    title: '待应用', key: 'pending', width: 100,
    render: (row) => h(UiBadge, { tone: row.pending > 0 ? 'warning' : 'neutral' }, () => `${row.pending}`),
  },
  {
    title: '漂移', key: 'drift', width: 90,
    render: (row) => h(UiBadge, { tone: row.drift > 0 ? 'warning' : 'neutral' }, () => `${row.drift}`),
  },
  {
    title: '异常', key: 'broken', width: 90,
    render: (row) => h(UiBadge, { tone: row.broken > 0 ? 'danger' : 'neutral' }, () => `${row.broken}`),
  },
]

onMounted(loadAll)
</script>

<style scoped>
.igtop { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.igtop__empty { color: var(--ui-fg-3); }
.igtop__link { color: var(--ui-brand); cursor: pointer; text-decoration: none; }
.igtop__link:hover { text-decoration: underline; }
</style>
