<template>
  <div class="rt-page">
    <UiSection title="暴露方式">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loading" @click="load">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </UiButton>
        <UiButton
          variant="primary" size="sm"
          :loading="applying"
          :disabled="config.expose_mode === 'none'"
          @click="doApply"
        >应用配置</UiButton>
      </template>

      <UiCard padding="md">
        <NRadioGroup v-model:value="config.expose_mode" @update:value="onModeChange">
          <NRadioButton value="none">不暴露</NRadioButton>
          <NRadioButton value="path">路径转发</NRadioButton>
          <NRadioButton value="site">独立站点</NRadioButton>
        </NRadioGroup>

        <div class="rt-desc">
          <Info :size="14" class="rt-desc__icon" />
          <span v-if="config.expose_mode === 'none'">此应用仅内网访问，不生成任何 Nginx 配置。</span>
          <span v-else-if="config.expose_mode === 'path'">
            所有应用共用主域名，通过路径区分（如 <code>server.com/myapp/</code>）。路由规则写入
            <code>/etc/nginx/app-locations/{{ app?.name }}.conf</code>。
          </span>
          <span v-else-if="config.expose_mode === 'site'">
            应用独占一个域名（<code>{{ app?.domain || '请先在概览中设置域名' }}</code>），生成独立 Nginx 站点配置。
          </span>
        </div>
      </UiCard>
    </UiSection>

    <template v-if="config.expose_mode !== 'none'">
      <UiSection title="路由规则">
        <template #extra>
          <UiButton variant="primary" size="sm" @click="openAdd">
            <template #icon><Plus :size="14" /></template>
            添加规则
          </UiButton>
        </template>
        <UiCard padding="none">
          <NDataTable
            :columns="columns"
            :data="config.routes"
            :loading="loading"
            :row-key="(row: AppNginxRoute) => row.id"
            size="small"
            :bordered="false"
          />
        </UiCard>
      </UiSection>

      <UiSection v-if="applyOutput" title="应用输出">
        <LogOutput :content="applyOutput" tone="dark">{{ applyOutput }}</LogOutput>
      </UiSection>
    </template>

    <NModal
      v-model:show="routeVisible"
      preset="card"
      :title="editRoute ? '编辑规则' : '添加规则'"
      style="width: 540px"
      :bordered="false"
    >
      <NForm :model="routeForm" label-placement="left" label-width="90">
        <NFormItem label="路径">
          <NInput v-model:value="routeForm.path" placeholder="如 / 或 /api" />
        </NFormItem>
        <NFormItem label="上游地址">
          <NInput v-model:value="routeForm.upstream" placeholder="如 http://127.0.0.1:3000" />
        </NFormItem>
        <NFormItem label="额外指令">
          <NInput
            v-model:value="routeForm.extra"
            type="textarea"
            placeholder="可选，如 proxy_read_timeout 300;"
            :autosize="{ minRows: 2, maxRows: 5 }"
          />
        </NFormItem>
        <NFormItem label="排序">
          <NInputNumber v-model:value="routeForm.sort" :min="0" style="width: 100%" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="modal-foot">
          <UiButton variant="secondary" size="sm" @click="routeVisible = false">取消</UiButton>
          <UiButton variant="primary" size="sm" :loading="saving" @click="confirmRoute">保存</UiButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute } from 'vue-router'
import {
  NRadioGroup, NRadioButton, NDataTable, NModal, NForm, NFormItem,
  NInput, NInputNumber, NPopconfirm, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshCw, Plus, Info } from 'lucide-vue-next'
import { showApiError } from '@/utils/errors'
import { useAppStore } from '@/stores/app'
import { getAppNginx, setExposeMode, addRoute, updateRoute, deleteRoute, applyNginx } from '@/api/approutes'
import type { AppNginxConfig, AppNginxRoute } from '@/types/api'
import UiSection from '@/components/ui/UiSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import LogOutput from '@/components/ui/LogOutput.vue'

const route = useRoute()
const appStore = useAppStore()
const message = useMessage()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const loading = ref(false)
const applying = ref(false)
const saving = ref(false)
const applyOutput = ref('')

const config = ref<AppNginxConfig>({ expose_mode: 'none', routes: [] })

const columns = computed<DataTableColumns<AppNginxRoute>>(() => [
  { title: '排序', key: 'sort', width: 70 },
  { title: '路径', key: 'path', width: 160 },
  {
    title: '上游地址', key: 'upstream', minWidth: 200, ellipsis: { tooltip: true },
    render: (row) => h('code', { class: 'mono' }, row.upstream),
  },
  {
    title: '额外指令', key: 'extra', minWidth: 160, ellipsis: { tooltip: true },
    render: (row) => row.extra
      ? h('code', { class: 'mono mono--muted' }, row.extra)
      : h('span', { class: 'placeholder' }, '—'),
  },
  {
    title: '操作', key: 'ops', width: 140, fixed: 'right' as const,
    render: (row) => h('div', { class: 'cell-ops' }, [
      h(UiButton, { variant: 'ghost', size: 'sm', onClick: () => openEdit(row) }, () => '编辑'),
      h(NPopconfirm, {
        onPositiveClick: () => delRoute(row),
        positiveText: '删除', negativeText: '取消',
      }, {
        trigger: () => h(UiButton, { variant: 'ghost', size: 'sm' },
          () => h('span', { class: 'text-danger' }, '删除')),
        default: () => '确认删除该规则？',
      }),
    ]),
  },
])

async function load() {
  loading.value = true
  try { config.value = await getAppNginx(appId.value) }
  catch { /* ignore */ }
  finally { loading.value = false }
}

async function onModeChange(mode: string | number | boolean) {
  try {
    await setExposeMode(appId.value, mode as 'none' | 'path' | 'site')
    message.success('模式已更新')
  } catch (e: any) {
    showApiError(e, '更新失败')
    await load()
  }
}

async function doApply() {
  applying.value = true
  applyOutput.value = ''
  try {
    const res = await applyNginx(appId.value)
    applyOutput.value = res.output || 'nginx reload 成功'
    message.success('配置已应用')
  } catch (e: any) {
    showApiError(e, '应用失败')
  } finally { applying.value = false }
}

const routeVisible = ref(false)
const editRoute = ref<AppNginxRoute | null>(null)
const routeForm = ref({ path: '/', upstream: '', extra: '', sort: 0 })

function openAdd() {
  editRoute.value = null
  routeForm.value = { path: '/', upstream: '', extra: '', sort: config.value.routes.length * 10 }
  routeVisible.value = true
}

function openEdit(row: AppNginxRoute) {
  editRoute.value = row
  routeForm.value = { path: row.path, upstream: row.upstream, extra: row.extra, sort: row.sort }
  routeVisible.value = true
}

async function confirmRoute() {
  if (!routeForm.value.path || !routeForm.value.upstream) {
    message.warning('路径和上游地址不能为空')
    return
  }
  saving.value = true
  try {
    if (editRoute.value) await updateRoute(appId.value, editRoute.value.id, routeForm.value)
    else await addRoute(appId.value, routeForm.value)
    message.success('保存成功')
    routeVisible.value = false
    await load()
  } catch { message.error('保存失败') }
  finally { saving.value = false }
}

async function delRoute(row: AppNginxRoute) {
  try {
    await deleteRoute(appId.value, row.id)
    message.success('已删除')
    await load()
  } catch { message.error('删除失败') }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await load()
})
</script>

<style scoped>
.rt-page { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-4); }

.rt-desc {
  display: flex; align-items: flex-start; gap: var(--space-2);
  margin-top: var(--space-3);
  font-size: var(--fs-sm);
  color: var(--ui-fg-3);
  line-height: var(--lh-relaxed);
}
.rt-desc__icon { flex-shrink: 0; margin-top: 3px; color: var(--ui-brand); }
.rt-desc code {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  background: var(--ui-bg-2);
  border: 1px solid var(--ui-border);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
}

.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

:deep(.mono) {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  color: var(--ui-fg-2); background: var(--ui-bg-2);
  padding: 1px 6px; border-radius: var(--radius-sm);
  border: 1px solid var(--ui-border);
}
:deep(.mono--muted) { color: var(--ui-fg-3); }
:deep(.placeholder) { color: var(--ui-fg-4); }
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
