<template>
  <div class="dc-page">
    <UiCard padding="none">
      <div class="dc-toolbar">
        <div class="dc-hint">扫描当前服务器上运行的 Docker 容器、docker-compose 项目、systemd 服务与 Nginx 静态站点，选中后批量导入为部署项。</div>
        <UiButton variant="primary" size="sm" :loading="scanning" @click="scan">
          <template #icon><RefreshCw :size="14" /></template>
          {{ scanned ? '重新扫描' : '开始扫描' }}
        </UiButton>
      </div>

      <div v-if="scanned" class="dc-body">
        <UiTabs :items="tabItems" :model-value="activeTab" @change="val => activeTab = String(val)" />

        <div class="dc-tab-body">
          <NDataTable
            :columns="columns"
            :data="currentList"
            :row-key="(row: Candidate) => row.source_id"
            :checked-row-keys="selectedKeys[activeTab]"
            @update:checked-row-keys="(keys: Array<string | number>) => selectedKeys[activeTab] = keys.map(String)"
            size="small"
            :bordered="false"
            :pagination="{ pageSize: 20 }"
          />
        </div>

        <div class="dc-footer">
          <div class="dc-summary">
            已选 <b>{{ totalSelected }}</b> 项
            <span v-if="result.errors?.length" class="dc-errs">· 扫描错误 {{ result.errors.length }} 条</span>
          </div>
          <UiButton variant="primary" size="sm" :disabled="totalSelected === 0" :loading="importing" @click="doImport">
            <template #icon><Download :size="14" /></template>
            导入所选 ({{ totalSelected }})
          </UiButton>
        </div>

        <div v-if="result.errors?.length" class="dc-errors">
          <div class="dc-errors-title">扫描错误</div>
          <pre>{{ result.errors.join('\n') }}</pre>
        </div>
      </div>

      <div v-else-if="!scanning" class="dc-empty">
        <div>点击「开始扫描」检测当前服务器上可迁移的服务。</div>
      </div>
    </UiCard>

    <NModal
      v-model:show="takeoverDialogVisible"
      preset="card"
      title="接管到标准目录"
      style="width: 540px"
      :bordered="false"
      :mask-closable="false"
    >
      <div v-if="takeoverTarget" class="dc-tk-info">
        <div><b>{{ takeoverTarget.name }}</b> · {{ takeoverTarget.kind }}</div>
        <div class="dc-tk-sub">{{ takeoverTarget.summary }}</div>
      </div>
      <NForm :model="takeoverForm" label-placement="left" label-width="100" style="margin-top: var(--space-4)">
        <NFormItem label="目标名称">
          <NInput v-model:value="takeoverForm.target_name" placeholder="例如 lxy-app" />
        </NFormItem>
      </NForm>
      <div class="dc-tk-warn">
        <div v-if="takeoverTarget?.kind === 'nginx'">
          复制静态文件到 <code>/opt/serverhub/apps/{{ takeoverForm.target_name || '<name>' }}/</code>，
          改写相关 nginx <code>root</code> / <code>alias</code> 指令并 reload。
          原目录会被改名为 <code>.serverhub-takeover-&lt;ts&gt;</code> 保留。
        </div>
        <div v-else-if="takeoverTarget?.kind === 'compose'">
          停掉源 compose 项目（保留 volumes），复制项目目录到
          <code>/opt/serverhub/apps/{{ takeoverForm.target_name || '<name>' }}/</code>，在新位置启动。
          原项目目录会被改名保留。
        </div>
        <div v-else-if="takeoverTarget?.kind === 'docker'">
          基于 <code>docker inspect</code> 物化为 compose 项目，bind 数据复制到 <code>./data/</code>，
          原容器停止并重命名为 <code>{{ takeoverForm.target_name || '<name>' }}-pre-takeover-&lt;ts&gt;</code> 保留。
        </div>
        <div v-else-if="takeoverTarget?.kind === 'systemd'">
          复制 WorkingDirectory + ExecStart 二进制，写入新 unit
          <code>serverhub-{{ takeoverForm.target_name || '<name>' }}.service</code>，
          停掉旧 unit 并启动新 unit。<b>系统包托管的服务会被拒绝接管。</b>
        </div>
        <div style="margin-top:6px;">中途任何步骤失败将自动回滚。</div>
      </div>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="takeoverDialogVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="takingOver" :disabled="!takeoverForm.target_name" @click="confirmTakeover">
            执行接管
          </UiButton>
        </div>
      </template>
    </NModal>

    <NModal
      v-model:show="takeoverLogVisible"
      preset="card"
      :title="takeoverLogTitle"
      style="width: 760px"
      :bordered="false"
    >
      <div :class="['dc-tk-status', takeoverStatusTone]">{{ takeoverStatusText }}</div>
      <pre class="dc-tk-log">{{ takeoverResult?.output || '' }}</pre>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="takeoverLogVisible = false">关闭</UiButton>
          <UiButton
            v-if="takeoverResult?.success && takeoverResult.deploy_id"
            variant="primary"
            size="sm"
            @click="goToApp(takeoverResult.deploy_id!)"
          >
            查看应用
          </UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDataTable, NModal, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Download } from 'lucide-vue-next'
import { scanServer, importCandidates, takeoverCandidate } from '@/api/discovery'
import type { Candidate, ScanResult, TakeoverResult } from '@/api/discovery'
import { useAppStore } from '@/stores/app'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTabs from '@/components/ui/UiTabs.vue'
import UiBadge from '@/components/ui/UiBadge.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const appStore = useAppStore()
const serverId = computed(() => Number(route.params.serverId))

const scanning = ref(false)
const scanned = ref(false)
const importing = ref(false)
const activeTab = ref<'docker' | 'compose' | 'systemd' | 'nginx'>('docker')

const result = reactive<ScanResult>({ docker: [], compose: [], systemd: [], nginx: [], errors: [] })
const selectedKeys = reactive<Record<string, string[]>>({ docker: [], compose: [], systemd: [], nginx: [] })

const tabItems = computed(() => [
  { value: 'docker',  label: `Docker (${result.docker.length})` },
  { value: 'compose', label: `Compose (${result.compose.length})` },
  { value: 'systemd', label: `systemd (${result.systemd.length})` },
  { value: 'nginx',   label: `Nginx 静态 (${result.nginx.length})` },
])

const currentList = computed<Candidate[]>(() => result[activeTab.value] ?? [])

const totalSelected = computed(() =>
  selectedKeys.docker.length + selectedKeys.compose.length + selectedKeys.systemd.length + selectedKeys.nginx.length,
)

const columns = computed<DataTableColumns<Candidate>>(() => [
  { type: 'selection' },
  { title: '名称', key: 'name', minWidth: 180, render: (row) => h('code', { class: 'dc-name' }, row.name) },
  {
    title: '类型', key: 'kind', width: 110,
    render: (row) => h(UiBadge, { tone: toneOf(row.kind) }, { default: () => row.kind }),
  },
  { title: '说明', key: 'summary', minWidth: 240 },
  {
    title: '建议', key: 'suggested', minWidth: 200,
    render: (row) => {
      const s = row.suggested
      const parts: string[] = [`type=${s.type}`]
      if (s.work_dir) parts.push(`dir=${s.work_dir}`)
      if (s.compose_file) parts.push(`file=${s.compose_file}`)
      if (s.image_name) parts.push(`image=${s.image_name}`)
      if (s.runtime) parts.push(`runtime=${s.runtime}`)
      if (s.env && s.env.length) parts.push(`env=${s.env.length}`)
      return h('span', { class: 'dc-sug' }, parts.join('  '))
    },
  },
  {
    title: '操作', key: 'actions', width: 120,
    render: (row) => {
      return h(UiButton, {
        size: 'sm',
        variant: 'secondary',
        title: '接管到 /opt/serverhub/apps/',
        onClick: () => openTakeover(row),
      }, { default: () => '接管' })
    },
  },
])

function toneOf(kind: string): 'success' | 'warning' | 'neutral' | 'info' {
  if (kind === 'docker') return 'success'
  if (kind === 'compose') return 'warning'
  if (kind === 'nginx') return 'info'
  return 'neutral'
}

async function scan() {
  scanning.value = true
  try {
    const data = await scanServer(serverId.value)
    result.docker = data.docker ?? []
    result.compose = data.compose ?? []
    result.systemd = data.systemd ?? []
    result.nginx = data.nginx ?? []
    result.errors = data.errors ?? []
    selectedKeys.docker = []
    selectedKeys.compose = []
    selectedKeys.systemd = []
    selectedKeys.nginx = []
    scanned.value = true
    const total = result.docker.length + result.compose.length + result.systemd.length + result.nginx.length
    message.success(`扫描完成：发现 ${total} 个候选`)
  } catch (e: unknown) {
    const err = e as { message?: string }
    message.error('扫描失败：' + (err.message ?? String(e)))
  } finally {
    scanning.value = false
  }
}

async function doImport() {
  if (totalSelected.value === 0) return
  importing.value = true
  try {
    const pick = (list: Candidate[], keys: string[]) =>
      list.filter((c) => keys.includes(c.source_id))
    const payload = {
      docker: pick(result.docker, selectedKeys.docker),
      compose: pick(result.compose, selectedKeys.compose),
      systemd: pick(result.systemd, selectedKeys.systemd),
      nginx: pick(result.nginx, selectedKeys.nginx),
    }
    const data = await importCandidates(serverId.value, payload)
    const parts = [`导入 ${data.imported}`, `跳过 ${data.skipped}`]
    if (data.errors?.length) parts.push(`错误 ${data.errors.length}`)
    message.success(parts.join(' · '))
    if (data.errors?.length) {
      result.errors = data.errors
    }
    selectedKeys.docker = []
    selectedKeys.compose = []
    selectedKeys.systemd = []
    selectedKeys.nginx = []
    // 新导入的应用需要立刻反映在侧边栏 / 应用列表里
    await appStore.fetch()
    if (data.imported > 0 && !data.errors?.length) {
      router.push('/apps')
    }
  } catch (e: unknown) {
    const err = e as { message?: string }
    message.error('导入失败：' + (err.message ?? String(e)))
  } finally {
    importing.value = false
  }
}

// ───────── 接管（Takeover）─────────
const takeoverDialogVisible = ref(false)
const takeoverLogVisible = ref(false)
const takingOver = ref(false)
const takeoverTarget = ref<Candidate | null>(null)
const takeoverForm = reactive<{ target_name: string }>({ target_name: '' })
const takeoverResult = ref<TakeoverResult | null>(null)

const takeoverLogTitle = computed(() => {
  const r = takeoverResult.value
  if (!r) return '接管日志'
  if (r.success) return '接管成功'
  if (r.rolled_back) return '已自动回滚'
  return '接管失败'
})

const takeoverStatusTone = computed(() => {
  const r = takeoverResult.value
  if (!r) return 'dc-tk-status-info'
  if (r.success) return 'dc-tk-status-ok'
  if (r.rolled_back) return 'dc-tk-status-warn'
  return 'dc-tk-status-err'
})

const takeoverStatusText = computed(() => {
  const r = takeoverResult.value
  if (!r) return ''
  if (r.success) return `✓ Deploy 已创建（id=${r.deploy_id}），原服务已迁移至 /opt/serverhub/apps/`
  if (r.rolled_back) return `↩ ${r.error || '步骤失败'}，已自动回滚到接管前状态`
  return `✗ ${r.error || '失败'}（注意：可能未完整回滚，请检查 /opt/serverhub/backups/）`
})

function slugify(s: string): string {
  return s.toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64) || 'app'
}

function openTakeover(row: Candidate) {
  takeoverTarget.value = row
  takeoverForm.target_name = slugify(row.name)
  takeoverDialogVisible.value = true
}

async function confirmTakeover() {
  if (!takeoverTarget.value || !takeoverForm.target_name) return
  takingOver.value = true
  try {
    const data = await takeoverCandidate(serverId.value, {
      candidate: takeoverTarget.value,
      target_name: takeoverForm.target_name,
    })
    takeoverResult.value = data
    takeoverDialogVisible.value = false
    takeoverLogVisible.value = true
    if (data.success) {
      await appStore.fetch()
      message.success('接管成功')
    } else if (data.rolled_back) {
      message.warning('已自动回滚')
    } else {
      message.error('接管失败')
    }
  } catch (e: unknown) {
    const err = e as { message?: string }
    message.error('接管请求失败：' + (err.message ?? String(e)))
  } finally {
    takingOver.value = false
  }
}

function goToApp(deployId: number) {
  takeoverLogVisible.value = false
  router.push(`/apps/${deployId}`)
}
</script>

<style scoped>
.dc-page { padding: var(--space-4) var(--space-8) var(--space-6); }
.dc-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--ui-border);
}
.dc-hint { font-size: var(--fs-sm); color: var(--ui-fg-3); }
.dc-body { display: flex; flex-direction: column; }
.dc-tab-body { padding: var(--space-3) var(--space-5); }
.dc-footer {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--space-3) var(--space-5);
  border-top: 1px solid var(--ui-border);
}
.dc-summary { font-size: var(--fs-sm); color: var(--ui-fg-2); }
.dc-errs { color: var(--ui-danger); margin-left: var(--space-2); }
.dc-empty {
  padding: var(--space-10) var(--space-5);
  text-align: center;
  color: var(--ui-fg-3);
  font-size: var(--fs-sm);
}
.dc-errors {
  padding: var(--space-3) var(--space-5);
  border-top: 1px solid var(--ui-border);
  background: var(--ui-bg-2);
}
.dc-errors-title { font-size: var(--fs-sm); color: var(--ui-danger); margin-bottom: var(--space-2); }
.dc-errors pre {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-2);
  white-space: pre-wrap;
  margin: 0;
}
:deep(.dc-name) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
}
:deep(.dc-sug) {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}

.modal-foot {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}
.dc-tk-info {
  padding: var(--space-3);
  background: var(--ui-bg-2);
  border-radius: var(--radius-md);
  font-size: var(--fs-sm);
}
.dc-tk-sub {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  margin-top: var(--space-1);
  font-family: var(--font-mono);
}
.dc-tk-warn {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
  background: var(--ui-bg-2);
  padding: var(--space-3);
  border-radius: var(--radius-md);
  line-height: 1.6;
}
.dc-tk-warn code {
  font-family: var(--font-mono);
  background: var(--ui-bg);
  padding: 1px 4px;
  border-radius: var(--radius-sm);
}
.dc-tk-status {
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  font-size: var(--fs-sm);
  margin-bottom: var(--space-3);
}
.dc-tk-status-ok   { background: rgba(34,197,94,.12);  color: #16a34a; }
.dc-tk-status-warn { background: rgba(234,179,8,.14);  color: #b45309; }
.dc-tk-status-err  { background: rgba(239,68,68,.12);  color: #dc2626; }
.dc-tk-status-info { background: var(--ui-bg-2);       color: var(--ui-fg-2); }
.dc-tk-log {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: #0b0f17;
  color: #e6edf3;
  padding: var(--space-3);
  border-radius: var(--radius-md);
  max-height: 480px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
