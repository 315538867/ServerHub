<template>
  <div class="ov-page">
    <!-- 访问入口 -->
    <UiSection v-if="app?.expose_mode !== 'none' && app?.access_url" title="访问入口">
      <UiCard padding="md">
        <div class="ov__access-bar">
          <div class="ov__access-left">
            <Globe :size="18" class="ov__access-icon" />
            <a :href="app.access_url" target="_blank" rel="noopener" class="ov__access-link">{{ app.access_url }}</a>
          </div>
          <div class="ov__access-actions">
            <UiButton size="sm" variant="secondary" @click="copyUrl">
              <template #icon><Copy :size="14" /></template>
              复制
            </UiButton>
            <UiButton size="sm" variant="primary" @click="openUrl">
              <template #icon><ExternalLink :size="14" /></template>
              打开
            </UiButton>
          </div>
        </div>
      </UiCard>
    </UiSection>

    <!-- 拓扑 -->
    <NetworkTopology :app-id="appId" :ingresses="appIngresses" />

    <AppMetricsCard v-if="app?.container_name" :app-id="appId" />

    <UiSection title="应用信息">
      <template #extra>
        <UiBadge :tone="statusTone">
          <span class="ov__badge">
            <StatusDot :status="app?.status || 'unknown'" :size="6" />
            {{ statusText }}
          </span>
        </UiBadge>
      </template>
      <UiCard padding="md">
        <div class="ov__grid">
          <div class="ov__cell"><span class="ov__lbl">描述</span><span class="ov__val">{{ app?.description || '—' }}</span></div>
          <div class="ov__cell"><span class="ov__lbl">域名</span><span class="ov__val">{{ app?.domain || '—' }}</span></div>
          <div class="ov__cell">
            <span class="ov__lbl">服务器</span>
            <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="ov__link">
              {{ server.name }} <code class="ov__code">{{ server.host }}</code>
            </router-link>
            <span v-else class="ov__val">—</span>
          </div>
          <div class="ov__cell">
            <span class="ov__lbl">Nginx 站点</span>
            <router-link v-if="app?.site_name && server" :to="`/servers/${server.id}/nginx`" class="ov__link">
              {{ app.site_name }}
            </router-link>
            <span v-else class="ov__val ov__muted">{{ app?.site_name || '未关联' }}</span>
          </div>
          <div class="ov__cell">
            <span class="ov__lbl">容器</span>
            <router-link v-if="app?.container_name && server" :to="`/servers/${server.id}/docker`" class="ov__link">
              <code class="ov__code">🐳 {{ app.container_name }}</code>
            </router-link>
            <span v-else class="ov__val ov__muted">{{ app?.container_name || '未关联' }}</span>
          </div>
          <div class="ov__cell">
            <span class="ov__lbl">基础目录</span>
            <code v-if="app?.base_dir" class="ov__code">{{ app.base_dir }}</code>
            <span v-else class="ov__val">—</span>
          </div>
          <div class="ov__cell"><span class="ov__lbl">创建时间</span><span class="ov__val ov__time">{{ app?.created_at || '—' }}</span></div>
          <div class="ov__cell"><span class="ov__lbl">最后更新</span><span class="ov__val ov__time">{{ app?.updated_at || '—' }}</span></div>
        </div>
      </UiCard>
    </UiSection>

    <UiSection v-if="app?.base_dir" title="目录结构">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loadingDirs" @click="fetchDirs">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
        <UiButton variant="primary" size="sm" :loading="initializingDirs" @click="handleInitDirs">初始化</UiButton>
      </template>
      <UiCard padding="none">
        <UiStateBanner v-if="dirsError" tone="danger" :title="dirsError" />
        <NDataTable
          v-else
          :columns="dirColumns"
          :data="dirs"
          :loading="loadingDirs"
          :row-key="(row: AppDirEntry) => row.name"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>

    <UiSection title="关联服务">
      <template #extra>
        <UiButton variant="primary" size="sm" @click="createServiceModalRef?.open()">
          新建服务
        </UiButton>
        <UiButton variant="secondary" size="sm" @click="attachServiceModalRef?.open()">
          挂载已有
        </UiButton>
        <UiButton variant="secondary" size="sm" :loading="loadingServices" @click="fetchServices">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>
      <UiCard padding="none">
        <NDataTable
          :columns="serviceColumns"
          :data="services"
          :loading="loadingServices"
          :row-key="(row: AppService) => row.id"
          size="small"
          :bordered="false"
        />
      </UiCard>
    </UiSection>

    <UiSection title="快捷操作">
      <UiCard padding="md">
        <div class="ov__actions">
          <UiButton v-if="server" variant="secondary" size="sm" @click="$router.push(`/apps/${appId}/ops/terminal`)">打开终端</UiButton>
          <UiButton v-if="server" variant="secondary" size="sm" @click="$router.push(`/servers/${server.id}/files`)">文件管理</UiButton>
          <UiButton variant="danger" size="sm" @click="handleDelete">删除应用</UiButton>
        </div>
      </UiCard>
    </UiSection>

    <CreateServiceModal
      ref="createServiceModalRef"
      :server-id="app?.server_id ?? 0"
      :application-id="appId"
      @done="fetchServices"
    />
    <AttachServiceModal
      ref="attachServiceModalRef"
      :app-id="appId"
      :server-id="app?.server_id ?? 0"
      @done="fetchServices"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDataTable, useMessage, useDialog } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Globe, Copy, ExternalLink } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp, getAppDirs, initAppDirs, listAppServices, detachServiceFromApp, listAppIngresses } from '@/api/application'
import type { AppDirEntry } from '@/types/api'
import type { AppService, AppIngress } from '@/api/application'
import AppMetricsCard from '@/components/apps/AppMetricsCard.vue'
import NetworkTopology from '@/components/apps/NetworkTopology.vue'
import CreateServiceModal from '@/components/apps/CreateServiceModal.vue'
import AttachServiceModal from '@/components/apps/AttachServiceModal.vue'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiStateBanner from '@/components/ui/UiStateBanner.vue'
import StatusDot from '@/components/ui/StatusDot.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()
const message = useMessage()
const dialog = useDialog()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

const appIngresses = ref<AppIngress[]>([])

async function fetchIngresses() {
  try { appIngresses.value = await listAppIngresses(appId.value) }
  catch { /* 静默失败，拓扑自动回退 */ }
}

function copyUrl() {
  if (!app.value?.access_url) return
  navigator.clipboard.writeText(app.value.access_url).then(
    () => message.success('已复制到剪贴板'),
    () => message.error('复制失败'),
  )
}
function openUrl() {
  if (app.value?.access_url) window.open(app.value.access_url, '_blank', 'noopener')
}

// R3 起 app.status 枚举: running | syncing | error | unknown
const statusTone = computed<any>(() => {
  const s = app.value?.status
  if (s === 'running') return 'success'
  if (s === 'syncing') return 'warning'
  if (s === 'error') return 'danger'
  return 'neutral'
})
const statusText = computed(() => {
  const s = app.value?.status
  return ({ running: '运行中', syncing: '同步中', error: '错误', unknown: '未知' } as Record<string, string>)[s ?? ''] ?? (s ?? '—')
})

const dirs = ref<AppDirEntry[]>([])
const loadingDirs = ref(false)
const initializingDirs = ref(false)
const dirsError = ref('')

const dirColumns = computed<DataTableColumns<AppDirEntry>>(() => [
  {
    title: '目录', key: 'name', width: 120,
    render: (row) => h('span', { class: 'ov__dir-name' }, row.name),
  },
  {
    title: '路径', key: 'path', minWidth: 220,
    render: (row) => h('code', { class: 'ov__code' }, row.path),
  },
  {
    title: '状态', key: 'status', width: 90,
    render: (row) => h(UiBadge,
      { tone: row.status === 'ok' ? 'success' : 'danger' },
      () => row.status === 'ok' ? '正常' : '缺失'),
  },
  {
    title: '占用', key: 'size', width: 100,
    render: (row) => h('span', { class: 'ov__time' }, row.size || '—'),
  },
  {
    title: '修改时间', key: 'mtime', width: 170,
    render: (row) => h('span', { class: 'ov__time' }, row.mtime || '—'),
  },
])

async function fetchDirs() {
  if (!app.value?.base_dir) return
  loadingDirs.value = true
  dirsError.value = ''
  try { dirs.value = await getAppDirs(appId.value) }
  catch (e: any) { dirsError.value = e.message || '加载失败' }
  finally { loadingDirs.value = false }
}

const services = ref<AppService[]>([])
const loadingServices = ref(false)

const createServiceModalRef = ref<InstanceType<typeof CreateServiceModal>>()
const attachServiceModalRef = ref<InstanceType<typeof AttachServiceModal>>()

const serviceColumns = computed<DataTableColumns<AppService>>(() => [
  {
    title: '名称', key: 'name', minWidth: 140,
    render: (row) => h('router-link', { to: `/services/${row.id}`, class: 'ov__link' }, () => row.name),
  },
  { title: '类型', key: 'type', width: 130 },
  {
    title: '状态', key: 'last_status', width: 90,
    render: (row) => h(UiBadge,
      { tone: row.last_status === 'success' ? 'success' : row.last_status === 'failed' ? 'danger' : 'neutral' },
      () => row.last_status || '—'),
  },
  { title: '来源', key: 'source_kind', width: 100, render: (row) => row.source_kind || '—' },
  { title: '目录', key: 'work_dir', minWidth: 200, render: (row) => h('code', { class: 'ov__code' }, row.work_dir || '—') },
  {
    title: '操作', key: 'actions', width: 80,
    render: (row) => h(UiButton,
      { size: 'sm', variant: 'ghost', onClick: () => confirmDetach(row) },
      () => h('span', { class: 'text-danger' }, '卸下')),
  },
])

async function fetchServices() {
  loadingServices.value = true
  try { services.value = await listAppServices(appId.value) }
  catch (e: any) { message.error(e.message || '加载服务失败') }
  finally { loadingServices.value = false }
}

function confirmDetach(row: AppService) {
  dialog.warning({
    title: '卸下服务',
    content: `将服务「${row.name}」从当前应用卸下？服务本身不会被删除，仅解除关联。`,
    positiveText: '卸下', negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await detachServiceFromApp(appId.value, row.id)
        message.success('已卸下')
        await fetchServices()
      } catch (e: any) { message.error(e.message || '操作失败') }
    },
  })
}

async function handleInitDirs() {
  initializingDirs.value = true
  try {
    await initAppDirs(appId.value)
    message.success('目录初始化成功')
    await fetchDirs()
  } catch (e: any) { message.error(e.message || '初始化失败') }
  finally { initializingDirs.value = false }
}

function handleDelete() {
  dialog.warning({
    title: '危险操作',
    content: `确认删除应用「${app.value?.name}」？此操作不可恢复。`,
    positiveText: '删除', negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await deleteApp(appId.value)
        message.success('已删除')
        await appStore.fetch()
        router.push('/dashboard')
      } catch { message.error('删除失败') }
    },
  })
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
  if (app.value?.base_dir) fetchDirs()
  fetchServices()
  fetchIngresses()
})
</script>

<style scoped>
.ov-page { display: flex; flex-direction: column; gap: var(--space-4); padding: var(--space-6); }

.ov__badge { display: inline-flex; align-items: center; gap: var(--space-2); }

.ov__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-2) var(--space-6);
}
@media (max-width: 720px) { .ov__grid { grid-template-columns: 1fr; } }

.ov__cell {
  display: flex; align-items: center; gap: var(--space-3);
  padding: var(--space-2) 0;
  border-bottom: 1px dashed var(--ui-border);
  min-width: 0;
}
.ov__cell:nth-last-child(-n+2) { border-bottom: none; }

.ov__lbl {
  flex-shrink: 0;
  width: 84px;
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.ov__val {
  font-size: var(--fs-sm);
  color: var(--ui-fg);
  min-width: 0;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.ov__muted { color: var(--ui-fg-4); }
.ov__time { font-size: var(--fs-xs); color: var(--ui-fg-3); font-variant-numeric: tabular-nums; }

.ov__link {
  font-size: var(--fs-sm);
  color: var(--ui-brand-fg);
  text-decoration: none;
  display: inline-flex; align-items: center; gap: var(--space-2);
  transition: color var(--dur-fast) var(--ease);
}
.ov__link:hover { color: var(--ui-brand); text-decoration: underline; }

.ov__code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
}

:deep(.ov__dir-name) { font-weight: var(--fw-medium); color: var(--ui-fg); font-size: var(--fs-sm); }

.ov__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }

.ov__access-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  flex-wrap: wrap;
}
.ov__access-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}
.ov__access-icon {
  color: var(--ui-success);
  flex-shrink: 0;
}
.ov__access-link {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-brand-fg);
  text-decoration: none;
  font-family: var(--font-mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color var(--dur-fast) var(--ease);
}
.ov__access-link:hover { color: var(--ui-brand); text-decoration: underline; }
.ov__access-actions {
  display: flex;
  gap: var(--space-2);
  flex-shrink: 0;
}
</style>
