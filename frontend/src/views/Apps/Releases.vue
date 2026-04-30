<template>
  <div class="releases-page">
    <!-- access_url 上下文 -->
    <UiStateBanner
      v-if="app?.access_url"
      tone="info"
      :title="`访问入口: ${app.access_url}`"
      description="配置 Release Set 并 Apply 后生效。draft 状态的 Ingress 需前往对应 Edge 服务器「应用配置」后生效。"
    />

    <UiSection title="Release Set（应用级组合发布）">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="reload">
          刷新
        </UiButton>
        <UiButton variant="primary" size="sm" @click="showCreate = true">
          从当前快照
        </UiButton>
      </template>

      <UiCard padding="none">
        <NDataTable
          :columns="columns"
          :data="rows"
          :loading="loading"
          :bordered="false"
          :row-key="(r: AppReleaseSet) => r.id"
          size="small"
        />
        <div v-if="!loading && rows.length === 0" class="releases-page__empty">
          暂无发布集。点击「从当前快照」基于各 Service 的 CurrentRelease 创建第一个发布集。
        </div>
      </UiCard>
    </UiSection>

    <!-- 创建对话框 -->
    <NModal
      v-model:show="showCreate"
      preset="card"
      title="从当前 Service 状态创建发布集"
      style="max-width:520px"
    >
      <NForm label-placement="top">
        <NFormItem label="标签（可选，留空则按 YYYY-MM-DD-N 生成）">
          <NInput v-model:value="createForm.label" placeholder="如 v1.2.0" />
        </NFormItem>
        <NFormItem label="备注">
          <NInput
            v-model:value="createForm.note"
            type="textarea"
            :rows="3"
            placeholder="本次发布说明……"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <UiButton variant="secondary" size="sm" @click="showCreate = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="creating" @click="doCreate">创建</UiButton>
        </NSpace>
      </template>
    </NModal>

    <!-- 进度抽屉 -->
    <NDrawer v-model:show="showDrawer" :width="760">
      <NDrawerContent
        :title="activeSet ? `发布集 #${activeSet.id} ${activeSet.label || ''}` : ''"
        :native-scrollbar="false"
        closable
      >
        <template v-if="activeSet">
          <NSpace align="center" style="margin-bottom:12px">
            <UiBadge :tone="setStatusTone(activeSet.status)" size="sm">
              {{ STATUS_LABEL[activeSet.status] || activeSet.status }}
            </UiBadge>
            <NText depth="3">
              {{ activeSet.applied_at
                ? `上次应用 ${new Date(activeSet.applied_at).toLocaleString()}`
                : '尚未应用' }}
            </NText>
            <NSpace style="margin-left:auto">
              <UiButton
                variant="primary"
                size="sm"
                :loading="running && runMode === 'apply'"
                :disabled="running"
                @click="runApply"
              >
                {{ activeSet.status === 'draft' ? '首次 Apply' : '再次 Apply' }}
              </UiButton>
              <UiButton
                variant="secondary"
                size="sm"
                :loading="running && runMode === 'rollback'"
                :disabled="running || !canRollback"
                @click="runRollback"
              >
                Rollback
              </UiButton>
            </NSpace>
          </NSpace>

          <NCard
            v-for="p in panes"
            :key="p.serviceId"
            size="small"
            style="margin-bottom:12px"
          >
            <template #header>
              <NSpace align="center" justify="space-between" style="width:100%">
                <NText>
                  Service #{{ p.serviceId }}
                  <NText depth="3"> · Release #{{ p.releaseId }}</NText>
                </NText>
                <NSpace>
                  <UiBadge
                    v-if="p.status === 'pending'"
                    size="sm" tone="neutral"
                  >待执行</UiBadge>
                  <UiBadge
                    v-else-if="p.status === 'running'"
                    size="sm" tone="info"
                  >执行中</UiBadge>
                  <UiBadge
                    v-else
                    size="sm"
                    :tone="itemStatusTone(p.status)"
                  >
                    {{ p.status }}
                    <template v-if="p.durationSec !== undefined">
                      · {{ p.durationSec }}s
                    </template>
                  </UiBadge>
                </NSpace>
              </NSpace>
            </template>
            <pre v-if="p.lines.length" class="log">{{ p.lines.join('\n') }}</pre>
            <NText v-else depth="3" style="font-size:12px">
              <template v-if="p.status === 'pending'">尚未开始</template>
              <template v-else-if="p.status === 'running'">等待输出……</template>
              <template v-else>无日志输出</template>
            </NText>
            <UiStateBanner
              v-if="p.error"
              tone="danger"
              :title="p.error"
            />
          </NCard>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  NCard, NDataTable, NDrawer, NDrawerContent, NForm,
  NFormItem, NInput, NModal, NSpace, NText,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { $message } from '@/utils/discrete'
import {
  applyAppReleaseSetSSE,
  createAppReleaseSet,
  getAppReleaseSet,
  listAppReleaseSets,
  rollbackAppReleaseSetSSE,
} from '@/api/apprelease'
import type {
  AppReleaseSet,
  AppReleaseSetItem,
  AppReleaseSetStatus,
  AppReleaseSummaryItem,
  AppReleaseSummaryStatus,
  ApplySseEvent,
  ServiceDoneEvent,
  ServiceLineEvent,
  ServiceStartedEvent,
  SetDoneEvent,
} from '@/types/apprelease'
import { useAppStore } from '@/stores/app'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiStateBanner from '@/components/ui/UiStateBanner.vue'

const route = useRoute()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const rows = ref<AppReleaseSet[]>([])
const loading = ref(false)

async function reload() {
  loading.value = true
  try {
    rows.value = (await listAppReleaseSets(appId.value)) ?? []
  } catch {
    rows.value = []
  } finally {
    loading.value = false
  }
}

// ── Status helpers ──────────────────────────────────────────────────
const STATUS_LABEL: Record<AppReleaseSetStatus, string> = {
  draft: '草稿',
  applying: '执行中',
  success: '成功',
  partial: '部分成功',
  failed: '失败',
  rolled_back: '已回滚',
}

function setStatusTone(s: string): 'success' | 'neutral' | 'warning' | 'danger' | 'info' {
  switch (s) {
    case 'success': return 'success'
    case 'applying': return 'info'
    case 'partial':
    case 'rolled_back': return 'warning'
    case 'failed': return 'danger'
    default: return 'neutral'
  }
}

function itemStatusTone(s: string): 'success' | 'neutral' | 'warning' | 'danger' | 'info' {
  switch (s) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    default: return 'neutral'
  }
}

// ── 创建对话框 ──────────────────────────────────────────────────────
const showCreate = ref(false)
const createForm = ref<{ label: string; note: string }>({ label: '', note: '' })
const creating = ref(false)

async function doCreate() {
  creating.value = true
  try {
    await createAppReleaseSet(appId.value, {
      label: createForm.value.label || undefined,
      note: createForm.value.note || undefined,
    })
    $message().success('已创建发布集')
    showCreate.value = false
    createForm.value = { label: '', note: '' }
    await reload()
  } catch {
    /* 拦截器已弹错 */
  } finally {
    creating.value = false
  }
}

// ── Apply 进度抽屉 ───────────────────────────────────────────────────
const showDrawer = ref(false)
const activeSet = ref<AppReleaseSet | null>(null)
const running = ref(false)
const runMode = ref<'apply' | 'rollback'>('apply')
let abortCtrl: AbortController | null = null

interface ServicePane {
  serviceId: number
  releaseId: number
  status: AppReleaseSummaryStatus | 'pending' | 'running'
  durationSec?: number
  error?: string
  lines: string[]
}
const panes = ref<ServicePane[]>([])
const summary = ref<AppReleaseSummaryItem[]>([])

function openSet(row: AppReleaseSet) {
  activeSet.value = row
  panes.value = parseItems(row.items).map((it) => ({
    serviceId: it.service_id,
    releaseId: it.release_id,
    status: 'pending',
    lines: [],
  }))
  summary.value = parseSummary(row.last_summary)
  for (const s of summary.value) {
    const p = panes.value.find((x) => x.serviceId === s.service_id)
    if (p) {
      p.status = s.status
      p.error = s.error
    }
  }
  showDrawer.value = true
}

function parseItems(raw: string): AppReleaseSetItem[] {
  if (!raw) return []
  try { return JSON.parse(raw) as AppReleaseSetItem[] } catch { return [] }
}
function parseSummary(raw: string): AppReleaseSummaryItem[] {
  if (!raw) return []
  try { return JSON.parse(raw) as AppReleaseSummaryItem[] } catch { return [] }
}

function applyEvent(e: ApplySseEvent) {
  switch (e.name) {
    case 'set_started': {
      for (const p of panes.value) {
        p.status = 'pending'
        p.lines = []
        p.error = undefined
        p.durationSec = undefined
      }
      break
    }
    case 'service_started': {
      const d = e.data as ServiceStartedEvent
      const p = ensurePane(d.service_id, d.release_id)
      p.status = 'running'
      break
    }
    case 'service_line': {
      const d = e.data as ServiceLineEvent
      const p = ensurePane(d.service_id)
      p.lines.push(d.line)
      if (p.lines.length > 500) p.lines.splice(0, p.lines.length - 500)
      break
    }
    case 'service_done': {
      const d = e.data as ServiceDoneEvent
      const p = ensurePane(d.service_id)
      p.status = d.status
      p.durationSec = d.duration_sec
      p.error = d.error
      break
    }
    case 'set_done': {
      const d = e.data as SetDoneEvent
      summary.value = d.summary
      if (activeSet.value) activeSet.value.status = d.status
      break
    }
    case 'error': {
      $message().error((e.data as { error: string }).error || 'SSE 异常')
      break
    }
  }
}

function ensurePane(serviceId: number, releaseId?: number): ServicePane {
  let p = panes.value.find((x) => x.serviceId === serviceId)
  if (!p) {
    p = { serviceId, releaseId: releaseId ?? 0, status: 'pending', lines: [] }
    panes.value.push(p)
  }
  return p
}

async function runStream(mode: 'apply' | 'rollback') {
  if (!activeSet.value) return
  abortCtrl?.abort()
  abortCtrl = new AbortController()
  running.value = true
  runMode.value = mode
  const fn = mode === 'apply' ? applyAppReleaseSetSSE : rollbackAppReleaseSetSSE
  const errPrefix = mode === 'apply' ? 'Apply' : 'Rollback'
  try {
    await fn(appId.value, activeSet.value.id, {
      onEvent: applyEvent,
      signal: abortCtrl.signal,
    })
    activeSet.value = await getAppReleaseSet(appId.value, activeSet.value.id)
    await reload()
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    $message().error((e as Error).message || `${errPrefix} 失败`)
  } finally {
    running.value = false
  }
}

onBeforeUnmount(() => {
  abortCtrl?.abort()
})

const runApply = () => runStream('apply')
const runRollback = () => runStream('rollback')

const canRollback = computed(() => {
  if (!activeSet.value) return false
  return ['success', 'partial'].includes(activeSet.value.status)
})

// ── 表格列 ──────────────────────────────────────────────────────────
const columns: DataTableColumns<AppReleaseSet> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '标签', key: 'label', width: 160 },
  {
    title: '状态', key: 'status', width: 120,
    render(row) {
      return h(
        UiBadge,
        { tone: setStatusTone(row.status), size: 'sm' },
        () => STATUS_LABEL[row.status as AppReleaseSetStatus] || row.status,
      )
    },
  },
  {
    title: '服务数', key: '_count', width: 90,
    render(row) { return parseItems(row.items).length },
  },
  { title: '备注', key: 'note', ellipsis: { tooltip: true } },
  { title: '创建人', key: 'created_by', width: 110 },
  {
    title: '创建时间', key: 'created_at', width: 170,
    render(row) {
      return row.created_at ? new Date(row.created_at).toLocaleString() : '-'
    },
  },
  {
    title: '操作', key: '_act', width: 110,
    render(row) {
      return h(
        UiButton,
        { variant: 'secondary', size: 'sm', onClick: () => openSet(row) },
        () => '查看 / 应用',
      )
    },
  },
]

onMounted(async () => {
  await appStore.ensure()
  reload()
})
</script>

<style scoped>
.releases-page {
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.releases-page__empty {
  padding: var(--space-10) var(--space-4);
  text-align: center;
  color: var(--ui-fg-4);
  font-size: var(--fs-sm);
}
.log {
  margin: 0;
  padding: 8px 10px;
  background: var(--n-color, #1e1e1e);
  color: #d4d4d4;
  border-radius: 4px;
  font-family: 'JetBrains Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
  max-height: 280px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
