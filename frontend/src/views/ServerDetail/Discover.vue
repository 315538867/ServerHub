<template>
  <div class="dc-page">
    <UiCard padding="none">
      <div class="dc-toolbar">
        <div class="dc-hint">扫描当前服务器上运行的 Docker 容器、docker-compose 项目与 systemd 服务，选中后批量导入为部署项。</div>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NDataTable, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Download } from 'lucide-vue-next'
import { scanServer, importCandidates } from '@/api/discovery'
import type { Candidate, ScanResult } from '@/api/discovery'
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
const activeTab = ref<'docker' | 'compose' | 'systemd'>('docker')

const result = reactive<ScanResult>({ docker: [], compose: [], systemd: [], errors: [] })
const selectedKeys = reactive<Record<string, string[]>>({ docker: [], compose: [], systemd: [] })

const tabItems = computed(() => [
  { value: 'docker',  label: `Docker (${result.docker.length})` },
  { value: 'compose', label: `Compose (${result.compose.length})` },
  { value: 'systemd', label: `systemd (${result.systemd.length})` },
])

const currentList = computed<Candidate[]>(() => result[activeTab.value] ?? [])

const totalSelected = computed(() =>
  selectedKeys.docker.length + selectedKeys.compose.length + selectedKeys.systemd.length,
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
      return h('span', { class: 'dc-sug' }, parts.join('  '))
    },
  },
])

function toneOf(kind: string): 'success' | 'warning' | 'neutral' {
  if (kind === 'docker') return 'success'
  if (kind === 'compose') return 'warning'
  return 'neutral'
}

async function scan() {
  scanning.value = true
  try {
    const data = await scanServer(serverId.value)
    result.docker = data.docker ?? []
    result.compose = data.compose ?? []
    result.systemd = data.systemd ?? []
    result.errors = data.errors ?? []
    selectedKeys.docker = []
    selectedKeys.compose = []
    selectedKeys.systemd = []
    scanned.value = true
    const total = result.docker.length + result.compose.length + result.systemd.length
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
</style>
