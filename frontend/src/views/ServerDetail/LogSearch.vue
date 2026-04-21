<template>
  <div class="lsp-page">
    <UiSection>
      <template #title>
        <span class="lsp-title"><Search :size="16" /> 日志搜索</span>
      </template>
      <template #extra>
        <UiButton variant="secondary" size="sm" :disabled="!lines.length" @click="doExport">
          <template #icon><Download :size="14" /></template>
          导出 .txt
        </UiButton>
      </template>

      <UiCard padding="md">
        <div class="lsp-form">
          <label class="lsp-field">
            <span class="lbl">日志源</span>
            <NSelect
              v-model:value="form.source"
              :options="sourceOptions"
              size="small"
              style="width: 160px"
              @update:value="onSourceChange"
            />
          </label>

          <label v-if="needTarget" class="lsp-field">
            <span class="lbl">{{ targetLabel }}</span>
            <NInput v-model:value="form.target" size="small" :placeholder="targetPh" style="width: 260px" />
          </label>

          <label v-if="supportsSince" class="lsp-field">
            <span class="lbl">时间范围</span>
            <NSelect
              v-model:value="form.since"
              :options="sinceOptions"
              size="small"
              clearable
              placeholder="全部"
              style="width: 120px"
            />
          </label>

          <label class="lsp-field lsp-field-grow">
            <span class="lbl">关键字</span>
            <NInput
              v-model:value="form.query"
              size="small"
              placeholder="ERROR / timeout / regex..."
              @keyup.enter="doSearch"
            />
          </label>

          <label class="lsp-field">
            <span class="lbl">前后行</span>
            <NInputNumber v-model:value="form.context" size="small" :min="0" :max="10" style="width: 90px" />
          </label>

          <label class="lsp-field">
            <span class="lbl">上限</span>
            <NInputNumber v-model:value="form.limit" size="small" :min="10" :max="2000" :step="100" style="width: 110px" />
          </label>

          <div class="lsp-flags">
            <NCheckbox v-model:checked="form.regex">正则</NCheckbox>
            <NCheckbox v-model:checked="form.case_sensitive">区分大小写</NCheckbox>
          </div>

          <UiButton variant="primary" size="sm" :loading="loading" @click="doSearch">
            <template #icon><Search :size="14" /></template>
            搜索
          </UiButton>
        </div>
      </UiCard>
    </UiSection>

    <UiSection>
      <template #title>
        <span class="lsp-title">
          结果
          <UiBadge v-if="lines.length" tone="neutral">{{ lines.length }} 行</UiBadge>
          <UiBadge v-if="truncated" tone="warning">已截断</UiBadge>
        </span>
      </template>
      <UiCard padding="none">
        <div v-if="error" class="lsp-err">{{ error }}</div>
        <div v-else-if="!lines.length && !loading" class="lsp-empty">暂无结果</div>
        <pre v-else class="lsp-out"><code>{{ outputText }}</code></pre>
      </UiCard>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { NSelect, NInput, NInputNumber, NCheckbox, useMessage } from 'naive-ui'
import { Search, Download } from 'lucide-vue-next'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import { searchLogs, type LogSearchReq } from '@/api/logsearch'

const route = useRoute()
const message = useMessage()
const serverId = computed(() => Number(route.params.serverId))

const form = ref<LogSearchReq>({
  source: 'journalctl',
  target: '',
  query: '',
  regex: false,
  case_sensitive: false,
  since: '1h',
  context: 0,
  limit: 500,
})

const sourceOptions = [
  { label: 'systemd journal', value: 'journalctl' },
  { label: 'Docker 容器', value: 'docker' },
  { label: 'Nginx access', value: 'nginx-access' },
  { label: 'Nginx error', value: 'nginx-error' },
]

const sinceOptions = [
  { label: '30 分钟', value: '30m' },
  { label: '1 小时', value: '1h' },
  { label: '2 小时', value: '2h' },
  { label: '6 小时', value: '6h' },
  { label: '1 天', value: '1d' },
  { label: '2 天', value: '2d' },
  { label: '7 天', value: '7d' },
]

const needTarget = computed(() => form.value.source === 'docker' || form.value.source === 'journalctl')
const supportsSince = computed(() => form.value.source === 'docker' || form.value.source === 'journalctl')
const targetLabel = computed(() => form.value.source === 'docker' ? '容器' : '服务单元')
const targetPh = computed(() => form.value.source === 'docker' ? 'nginx / 容器名或 ID' : 'nginx.service')

function onSourceChange(v: string) {
  form.value.source = v as LogSearchReq['source']
  form.value.target = ''
}

const loading = ref(false)
const lines = ref<string[]>([])
const truncated = ref(false)
const error = ref('')

const outputText = computed(() => lines.value.join('\n'))

async function doSearch() {
  if (!form.value.query.trim()) { message.warning('请填写关键字'); return }
  if (needTarget.value && !form.value.target?.trim()) { message.warning('请填写目标'); return }
  loading.value = true
  error.value = ''
  try {
    const res = await searchLogs(serverId.value, { ...form.value })
    lines.value = res.lines.map(l => l.raw)
    truncated.value = !!res.truncated
    if (res.error) error.value = res.error
    if (!lines.value.length && !error.value) message.info('无匹配结果')
  } catch (e: any) {
    if (e?.response?.status === 429) error.value = '搜索并发已满，请稍后重试'
    else error.value = e?.message ?? '搜索失败'
  } finally {
    loading.value = false
  }
}

function doExport() {
  const blob = new Blob([outputText.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  a.href = url
  a.download = `logs-${form.value.source}-${ts}.txt`
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.lsp-page {
  padding: var(--space-4) var(--space-8) var(--space-6);
  display: flex; flex-direction: column; gap: var(--space-4);
}
.lsp-title {
  display: inline-flex; align-items: center; gap: var(--space-2);
}
.lsp-form {
  display: flex; flex-wrap: wrap; gap: var(--space-3) var(--space-4);
  align-items: flex-end;
}
.lsp-field { display: flex; flex-direction: column; gap: 4px; }
.lsp-field-grow { flex: 1; min-width: 240px; }
.lsp-field .lbl {
  font-size: var(--fs-xs);
  color: var(--ui-fg-3);
}
.lsp-flags {
  display: flex; gap: var(--space-3);
  padding-bottom: 4px;
}
.lsp-out {
  margin: 0;
  max-height: 65vh;
  overflow: auto;
  padding: var(--space-3);
  background: var(--ui-bg-2);
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  line-height: 1.55;
  color: var(--ui-fg);
  white-space: pre-wrap;
  word-break: break-all;
}
.lsp-empty, .lsp-err {
  padding: var(--space-6);
  color: var(--ui-fg-3);
  text-align: center;
  font-size: var(--fs-sm);
}
.lsp-err { color: var(--ui-danger); }
</style>
