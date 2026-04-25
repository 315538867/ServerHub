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
      style="width: 600px"
      :bordered="false"
    >
      <NForm :model="routeForm" label-placement="left" label-width="100">
        <NFormItem label="路径">
          <NInput v-model:value="routeForm.path" placeholder="如 / 或 /api" />
        </NFormItem>
        <NFormItem label="上游">
          <NRadioGroup v-model:value="routeForm.upstream_mode" size="small" style="margin-bottom: 8px">
            <NRadioButton value="service">选 Service</NRadioButton>
            <NRadioButton value="url">手填 URL</NRadioButton>
          </NRadioGroup>
          <div v-if="routeForm.upstream_mode === 'service'" class="rt-up-row">
            <NSelect
              v-model:value="routeForm.service_id"
              :options="serviceOptions"
              :loading="servicesLoading"
              placeholder="选择一个 Service"
              clearable
              style="flex: 1"
              @update:value="onServiceChange"
            />
            <NInputNumber
              v-model:value="routeForm.service_port"
              :min="1"
              :max="65535"
              placeholder="端口"
              style="width: 120px"
            />
          </div>
          <NInput
            v-else
            v-model:value="routeForm.url"
            placeholder="如 http://127.0.0.1:3000"
          />
          <div class="rt-up-hint">
            <template v-if="routeForm.upstream_mode === 'service'">
              保存时将自动渲染为 <code>http://127.0.0.1:&lt;port&gt;</code>；端口未填时使用 Service 的默认端口。
            </template>
            <template v-else>
              直接写入配置文件的 proxy_pass，不做校验。
            </template>
          </div>
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
import { ref, computed, onMounted, h, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  NRadioGroup, NRadioButton, NDataTable, NModal, NForm, NFormItem,
  NInput, NInputNumber, NPopconfirm, NSelect, useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { RefreshCw, Plus, Info } from 'lucide-vue-next'
import { showApiError } from '@/utils/errors'
import { useAppStore } from '@/stores/app'
import { getAppNginx, setExposeMode, addRoute, updateRoute, deleteRoute, applyNginx } from '@/api/approutes'
import { getServerServices } from '@/api/servers'
import type { AppNginxConfig, AppNginxRoute, ServerService } from '@/types/api'
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

// ───────── 同服务器 Services 列表 ─────────
const services = ref<ServerService[]>([])
const servicesLoading = ref(false)

const serviceOptions = computed<SelectOption[]>(() =>
  services.value.map((s) => ({
    label: s.exposed_port > 0 ? `${s.name} (:${s.exposed_port})` : s.name,
    value: s.id,
  })),
)

async function loadServices() {
  if (!app.value?.server_id) {
    services.value = []
    return
  }
  servicesLoading.value = true
  try {
    services.value = await getServerServices(app.value.server_id)
  } catch {
    services.value = []
  } finally {
    servicesLoading.value = false
  }
}

// upstream 的展示渲染：如果 URL 能反解成 Service 形式就带标记
function upstreamLabel(upstream: string): { text: string; isService: boolean } {
  const m = upstream.match(/^http:\/\/127\.0\.0\.1:(\d+)\/?$/)
  if (m) {
    const port = Number(m[1])
    const svc = services.value.find((s) => s.exposed_port === port)
    if (svc) return { text: `${svc.name} (:${port})`, isService: true }
  }
  return { text: upstream, isService: false }
}

const columns = computed<DataTableColumns<AppNginxRoute>>(() => [
  { title: '排序', key: 'sort', width: 70 },
  { title: '路径', key: 'path', width: 160 },
  {
    title: '上游', key: 'upstream', minWidth: 220, ellipsis: { tooltip: true },
    render: (row) => {
      const { text, isService } = upstreamLabel(row.upstream)
      return h('code', { class: isService ? 'mono mono--svc' : 'mono' }, text)
    },
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

// ───────── 路由表单 ─────────
const routeVisible = ref(false)
const editRoute = ref<AppNginxRoute | null>(null)
const routeForm = ref<{
  path: string
  upstream_mode: 'service' | 'url'
  service_id: number | null
  service_port: number | null
  url: string
  extra: string
  sort: number
}>({
  path: '/',
  upstream_mode: 'service',
  service_id: null,
  service_port: null,
  url: '',
  extra: '',
  sort: 0,
})

function resetForm(baseSort: number) {
  routeForm.value = {
    path: '/',
    upstream_mode: 'service',
    service_id: null,
    service_port: null,
    url: '',
    extra: '',
    sort: baseSort,
  }
}

function openAdd() {
  resetForm(config.value.routes.length * 10)
  editRoute.value = null
  routeVisible.value = true
}

function openEdit(row: AppNginxRoute) {
  editRoute.value = row
  // 尝试把 upstream 反解成 Service 形式
  const m = row.upstream.match(/^http:\/\/127\.0\.0\.1:(\d+)\/?$/)
  const svc = m ? services.value.find((s) => s.exposed_port === Number(m[1])) : null
  if (m && svc) {
    routeForm.value = {
      path: row.path,
      upstream_mode: 'service',
      service_id: svc.id,
      service_port: Number(m[1]),
      url: '',
      extra: row.extra,
      sort: row.sort,
    }
  } else {
    routeForm.value = {
      path: row.path,
      upstream_mode: 'url',
      service_id: null,
      service_port: null,
      url: row.upstream,
      extra: row.extra,
      sort: row.sort,
    }
  }
  routeVisible.value = true
}

function onServiceChange(id: number | null) {
  if (id == null) {
    routeForm.value.service_port = null
    return
  }
  const svc = services.value.find((s) => s.id === id)
  if (svc && svc.exposed_port > 0) {
    routeForm.value.service_port = svc.exposed_port
  }
}

function buildUpstream(): string | null {
  const f = routeForm.value
  if (f.upstream_mode === 'url') {
    return f.url.trim() || null
  }
  if (!f.service_id) return null
  const port = f.service_port
  if (!port || port <= 0 || port > 65535) return null
  return `http://127.0.0.1:${port}`
}

async function confirmRoute() {
  if (!routeForm.value.path) {
    message.warning('路径不能为空')
    return
  }
  const upstream = buildUpstream()
  if (!upstream) {
    message.warning(routeForm.value.upstream_mode === 'service'
      ? '请选择 Service 并填写端口'
      : '请填写上游 URL')
    return
  }
  saving.value = true
  try {
    const body = {
      path: routeForm.value.path,
      upstream,
      extra: routeForm.value.extra,
      sort: routeForm.value.sort,
    }
    if (editRoute.value) await updateRoute(appId.value, editRoute.value.id, body)
    else await addRoute(appId.value, body)
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

// 应用首次加载需要等 appStore 就绪后再拉 services
watch(() => app.value?.server_id, (sid) => {
  if (sid) loadServices()
}, { immediate: true })

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await load()
  await loadServices()
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

.rt-up-row {
  display: flex; gap: var(--space-2); align-items: center;
}
.rt-up-hint {
  margin-top: var(--space-1);
  font-size: var(--fs-xs);
  color: var(--ui-fg-4);
}
.rt-up-hint code {
  font-family: var(--font-mono);
  background: var(--ui-bg-2);
  padding: 0 4px;
  border-radius: var(--radius-sm);
}

.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-2); }

:deep(.mono) {
  font-family: var(--font-mono); font-size: var(--fs-xs);
  color: var(--ui-fg-2); background: var(--ui-bg-2);
  padding: 1px 6px; border-radius: var(--radius-sm);
  border: 1px solid var(--ui-border);
}
:deep(.mono--svc) {
  color: var(--ui-brand);
  border-color: color-mix(in srgb, var(--ui-brand) 40%, transparent);
}
:deep(.mono--muted) { color: var(--ui-fg-3); }
:deep(.placeholder) { color: var(--ui-fg-4); }
:deep(.cell-ops) { display: inline-flex; gap: var(--space-1); }
:deep(.text-danger) { color: var(--ui-danger-fg); }
</style>
