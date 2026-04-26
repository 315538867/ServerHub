<template>
  <div class="app-ig">
    <UiSection title="Ingress 路由（反向视图）">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="load">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>

      <UiCard v-if="!loading && rows.length === 0" padding="md">
        <div class="app-ig__empty">
          <p>没有任何 Ingress 路由到本应用的 Service。</p>
          <p class="app-ig__hint">
            去
            <RouterLink class="app-ig__link" to="/ingresses">Ingress 管理</RouterLink>
            或对应 Server 的
            <code>路由</code> 页新增一条把上游指到本应用的 Service。
          </p>
        </div>
      </UiCard>

      <UiCard v-else padding="none">
        <NDataTable
          :columns="columns"
          :data="rows"
          :loading="loading"
          :row-key="(row: AppIngress) => row.id"
          size="small"
          :bordered="false"
          :default-expand-all="true"
          :render-expand="renderRoutes"
        />
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { useRoute, RouterLink, useRouter } from 'vue-router'
import { NDataTable, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { listAppIngresses, type AppIngress } from '@/api/application'
import type { IngressRoute } from '@/api/ingresses'
import { showApiError } from '@/utils/errors'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const route = useRoute()
const router = useRouter()
const appId = computed(() => Number(route.params.appId))

const loading = ref(false)
const rows = ref<AppIngress[]>([])

async function load() {
  loading.value = true
  try {
    rows.value = await listAppIngresses(appId.value)
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

const columns: DataTableColumns<AppIngress> = [
  { type: 'expand', renderExpand: (row) => renderRoutes(row) } as any,
  { title: '域名', key: 'domain', minWidth: 180, ellipsis: { tooltip: true } },
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
    title: '所在 Edge', key: 'edge_server_name', width: 160,
    render: (row) => h('a', {
      class: 'app-ig__link',
      onClick: () => router.push(`/servers/${row.edge_server_id}/ingresses`),
    }, row.edge_server_name || `#${row.edge_server_id}`),
  },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => statusLabel(row.status)),
  },
  {
    title: '命中本应用的路由数', key: 'matching_routes', width: 160,
    render: (row) => `${row.matching_routes.length}`,
  },
]

// 反向视图只展示命中本 app 的子路由,而不是 ingress 的全部 routes —
// 用户在这里关心的是 "我的应用被怎么路由进来",别的 app 的 location 与 raw upstream
// 在这个上下文里属于干扰。要看完整 ingress 改去 server 的 ingress 页。
function renderRoutes(row: AppIngress) {
  if (!row.matching_routes.length) {
    return h('div', { class: 'app-ig__empty-row' }, '(无命中本应用的路由)')
  }
  return h('div', { class: 'app-ig__routes' },
    row.matching_routes.map((rt: IngressRoute) => h('div', { class: 'app-ig__route' }, [
      h('span', { class: 'app-ig__sort' }, `#${rt.sort}`),
      h('code', { class: 'app-ig__mono' }, rt.path),
      h(NTag, { size: 'tiny', type: 'default' }, { default: () => rt.protocol || 'http' }),
      rt.websocket ? h(NTag, { size: 'tiny', type: 'success' }, { default: () => 'WS' }) : null,
      rt.listen_port ? h(NTag, { size: 'tiny', type: 'warning' }, { default: () => `:${rt.listen_port}` }) : null,
      h('span', { class: 'app-ig__arrow' }, '→ service#'),
      h('code', { class: 'app-ig__mono app-ig__mono--up' }, String(rt.upstream.service_id ?? '?')),
    ])),
  )
}

onMounted(load)
</script>

<style scoped>
.app-ig { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.app-ig__empty { display: flex; flex-direction: column; gap: var(--space-2); color: var(--ui-fg-3); }
.app-ig__hint { font-size: var(--fs-sm); }
.app-ig__link { color: var(--ui-brand); cursor: pointer; text-decoration: none; }
.app-ig__link:hover { text-decoration: underline; }
.app-ig__empty-row { padding: var(--space-2) var(--space-3); color: var(--ui-fg-4); font-size: var(--fs-sm); }
.app-ig__routes { display: flex; flex-direction: column; gap: var(--space-1); padding: var(--space-2) var(--space-3); }
.app-ig__route { display: flex; align-items: center; gap: var(--space-2); font-size: var(--fs-sm); }
.app-ig__sort { color: var(--ui-fg-4); font-size: var(--fs-xs); width: 36px; }
.app-ig__arrow { color: var(--ui-fg-4); }
:deep(.app-ig__mono) {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  background: var(--ui-bg-2); padding: 1px 6px;
  border-radius: var(--radius-sm); border: 1px solid var(--ui-border);
  color: var(--ui-fg-2);
}
:deep(.app-ig__mono--up) {
  color: var(--ui-brand);
  border-color: color-mix(in srgb, var(--ui-brand) 40%, transparent);
}
</style>
