<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NDataTable, NModal, NSpace, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { applyRelease, listReleases } from '@/api/release'
import type { Release, ReleaseStatus } from '@/types/release'
import NewReleaseWizard from '../wizards/NewReleaseWizard.vue'

const props = defineProps<{ sid: number }>()
const msg = useMessage()
const rows = ref<Release[]>([])
const loading = ref(false)
const showWizard = ref(false)

const statusTone: Record<ReleaseStatus, 'default' | 'success' | 'warning' | 'error'> = {
  draft: 'default', active: 'success', archived: 'default', rolled_back: 'warning',
}

async function load() {
  loading.value = true
  try {
    rows.value = await listReleases(props.sid)
  } finally {
    loading.value = false
  }
}

async function onApply(r: Release) {
  try {
    const run = await applyRelease(props.sid, r.id, 'manual')
    msg.success(`DeployRun #${run.id} ${run.status}`)
    await load()
  } catch (e: any) {
    msg.error(e?.message || '部署失败')
  }
}

const columns: DataTableColumns<Release> = [
  { title: 'ID', key: 'id', width: 60 },
  { title: 'Label', key: 'label' },
  { title: 'Artifact', key: 'artifact_id', width: 80 },
  { title: 'EnvSet', key: 'env_set_id', width: 80, render: r => String(r.env_set_id ?? '—') },
  { title: 'ConfigSet', key: 'config_set_id', width: 90, render: r => String(r.config_set_id ?? '—') },
  {
    title: 'Status', key: 'status', width: 100,
    render: r => h(NTag, { size: 'small', type: statusTone[r.status] }, { default: () => r.status }),
  },
  { title: 'CreatedAt', key: 'created_at', width: 180 },
  {
    title: '操作', key: 'actions', width: 200,
    render: r => h(NSpace, null, {
      default: () => {
        const buttons = [
          h(NButton, {
            size: 'small', type: 'primary',
            onClick: () => onApply(r),
          }, { default: () => r.status === 'active' ? '重发' : '应用' }),
        ]
        if (r.status === 'archived' || r.status === 'rolled_back') {
          buttons.push(h(NButton, {
            size: 'small',
            onClick: () => onApply(r),
          }, { default: () => '回滚到此' }))
        }
        return buttons
      },
    }),
  },
]

onMounted(load)
</script>

<template>
  <div>
    <NSpace justify="end" style="margin-bottom:8px">
      <NButton type="primary" size="small" @click="showWizard = true">新建 Release</NButton>
      <NButton size="small" @click="load">刷新</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="rows" :loading="loading" :bordered="false" size="small" />

    <NModal v-model:show="showWizard" preset="card" title="新建 Release" style="width:720px">
      <NewReleaseWizard :sid="sid" @done="() => { showWizard = false; load() }" />
    </NModal>
  </div>
</template>
