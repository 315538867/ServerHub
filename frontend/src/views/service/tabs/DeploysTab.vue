<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NDataTable, NModal, NSpace, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { getDeployRun, listDeployRuns } from '@/api/release'
import type { DeployRun, DeployRunStatus } from '@/types/release'

const props = defineProps<{ sid: number }>()
const msg = useMessage()
const rows = ref<DeployRun[]>([])
const loading = ref(false)
const show = ref(false)
const detail = ref<DeployRun | null>(null)

const tone: Record<DeployRunStatus, 'default' | 'success' | 'warning' | 'error' | 'info'> = {
  running: 'info', success: 'success', failed: 'error', rolled_back: 'warning',
}

async function load() {
  loading.value = true
  try { rows.value = await listDeployRuns(props.sid) }
  finally { loading.value = false }
}

async function viewDetail(r: DeployRun) {
  try {
    detail.value = await getDeployRun(props.sid, r.id)
    show.value = true
  } catch (e: any) { msg.error(e?.message || '加载失败') }
}

const columns: DataTableColumns<DeployRun> = [
  { title: 'ID', key: 'id', width: 60 },
  { title: 'Release', key: 'release_id', width: 80 },
  {
    title: 'Status', key: 'status', width: 110,
    render: r => h(NTag, { size: 'small', type: tone[r.status] }, { default: () => r.status }),
  },
  {
    title: 'Trigger', key: 'trigger_source', width: 140,
    render: r => {
      const isAuto = r.trigger_source === 'auto_rollback'
      return h(NTag, {
        size: 'small',
        type: isAuto ? 'warning' : 'default',
      }, { default: () => isAuto ? '自动回滚' : r.trigger_source })
    },
  },
  {
    title: '回滚自', key: 'rollback_from_run_id', width: 110,
    render: r => {
      if (r.rollback_from_run_id == null) return ''
      return h(NButton, {
        size: 'tiny', quaternary: true, type: 'warning',
        onClick: () => jumpTo(r.rollback_from_run_id!),
      }, { default: () => `#${r.rollback_from_run_id}` })
    },
  },
  { title: 'Started', key: 'started_at', width: 180 },
  { title: 'Dur(s)', key: 'duration_sec', width: 80 },
  {
    title: '操作', key: 'actions', width: 100,
    render: r => h(NButton, { size: 'small', onClick: () => viewDetail(r) }, { default: () => '查看' }),
  },
]

function jumpTo(runID: number) {
  const target = rows.value.find(r => r.id === runID)
  if (target) viewDetail(target)
  else msg.warning(`Run #${runID} 不在当前列表（可能已超出保留窗口）`)
}

onMounted(load)
</script>

<template>
  <div>
    <NSpace justify="end" style="margin-bottom:8px">
      <NButton size="small" @click="load">刷新</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="rows" :loading="loading" size="small" />

    <NModal v-model:show="show" preset="card" title="DeployRun 详情" style="width:780px">
      <div v-if="detail" style="margin-bottom:12px;font-size:13px">
        <div>
          Run #{{ detail.id }} · Release #{{ detail.release_id }} ·
          <NTag size="small" :type="tone[detail.status]">{{ detail.status }}</NTag>
          <NTag
            v-if="detail.trigger_source === 'auto_rollback'"
            size="small" type="warning" style="margin-left:6px"
          >自动回滚</NTag>
          <span v-else style="margin-left:6px;color:#94a3b8">{{ detail.trigger_source }}</span>
        </div>
        <div v-if="detail.rollback_from_run_id" style="margin-top:4px;color:#d97706">
          ↩︎ 由失败 Run
          <NButton size="tiny" quaternary type="warning" @click="jumpTo(detail.rollback_from_run_id)">
            #{{ detail.rollback_from_run_id }}
          </NButton>
          触发
        </div>
        <div v-if="detail.status === 'rolled_back'" style="margin-top:4px;color:#d97706">
          ⚠️ 本次失败已被自动回滚覆盖（见新版 Run）
        </div>
        <div style="margin-top:4px;color:#64748b">
          {{ detail.started_at }} · {{ detail.duration_sec }}s
        </div>
      </div>
      <pre style="background:#0b1020;color:#e2e8f0;padding:12px;border-radius:4px;font-size:12px;max-height:420px;overflow:auto">{{ detail?.output || '(no output)' }}</pre>
    </NModal>
  </div>
</template>
