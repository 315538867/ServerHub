<template>
  <div class="svc-page">
    <UiSection title="服务列表">
      <template #extra>
        <UiButton variant="primary" size="sm" @click="createServiceModalRef?.open()">
          新建服务
        </UiButton>
        <UiButton variant="secondary" size="sm" @click="attachServiceModalRef?.open()">
          挂载已有
        </UiButton>
        <UiButton
          variant="secondary"
          size="sm"
          @click="showReleaseSet = !showReleaseSet"
        >
          批量部署
        </UiButton>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="fetchServices">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
      </template>

      <UiCard padding="none">
        <NDataTable
          :columns="columns"
          :data="services"
          :loading="loading"
          :bordered="false"
          :row-key="(r: ServiceRow) => r.id"
          size="small"
        />
        <div v-if="!loading && services.length === 0" class="svc-empty">
          暂无服务。点击「新建服务」创建第一个服务，或「挂载已有」关联已发现的服务。
        </div>
      </UiCard>
    </UiSection>

    <AppReleaseSetDrawer
      v-if="showReleaseSet"
      :app-id="appId"
      @close="showReleaseSet = false"
    />

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
import { computed, h, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDataTable, NSpace, useDialog, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw } from 'lucide-vue-next'
import { listAppServices, detachServiceFromApp } from '@/api/application'
import type { AppService } from '@/api/application'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import CreateServiceModal from '@/components/apps/CreateServiceModal.vue'
import AttachServiceModal from '@/components/apps/AttachServiceModal.vue'
import AppReleaseSetDrawer from '@/components/apps/AppReleaseSetDrawer.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()
const message = useMessage()
const dialog = useDialog()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

interface ServiceRow extends AppService {
  current_release_id?: number | null
}

const services = ref<ServiceRow[]>([])
const loading = ref(false)
const showReleaseSet = ref(false)

const createServiceModalRef = ref<InstanceType<typeof CreateServiceModal>>()
const attachServiceModalRef = ref<InstanceType<typeof AttachServiceModal>>()

async function fetchServices() {
  loading.value = true
  try {
    services.value = await listAppServices(appId.value) as ServiceRow[]
  } catch (e: any) {
    message.error(e.message || '加载服务失败')
  } finally {
    loading.value = false
  }
}

function confirmDetach(row: ServiceRow) {
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

const columns: DataTableColumns<ServiceRow> = [
  {
    title: '名称', key: 'name', minWidth: 140,
    render: (row) => h(
      'router-link',
      {
        to: `/services/${row.id}`,
        class: 'svc-link',
      },
      () => row.name,
    ),
  },
  { title: '类型', key: 'type', width: 130 },
  {
    title: '状态', key: 'last_status', width: 90,
    render: (row) => h(UiBadge,
      {
        tone: row.last_status === 'success' ? 'success'
          : row.last_status === 'failed' ? 'danger' : 'neutral',
      },
      () => row.last_status || '—',
    ),
  },
  {
    title: '当前 Release', key: 'current_release_id', width: 130,
    render: (row) => (row as any).current_release_id ? `#${(row as any).current_release_id}` : '—',
  },
  {
    title: '来源', key: 'source_kind', width: 100,
    render: (row) => row.source_kind || '—',
  },
  {
    title: '操作', key: 'actions', width: 190,
    render: (row) => h(NSpace, null, () => [
      h(UiButton, {
        size: 'sm', variant: 'primary',
        onClick: () => router.push(`/services/${row.id}`),
      }, () => '管理 Release'),
      h(UiButton, {
        size: 'sm', variant: 'ghost',
        onClick: () => confirmDetach(row),
      }, () => h('span', { style: { color: 'var(--ui-danger)' } }, '卸下')),
    ]),
  },
]

onMounted(async () => {
  await appStore.ensure()
  await serverStore.ensure()
  fetchServices()
})
</script>

<style scoped>
.svc-page {
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.svc-empty {
  padding: var(--space-10) var(--space-4);
  text-align: center;
  color: var(--ui-fg-4);
  font-size: var(--fs-sm);
}
.svc-link {
  color: var(--ui-brand-fg);
  text-decoration: none;
  font-weight: var(--fw-medium);
}
.svc-link:hover {
  text-decoration: underline;
}
</style>
