<template>
  <div class="page-container">
    <!-- 模式选择 -->
    <UiSection title="暴露方式" padding="default">
      <template #extra>
        <t-space size="small">
          <t-button size="small" variant="outline" :loading="loading" @click="load">
            <template #icon><refresh-icon /></template>
          </t-button>
          <t-button
            size="small"
            theme="primary"
            :loading="applying"
            :disabled="config.expose_mode === 'none'"
            @click="doApply"
          >
            应用配置
          </t-button>
        </t-space>
      </template>

      <t-radio-group v-model="config.expose_mode" variant="default-filled" @change="onModeChange">
        <t-radio-button value="none">不暴露</t-radio-button>
        <t-radio-button value="path">路径转发</t-radio-button>
        <t-radio-button value="site">独立站点</t-radio-button>
      </t-radio-group>

      <div class="mode-desc">
        <template v-if="config.expose_mode === 'none'">
          <t-icon name="info-circle" class="mode-desc-icon" />
          此应用仅内网访问，不生成任何 Nginx 配置。
        </template>
        <template v-else-if="config.expose_mode === 'path'">
          <t-icon name="info-circle" class="mode-desc-icon" />
          所有应用共用主域名，通过路径区分（如 <code>server.com/myapp/</code>）。路由规则写入
          <code>/etc/nginx/app-locations/{{ app?.name }}.conf</code>。
        </template>
        <template v-else-if="config.expose_mode === 'site'">
          <t-icon name="info-circle" class="mode-desc-icon" />
          应用独占一个域名（<code>{{ app?.domain || '请先在概览中设置域名' }}</code>），生成独立 Nginx 站点配置。
        </template>
      </div>
    </UiSection>

    <!-- 路由规则（path / site 模式） -->
    <template v-if="config.expose_mode !== 'none'">
      <UiTableCard
        title="路由规则"
        :data="config.routes"
        :columns="columns"
        :loading="loading"
        row-key="id"
        :bordered="true"
        empty="暂无路由规则，点击「添加规则」开始配置"
      >
        <template #extra>
          <t-button theme="primary" size="small" @click="openAdd">添加规则</t-button>
        </template>
        <template #upstream="{ row }">
          <span class="mono">{{ row.upstream }}</span>
        </template>
        <template #extra-cell="{ row }">
          <span v-if="row.extra" class="mono extra-text">{{ row.extra }}</span>
          <span v-else class="text-placeholder">—</span>
        </template>
        <template #operations="{ row }">
          <t-space size="small">
            <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
            <t-popconfirm content="确认删除该规则？" @confirm="delRoute(row)">
              <t-button theme="danger" size="small" variant="text">删除</t-button>
            </t-popconfirm>
          </t-space>
        </template>
      </UiTableCard>

      <!-- 应用输出 -->
      <UiSection v-if="applyOutput" title="应用输出" padding="default">
        <LogOutput :content="applyOutput" tone="dark" min-height="80px" max-height="320px" />
      </UiSection>
    </template>

    <!-- 添加/编辑规则 Dialog -->
    <t-dialog
      v-model:visible="routeVisible"
      :header="editRoute ? '编辑规则' : '添加规则'"
      width="520px"
      :confirm-btn="{ content: '保存', loading: saving }"
      @confirm="confirmRoute"
    >
      <t-form :data="routeForm" label-width="90px" colon>
        <t-form-item label="路径">
          <t-input v-model="routeForm.path" placeholder="如 / 或 /api" />
        </t-form-item>
        <t-form-item label="上游地址">
          <t-input v-model="routeForm.upstream" placeholder="如 http://127.0.0.1:3000" />
        </t-form-item>
        <t-form-item label="额外指令">
          <t-textarea
            v-model="routeForm.extra"
            placeholder="可选，如 proxy_read_timeout 300;"
            :autosize="{ minRows: 2, maxRows: 5 }"
          />
        </t-form-item>
        <t-form-item label="排序">
          <t-input-number v-model="routeForm.sort" :min="0" class="full-width" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { showApiError } from '@/utils/errors'
import { useAppStore } from '@/stores/app'
import { getAppNginx, setExposeMode, addRoute, updateRoute, deleteRoute, applyNginx } from '@/api/approutes'
import type { AppNginxConfig, AppNginxRoute } from '@/types/api'

const route = useRoute()
const appStore = useAppStore()
const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))

const loading = ref(false)
const applying = ref(false)
const saving = ref(false)
const applyOutput = ref('')

const config = ref<AppNginxConfig>({ expose_mode: 'none', routes: [] })

// 注意：extra 列使用 cell="extra-cell" 避免与 #extra header slot 冲突
const columns = [
  { colKey: 'sort', title: '排序', width: 70 },
  { colKey: 'path', title: '路径', width: 160 },
  { colKey: 'upstream', title: '上游地址', minWidth: 200, ellipsis: true },
  { colKey: 'extra', title: '额外指令', minWidth: 160, ellipsis: true, cell: 'extra-cell' },
  { colKey: 'operations', title: '操作', width: 120, fixed: 'right' as const },
]

async function load() {
  loading.value = true
  try {
    config.value = await getAppNginx(appId.value)
  } catch { /* ignore */ }
  finally { loading.value = false }
}

async function onModeChange(mode: string | number | boolean) {
  try {
    await setExposeMode(appId.value, mode as 'none' | 'path' | 'site')
    MessagePlugin.success('模式已更新')
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
    MessagePlugin.success('配置已应用')
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
    MessagePlugin.warning('路径和上游地址不能为空')
    return
  }
  saving.value = true
  try {
    if (editRoute.value) {
      await updateRoute(appId.value, editRoute.value.id, routeForm.value)
    } else {
      await addRoute(appId.value, routeForm.value)
    }
    MessagePlugin.success('保存成功')
    routeVisible.value = false
    await load()
  } catch { MessagePlugin.error('保存失败') }
  finally { saving.value = false }
}

async function delRoute(row: AppNginxRoute) {
  try {
    await deleteRoute(appId.value, row.id)
    MessagePlugin.success('已删除')
    await load()
  } catch { MessagePlugin.error('删除失败') }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  await load()
})
</script>

<style scoped>
.mode-desc {
  display: flex;
  align-items: flex-start;
  gap: var(--ui-space-2);
  margin-top: var(--ui-space-4);
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg-3);
  line-height: var(--ui-lh-relaxed);
}
.mode-desc-icon {
  flex-shrink: 0;
  margin-top: var(--ui-space-1);
  color: var(--ui-brand);
}
.extra-text { font-size: var(--ui-fs-xs); color: var(--ui-fg-3); }
.text-placeholder { color: var(--ui-fg-placeholder); }
.full-width { width: 100%; }
</style>
