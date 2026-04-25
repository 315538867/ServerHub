<template>
  <div class="ig-page">
    <UiSection title="Ingress 路由">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="loadAll">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
        <UiButton variant="secondary" size="sm" :loading="dryRunning" @click="doDryRun">预览变更</UiButton>
        <UiButton variant="primary" size="sm" :loading="applying" @click="doApply">应用配置</UiButton>
        <UiButton variant="primary" size="sm" @click="openCreate">新建 Ingress</UiButton>
      </template>

      <UiCard padding="none">
        <NDataTable
          :columns="ingressColumns"
          :data="ingresses"
          :loading="loading"
          :row-key="(row: Ingress) => row.id"
          size="small"
          :bordered="false"
          :expand-column-width="36"
          :default-expand-all="true"
          :render-expand="renderExpand"
        />
      </UiCard>
    </UiSection>

    <UiSection v-if="diffChanges.length || lastApply" title="最近一次操作">
      <UiCard padding="md">
        <div v-if="lastApply" class="ig-applyhead">
          <UiBadge :tone="lastApply.rolled_back ? 'danger' : (lastApply.no_op ? 'neutral' : 'success')">
            {{ lastApply.rolled_back ? '已回滚' : (lastApply.no_op ? '无变更' : '已应用') }}
          </UiBadge>
          <span class="ig-muted">audit #{{ lastApply.audit_id }}</span>
        </div>
        <div v-if="diffChanges.length" class="ig-diff">
          <div v-for="c in diffChanges" :key="c.path" class="ig-diff__row" :data-kind="c.kind">
            <span class="ig-diff__sign">{{ kindSign(c.kind) }}</span>
            <code class="ig-diff__path">{{ c.path }}</code>
            <span v-if="c.kind === 'update'" class="ig-diff__hash">
              {{ (c.old_hash ?? '').slice(0, 8) }} → {{ (c.new_hash ?? '').slice(0, 8) }}
            </span>
            <span v-else-if="c.kind === 'add'" class="ig-diff__hash">{{ (c.new_hash ?? '').slice(0, 8) }}</span>
          </div>
        </div>
        <LogOutput v-if="lastApply?.output" :content="lastApply.output" tone="dark" style="margin-top: var(--space-3)" />
      </UiCard>
    </UiSection>

    <UiSection title="历史记录">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="auditLoading" @click="loadAudit">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>
      <UiCard padding="none">
        <NDataTable :columns="auditColumns" :data="audits" size="small" :bordered="false" :loading="auditLoading" />
      </UiCard>
    </UiSection>

    <NModal
      v-model:show="ingressVisible"
      preset="card"
      :title="editIngress ? '编辑 Ingress' : '新建 Ingress'"
      style="width: 540px"
      :bordered="false"
    >
      <NForm :model="ingressForm" label-placement="left" label-width="100">
        <NFormItem label="匹配方式">
          <NRadioGroup v-model:value="ingressForm.match_kind">
            <NRadioButton value="domain">域名独占</NRadioButton>
            <NRadioButton value="path">路径共享</NRadioButton>
          </NRadioGroup>
        </NFormItem>
        <NFormItem label="域名">
          <NInput v-model:value="ingressForm.domain" placeholder="example.com 或 _" />
        </NFormItem>
        <NFormItem label="默认路径">
          <NInput v-model:value="ingressForm.default_path" placeholder="可选，默认 location 的根路径" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="ig-foot">
          <UiButton variant="secondary" size="sm" @click="ingressVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="ingressSaving" @click="saveIngress">保存</UiButton>
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="routeVisible"
      preset="card"
      :title="editRoute ? '编辑路由' : '新增路由'"
      style="width: 600px"
      :bordered="false"
    >
      <NForm :model="routeForm" label-placement="left" label-width="100">
        <NFormItem label="路径">
          <NInput v-model:value="routeForm.path" placeholder="/" />
        </NFormItem>
        <NFormItem label="协议">
          <NSelect v-model:value="routeForm.protocol" :options="protocolOptions" />
        </NFormItem>
        <NFormItem label="WebSocket">
          <NSwitch v-model:value="routeForm.websocket" />
        </NFormItem>
        <NFormItem label="上游">
          <NRadioGroup v-model:value="routeForm.upstream_kind" size="small" style="margin-bottom: 8px">
            <NRadioButton value="service">选 Service</NRadioButton>
            <NRadioButton value="raw">手填 URL</NRadioButton>
          </NRadioGroup>
          <NSelect
            v-if="routeForm.upstream_kind === 'service'"
            v-model:value="routeForm.service_id"
            :options="serviceOptions"
            :loading="servicesLoading"
            placeholder="选择一个 Service"
            clearable
          />
          <NInput v-else v-model:value="routeForm.raw_url" placeholder="如 http://outside:9000" />
        </NFormItem>
        <NFormItem label="额外指令">
          <NInput
            v-model:value="routeForm.extra"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder="可选,如 proxy_read_timeout 300;"
          />
        </NFormItem>
        <NFormItem label="排序">
          <NInputNumber v-model:value="routeForm.sort" :min="0" style="width: 100%" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="ig-foot">
          <UiButton variant="secondary" size="sm" @click="routeVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="routeSaving" @click="saveRoute">保存</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  NDataTable, NModal, NForm, NFormItem, NInput, NInputNumber, NSelect,
  NRadioGroup, NRadioButton, NSwitch, NPopconfirm, NTag, useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { RefreshCw, Plus } from 'lucide-vue-next'
import { showApiError } from '@/utils/errors'
import {
  listIngresses, getIngress, createIngress, updateIngress, deleteIngress,
  addIngressRoute, updateIngressRoute, deleteIngressRoute,
  applyEdge, dryRunEdge, listAudit, listEdgeServices,
} from '@/api/ingresses'
import type {
  Ingress, IngressDetail, IngressRoute, IngressMatchKind,
  ApplyResult, IngressChange, AuditApply, ServiceOpt, ChangeKind,
} from '@/api/ingresses'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import LogOutput from '@/components/ui/LogOutput.vue'

const route = useRoute()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))

const loading = ref(false)
const applying = ref(false)
const dryRunning = ref(false)
const auditLoading = ref(false)

const ingresses = ref<Ingress[]>([])
const detailMap = ref<Record<number, IngressDetail>>({})
const audits = ref<AuditApply[]>([])
const lastApply = ref<ApplyResult | null>(null)
const diffChanges = ref<IngressChange[]>([])

const services = ref<ServiceOpt[]>([])
const servicesLoading = ref(false)
const serviceOptions = computed<SelectOption[]>(() =>
  services.value.map((s) => ({
    label: s.exposed_port > 0 ? `${s.name} (:${s.exposed_port})` : s.name,
    value: s.id,
  })),
)

const protocolOptions = [
  { label: 'http', value: 'http' },
  { label: 'grpc', value: 'grpc' },
  // tcp/udp 需要 nginx stream 段渲染，留待 P2-D3
]

// ── 列表 ──────────────────────────────────────────────────────────────────────

async function loadIngressList() {
  loading.value = true
  try {
    ingresses.value = await listIngresses(serverId.value)
    // 并发加载所有详情(routes)
    await Promise.all(ingresses.value.map(async (ig) => {
      detailMap.value[ig.id] = await getIngress(ig.id)
    }))
  } catch (e: any) {
    showApiError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

async function loadServices() {
  servicesLoading.value = true
  try {
    services.value = await listEdgeServices(serverId.value)
  } catch {
    services.value = []
  } finally {
    servicesLoading.value = false
  }
}

async function loadAudit() {
  auditLoading.value = true
  try {
    audits.value = await listAudit(serverId.value)
  } catch (e: any) {
    showApiError(e, '加载历史失败')
  } finally {
    auditLoading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadIngressList(), loadServices(), loadAudit()])
}

// ── Ingress 表单 ──────────────────────────────────────────────────────────────

const ingressVisible = ref(false)
const ingressSaving = ref(false)
const editIngress = ref<Ingress | null>(null)
const ingressForm = ref<{ match_kind: IngressMatchKind; domain: string; default_path: string }>({
  match_kind: 'domain',
  domain: '',
  default_path: '',
})

function openCreate() {
  editIngress.value = null
  ingressForm.value = { match_kind: 'domain', domain: '', default_path: '' }
  ingressVisible.value = true
}

function openEditIngress(ig: Ingress) {
  editIngress.value = ig
  ingressForm.value = {
    match_kind: ig.match_kind,
    domain: ig.domain,
    default_path: ig.default_path,
  }
  ingressVisible.value = true
}

async function saveIngress() {
  if (!ingressForm.value.domain.trim()) {
    message.warning('域名不能为空')
    return
  }
  ingressSaving.value = true
  try {
    if (editIngress.value) {
      await updateIngress(editIngress.value.id, {
        match_kind: ingressForm.value.match_kind,
        domain: ingressForm.value.domain,
        default_path: ingressForm.value.default_path,
      })
    } else {
      await createIngress({
        edge_server_id: serverId.value,
        match_kind: ingressForm.value.match_kind,
        domain: ingressForm.value.domain,
        default_path: ingressForm.value.default_path,
      })
    }
    message.success('已保存')
    ingressVisible.value = false
    await loadIngressList()
  } catch (e: any) {
    showApiError(e, '保存失败')
  } finally {
    ingressSaving.value = false
  }
}

async function delIngress(ig: Ingress) {
  try {
    await deleteIngress(ig.id)
    message.success('已删除')
    await loadIngressList()
  } catch (e: any) {
    showApiError(e, '删除失败')
  }
}

// ── Route 表单 ────────────────────────────────────────────────────────────────

const routeVisible = ref(false)
const routeSaving = ref(false)
const editRoute = ref<IngressRoute | null>(null)
const routeIngressId = ref<number>(0)

const routeForm = ref<{
  path: string
  protocol: string
  websocket: boolean
  upstream_kind: 'service' | 'raw'
  service_id: number | null
  raw_url: string
  extra: string
  sort: number
}>({
  path: '/',
  protocol: 'http',
  websocket: false,
  upstream_kind: 'service',
  service_id: null,
  raw_url: '',
  extra: '',
  sort: 0,
})

function openAddRoute(ig: Ingress) {
  routeIngressId.value = ig.id
  editRoute.value = null
  const baseSort = (detailMap.value[ig.id]?.routes.length ?? 0) * 10
  routeForm.value = {
    path: '/',
    protocol: 'http',
    websocket: false,
    upstream_kind: 'service',
    service_id: null,
    raw_url: '',
    extra: '',
    sort: baseSort,
  }
  routeVisible.value = true
}

function openEditRoute(ig: Ingress, rt: IngressRoute) {
  routeIngressId.value = ig.id
  editRoute.value = rt
  routeForm.value = {
    path: rt.path,
    protocol: rt.protocol || 'http',
    websocket: rt.websocket,
    upstream_kind: rt.upstream.type === 'raw' ? 'raw' : 'service',
    service_id: rt.upstream.service_id ?? null,
    raw_url: rt.upstream.raw_url ?? '',
    extra: rt.extra,
    sort: rt.sort,
  }
  routeVisible.value = true
}

async function saveRoute() {
  if (!routeForm.value.path.trim()) {
    message.warning('路径不能为空')
    return
  }
  const f = routeForm.value
  const upstream = f.upstream_kind === 'service'
    ? { type: 'service' as const, service_id: f.service_id }
    : { type: 'raw' as const, raw_url: f.raw_url }
  if (f.upstream_kind === 'service' && !f.service_id) {
    message.warning('请选择 Service')
    return
  }
  if (f.upstream_kind === 'raw' && !f.raw_url.trim()) {
    message.warning('请填写 URL')
    return
  }
  routeSaving.value = true
  try {
    const body = {
      path: f.path, protocol: f.protocol, websocket: f.websocket,
      upstream, extra: f.extra, sort: f.sort,
    }
    if (editRoute.value) {
      await updateIngressRoute(routeIngressId.value, editRoute.value.id, body)
    } else {
      await addIngressRoute(routeIngressId.value, body)
    }
    message.success('已保存')
    routeVisible.value = false
    detailMap.value[routeIngressId.value] = await getIngress(routeIngressId.value)
  } catch (e: any) {
    showApiError(e, '保存失败')
  } finally {
    routeSaving.value = false
  }
}

async function delRoute(ig: Ingress, rt: IngressRoute) {
  try {
    await deleteIngressRoute(ig.id, rt.id)
    message.success('已删除')
    detailMap.value[ig.id] = await getIngress(ig.id)
  } catch (e: any) {
    showApiError(e, '删除失败')
  }
}

// ── DryRun / Apply ────────────────────────────────────────────────────────────

async function doDryRun() {
  dryRunning.value = true
  try {
    const r = await dryRunEdge(serverId.value)
    diffChanges.value = r.changes ?? []
    lastApply.value = null
    if (!diffChanges.value.length) message.success('当前实际配置已与期望一致,无变更')
  } catch (e: any) {
    showApiError(e, '预览失败')
  } finally {
    dryRunning.value = false
  }
}

async function doApply() {
  applying.value = true
  try {
    const res = await applyEdge(serverId.value)
    lastApply.value = res
    diffChanges.value = res.changes ?? []
    if (res.rolled_back) {
      message.error('应用失败已回滚,详见输出')
    } else if (res.no_op) {
      message.info('无变更')
    } else {
      message.success('已应用')
    }
    await loadIngressList()
    await loadAudit()
  } catch (e: any) {
    showApiError(e, '应用失败')
  } finally {
    applying.value = false
  }
}

// ── 表格列 ────────────────────────────────────────────────────────────────────

function statusTone(status: string): 'success' | 'neutral' | 'warning' | 'danger' {
  switch (status) {
    case 'applied': return 'success'
    case 'pending': return 'warning'
    case 'failed': return 'danger'
    default: return 'neutral'
  }
}

const ingressColumns = computed<DataTableColumns<Ingress>>(() => [
  { type: 'expand', key: 'id', renderExpand: (row: Ingress) => renderRoutes(row) } as any,
  { title: '域名', key: 'domain', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: '匹配', key: 'match_kind', width: 100,
    render: (row) => h(NTag, { size: 'small', type: row.match_kind === 'domain' ? 'info' : 'default' },
      { default: () => row.match_kind === 'domain' ? '域名' : '路径' }),
  },
  { title: '默认路径', key: 'default_path', width: 140, ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge, { tone: statusTone(row.status) }, () => row.status || '—'),
  },
  {
    title: '最近应用', key: 'last_applied_at', width: 180,
    render: (row) => row.last_applied_at ? formatDate(row.last_applied_at) : '—',
  },
  {
    title: '操作', key: 'ops', width: 240, fixed: 'right' as const,
    render: (row) => h('div', { class: 'ig-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openAddRoute(row) }, () => '+ 路由'),
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEditIngress(row) }, () => '编辑'),
      h(NPopconfirm, {
        onPositiveClick: () => delIngress(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'ig-danger' }, '删除')),
        default: () => `确认删除 ${row.domain}?`,
      }),
    ]),
  },
])

function renderExpand(row: Ingress) { return renderRoutes(row) }

function renderRoutes(row: Ingress) {
  const detail = detailMap.value[row.id]
  if (!detail) return h('div', { class: 'ig-empty' }, '加载中…')
  if (!detail.routes.length) {
    return h('div', { class: 'ig-empty' }, [
      '暂无路由 · ',
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openAddRoute(row) }, () => '+ 添加'),
    ])
  }
  return h('div', { class: 'ig-routes' },
    detail.routes.map((rt) => h('div', { class: 'ig-route' }, [
      h('span', { class: 'ig-route__sort' }, `#${rt.sort}`),
      h('code', { class: 'ig-mono' }, rt.path),
      h(NTag, { size: 'tiny', type: 'default' }, { default: () => rt.protocol || 'http' }),
      rt.websocket ? h(NTag, { size: 'tiny', type: 'success' }, { default: () => 'WS' }) : null,
      h('span', { class: 'ig-arrow' }, '→'),
      h('code', { class: 'ig-mono ig-mono--up' }, upstreamLabel(rt)),
      rt.extra ? h('code', { class: 'ig-mono ig-mono--muted' }, rt.extra) : null,
      h('span', { class: 'ig-route__ops' }, [
        h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEditRoute(row, rt) }, () => '编辑'),
        h(NPopconfirm, {
          onPositiveClick: () => delRoute(row, rt),
          positiveText: '删除', negativeText: '取消',
        }, {
          trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
            () => h('span', { class: 'ig-danger' }, '删除')),
          default: () => '确认删除该路由?',
        }),
      ]),
    ])),
  )
}

function upstreamLabel(rt: IngressRoute): string {
  if (rt.upstream.type === 'raw') return rt.upstream.raw_url ?? '(empty)'
  const sid = rt.upstream.service_id
  if (!sid) return '(unset)'
  const svc = services.value.find((s) => s.id === sid)
  return svc ? `${svc.name}${svc.exposed_port ? ' :' + svc.exposed_port : ''}` : `service#${sid}`
}

const auditColumns: DataTableColumns<AuditApply> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '时间', key: 'created_at', width: 180, render: (row) => formatDate(row.created_at) },
  { title: '操作人', key: 'actor_username', width: 140 },
  {
    title: '结果', key: 'rolled_back', width: 100,
    render: (row) => h(UiBadge, { tone: row.rolled_back ? 'danger' : 'success' },
      () => row.rolled_back ? '已回滚' : '已应用'),
  },
  { title: '变更', key: 'changeset', ellipsis: { tooltip: true } },
]

function kindSign(k: ChangeKind): string {
  return k === 'add' ? '+' : k === 'delete' ? '-' : '~'
}

function formatDate(s: string | null): string {
  if (!s) return '—'
  try { return new Date(s).toLocaleString() } catch { return s }
}

onMounted(loadAll)
</script>

<style scoped>
.ig-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }
.ig-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }
.ig-muted { color: var(--ui-fg-3); margin-left: var(--space-2); font-size: var(--fs-xs); }
.ig-empty { padding: var(--space-2) var(--space-3); color: var(--ui-fg-3); font-size: var(--fs-sm); }
.ig-routes { display: flex; flex-direction: column; gap: var(--space-1); padding: var(--space-2) var(--space-3); }
.ig-route { display: flex; align-items: center; gap: var(--space-2); font-size: var(--fs-sm); }
.ig-route__sort { color: var(--ui-fg-4); font-size: var(--fs-xs); width: 36px; }
.ig-route__ops { margin-left: auto; display: inline-flex; gap: var(--space-1); }
.ig-arrow { color: var(--ui-fg-4); }
.ig-applyhead { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
.ig-diff { display: flex; flex-direction: column; gap: 4px; font-size: var(--fs-sm); }
.ig-diff__row { display: flex; align-items: center; gap: var(--space-2); }
.ig-diff__row[data-kind="add"] .ig-diff__sign { color: var(--ui-success-fg); }
.ig-diff__row[data-kind="delete"] .ig-diff__sign { color: var(--ui-danger-fg); }
.ig-diff__row[data-kind="update"] .ig-diff__sign { color: var(--ui-brand); }
.ig-diff__sign { font-family: var(--font-mono); font-weight: var(--fw-semibold); width: 12px; }
.ig-diff__path { font-family: var(--font-mono); }
.ig-diff__hash { font-family: var(--font-mono); font-size: var(--fs-xs); color: var(--ui-fg-4); }

:deep(.ig-mono) {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  background: var(--ui-bg-2); padding: 1px 6px;
  border-radius: var(--radius-sm); border: 1px solid var(--ui-border);
  color: var(--ui-fg-2);
}
:deep(.ig-mono--up) { color: var(--ui-brand); border-color: color-mix(in srgb, var(--ui-brand) 40%, transparent); }
:deep(.ig-mono--muted) { color: var(--ui-fg-3); }
:deep(.ig-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.ig-danger) { color: var(--ui-danger-fg); }
</style>
