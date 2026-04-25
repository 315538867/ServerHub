<template>
  <div class="sv-page">
    <UiCard padding="none">
      <div class="sv-toolbar">
        <div class="sv-hint">
          列出当前服务器上所有 Service。未绑定应用（「浮动」）的 Service 可在 Discover 中或此处重新归属。
        </div>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="load">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </div>

      <NDataTable
        :columns="columns"
        :data="services"
        :loading="loading"
        :row-key="(row: ServerService) => row.id"
        size="small"
        :bordered="false"
        :pagination="{ pageSize: 20 }"
      />
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDataTable, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { getServerServices } from '@/api/servers'
import type { ServerService } from '@/types/api'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))

const loading = ref(false)
const services = ref<ServerService[]>([])

const columns = computed<DataTableColumns<ServerService>>(() => [
  {
    title: '名称', key: 'name', minWidth: 180,
    render: (row) => h('code', { class: 'sv-name' }, row.name),
  },
  {
    title: '类型', key: 'type', width: 130,
    render: (row) => h(UiBadge, { tone: toneOfType(row.type) }, { default: () => row.type || '-' }),
  },
  {
    title: '归属', key: 'application_name', minWidth: 160,
    render: (row) => {
      if (row.application_id && row.application_name) {
        return h('a', {
          class: 'sv-app-link',
          onClick: (e: Event) => { e.preventDefault(); router.push(`/apps/${row.application_id}/overview`) },
        }, row.application_name)
      }
      return h(UiBadge, { tone: 'warning' }, { default: () => '浮动' })
    },
  },
  {
    title: '端口', key: 'exposed_port', width: 90,
    render: (row) => row.exposed_port > 0
      ? h('code', { class: 'sv-port' }, String(row.exposed_port))
      : h('span', { class: 'sv-muted' }, '—'),
  },
  {
    title: '上次状态', key: 'last_status', width: 120,
    render: (row) => h(UiBadge, { tone: toneOfStatus(row.last_status) }, { default: () => row.last_status || '—' }),
  },
  {
    title: '工作目录', key: 'work_dir', minWidth: 180,
    render: (row) => row.work_dir
      ? h('code', { class: 'sv-muted-mono' }, row.work_dir)
      : h('span', { class: 'sv-muted' }, '—'),
  },
  {
    title: '操作', key: 'ops', width: 110,
    render: (row) => h(UiButton, {
      size: 'sm', variant: 'ghost',
      onClick: () => router.push(`/services/${row.id}`),
    }, { default: () => '详情' }),
  },
])

function toneOfType(t: string): 'success' | 'warning' | 'info' | 'neutral' {
  if (t === 'docker') return 'success'
  if (t === 'docker-compose') return 'warning'
  if (t === 'native') return 'info'
  return 'neutral'
}

function toneOfStatus(s?: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (s === 'success' || s === 'running') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'syncing') return 'warning'
  return 'neutral'
}

async function load() {
  loading.value = true
  try {
    services.value = await getServerServices(serverId.value)
  } catch (e: unknown) {
    const err = e as { message?: string }
    message.error('加载失败：' + (err.message ?? String(e)))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.sv-page { padding: var(--space-4) var(--space-8) var(--space-6); }
.sv-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--ui-border);
}
.sv-hint { font-size: var(--fs-sm); color: var(--ui-fg-3); }
:deep(.sv-name) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
}
:deep(.sv-port) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
}
:deep(.sv-muted) { color: var(--ui-fg-4); }
:deep(.sv-muted-mono) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
:deep(.sv-app-link) {
  color: var(--ui-brand);
  cursor: pointer;
  text-decoration: none;
}
:deep(.sv-app-link:hover) { text-decoration: underline; }
</style>
