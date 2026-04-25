<script setup lang="ts">
// App 级 Releases（AppReleaseSet）页面：
//  - 列表 + "从当前快照"创建
//  - 点击行打开"应用进度"抽屉，POST 走 SSE 实时推送 service_started/_line/_done
//  - Rollback 按钮在抽屉内可见（仅当 status 为 success/partial 时启用）
//
// SSE 解析在 api/apprelease.ts 的 fetch+ReadableStream 里完成；本组件只消费事件。
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  NAlert, NButton, NCard, NDataTable, NDrawer, NDrawerContent,
  NEmpty, NForm, NFormItem, NInput, NModal, NSpace, NTag, NText,
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

const route = useRoute()
const appId = computed(() => Number(route.params.appId))

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
onMounted(reload)

// ── Status helpers ──────────────────────────────────────────────────
const STATUS_LABEL: Record<AppReleaseSetStatus, string> = {
  draft: '草稿',
  applying: '执行中',
  success: '成功',
  partial: '部分成功',
  failed: '失败',
  rolled_back: '已回滚',
}
const STATUS_TYPE: Record<AppReleaseSetStatus,
  'default' | 'success' | 'warning' | 'error' | 'info'> = {
  draft: 'default',
  applying: 'info',
  success: 'success',
  partial: 'warning',
  failed: 'error',
  rolled_back: 'warning',
}

const ITEM_STATUS_TYPE: Record<AppReleaseSummaryStatus,
  'default' | 'success' | 'warning' | 'error'> = {
  success: 'success',
  failed: 'error',
  skipped: 'default',
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
// SSE 流的 AbortController：组件卸载或重新发起时取消上一次 fetch，
// 避免离开页面后 ReadableStream 仍在后台 push 事件到已销毁的 panes
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
  // 把 last_summary 的状态回填到 panes
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
      // 重置所有 pane 的状态为 pending（rollback 场景下 items 已知）
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
      // 滚动窗口控制：保留最近 500 行
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
  // 重发起先取消上一次（理论上 running 守卫已防住，这里是兜底）
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
    // 拉一次最新（status/last_summary）
    activeSet.value = await getAppReleaseSet(appId.value, activeSet.value.id)
    await reload()
  } catch (e) {
    // 用户离开页面 abort 抛 AbortError，不弹错
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
        NTag,
        { type: STATUS_TYPE[row.status as AppReleaseSetStatus], size: 'small' },
        { default: () => STATUS_LABEL[row.status as AppReleaseSetStatus] || row.status },
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
        NButton,
        { size: 'small', onClick: () => openSet(row) },
        { default: () => '查看 / 应用' },
      )
    },
  },
]
</script>

<template>
  <div class="releases-page">
    <NCard :bordered="false">
      <template #header>
        <NSpace align="center" justify="space-between" style="width:100%">
          <NText strong>App Releases</NText>
          <NSpace>
            <NButton size="small" @click="reload" :loading="loading">刷新</NButton>
            <NButton type="primary" size="small" @click="showCreate = true">
              从当前快照
            </NButton>
          </NSpace>
        </NSpace>
      </template>

      <NAlert type="info" :bordered="false" style="margin-bottom:12px">
        从 App 下所有已绑定 CurrentRelease 的 Service 拍快照，统一 Apply / Rollback。
      </NAlert>

      <NEmpty v-if="!loading && rows.length === 0" description="暂无发布集" />
      <NDataTable
        v-else
        :columns="columns"
        :data="rows"
        :loading="loading"
        :bordered="false"
        :row-key="(r: AppReleaseSet) => r.id"
        size="small"
      />
    </NCard>

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
          <NButton @click="showCreate = false">取消</NButton>
          <NButton type="primary" :loading="creating" @click="doCreate">创建</NButton>
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
            <NTag :type="STATUS_TYPE[activeSet.status]" size="small">
              {{ STATUS_LABEL[activeSet.status] || activeSet.status }}
            </NTag>
            <NText depth="3">
              {{ activeSet.applied_at
                ? `上次应用 ${new Date(activeSet.applied_at).toLocaleString()}`
                : '尚未应用' }}
            </NText>
            <NSpace style="margin-left:auto">
              <NButton
                type="primary"
                size="small"
                :loading="running && runMode === 'apply'"
                :disabled="running"
                @click="runApply"
              >
                {{ activeSet.status === 'draft' ? '首次 Apply' : '再次 Apply' }}
              </NButton>
              <NButton
                size="small"
                :loading="running && runMode === 'rollback'"
                :disabled="running || !canRollback"
                @click="runRollback"
              >
                Rollback
              </NButton>
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
                  <NTag
                    v-if="p.status === 'pending'"
                    size="small"
                    type="default"
                  >待执行</NTag>
                  <NTag
                    v-else-if="p.status === 'running'"
                    size="small"
                    type="info"
                  >执行中</NTag>
                  <NTag
                    v-else
                    size="small"
                    :type="ITEM_STATUS_TYPE[p.status as AppReleaseSummaryStatus]"
                  >
                    {{ p.status }}
                    <template v-if="p.durationSec !== undefined">
                      · {{ p.durationSec }}s
                    </template>
                  </NTag>
                </NSpace>
              </NSpace>
            </template>
            <pre v-if="p.lines.length" class="log">{{ p.lines.join('\n') }}</pre>
            <NText v-else depth="3" style="font-size:12px">
              <template v-if="p.status === 'pending'">尚未开始</template>
              <template v-else-if="p.status === 'running'">等待输出……</template>
              <template v-else>无日志输出</template>
            </NText>
            <NAlert
              v-if="p.error"
              type="error"
              :bordered="false"
              style="margin-top:8px"
            >
              {{ p.error }}
            </NAlert>
          </NCard>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped>
.releases-page { padding: 0 4px; }
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
