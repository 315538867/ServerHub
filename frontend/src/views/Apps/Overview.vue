<template>
  <div class="page-container ov">
    <app-metrics-card v-if="app?.container_name" :app-id="appId" class="ov__metrics" />

    <UiSection title="应用信息">
      <template #extra>
        <UiBadge :tone="statusTone" variant="soft">
          <StatusDot :status="app?.status || 'unknown'" :size="6" />
          {{ statusText }}
        </UiBadge>
      </template>
      <div class="ov__grid">
        <div class="ov__cell"><span class="ov__lbl">描述</span><span class="ov__val">{{ app?.description || '—' }}</span></div>
        <div class="ov__cell"><span class="ov__lbl">域名</span><span class="ov__val">{{ app?.domain || '—' }}</span></div>
        <div class="ov__cell">
          <span class="ov__lbl">服务器</span>
          <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="ov__link">
            {{ server.name }} <code class="ov__code">{{ server.host }}</code>
          </router-link>
          <span v-else class="ov__val">—</span>
        </div>
        <div class="ov__cell">
          <span class="ov__lbl">Nginx 站点</span>
          <router-link v-if="app?.site_name && server" :to="`/servers/${server.id}/nginx`" class="ov__link">
            {{ app.site_name }}
          </router-link>
          <span v-else class="ov__val ov__muted">{{ app?.site_name || '未关联' }}</span>
        </div>
        <div class="ov__cell">
          <span class="ov__lbl">容器</span>
          <router-link v-if="app?.container_name && server" :to="`/servers/${server.id}/docker`" class="ov__link">
            <code class="ov__code">🐳 {{ app.container_name }}</code>
          </router-link>
          <span v-else class="ov__val ov__muted">{{ app?.container_name || '未关联' }}</span>
        </div>
        <div class="ov__cell">
          <span class="ov__lbl">基础目录</span>
          <code v-if="app?.base_dir" class="ov__code">{{ app.base_dir }}</code>
          <span v-else class="ov__val">—</span>
        </div>
        <div class="ov__cell"><span class="ov__lbl">创建时间</span><span class="ov__val ov__time">{{ app?.created_at || '—' }}</span></div>
        <div class="ov__cell"><span class="ov__lbl">最后更新</span><span class="ov__val ov__time">{{ app?.updated_at || '—' }}</span></div>
      </div>
    </UiSection>

    <UiSection v-if="app?.base_dir" title="目录结构" padding="flush">
      <template #extra>
        <UiButton variant="secondary" size="sm" :loading="loadingDirs" @click="fetchDirs">刷新</UiButton>
        <UiButton variant="primary" size="sm" :loading="initializingDirs" @click="handleInitDirs">初始化</UiButton>
      </template>
      <div class="ov__dirs">
        <UiStateBanner v-if="dirsError" status="danger" :title="dirsError" />
        <t-table
          v-else
          :data="dirs"
          :columns="dirColumns"
          row-key="name"
          size="small"
          :loading="loadingDirs"
          :empty="emptyEl"
        >
          <template #name="{ row }">
            <span class="ov__dir-name">{{ row.name }}</span>
          </template>
          <template #path="{ row }">
            <code class="ov__code">{{ row.path }}</code>
          </template>
          <template #status="{ row }">
            <UiBadge :tone="row.status === 'ok' ? 'success' : 'danger'" variant="soft">
              {{ row.status === 'ok' ? '正常' : '缺失' }}
            </UiBadge>
          </template>
          <template #size="{ row }"><span class="ov__time">{{ row.size || '—' }}</span></template>
          <template #mtime="{ row }"><span class="ov__time">{{ row.mtime || '—' }}</span></template>
        </t-table>
      </div>
    </UiSection>

    <UiSection title="快捷操作">
      <div class="ov__actions">
        <UiButton v-if="server" variant="secondary" size="sm" @click="$router.push(`/apps/${appId}/ops/terminal`)">打开终端</UiButton>
        <UiButton v-if="server" variant="secondary" size="sm" @click="$router.push(`/servers/${server.id}/files`)">文件管理</UiButton>
        <UiButton variant="danger" size="sm" @click="handleDelete">删除应用</UiButton>
      </div>
    </UiSection>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp, getAppDirs, initAppDirs } from '@/api/application'
import type { AppDirEntry } from '@/types/api'
import AppMetricsCard from '@/components/apps/AppMetricsCard.vue'
import UiSection from '@/components/ui/UiSection.vue'
import UiBadge from '@/components/ui/UiBadge.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiStateBanner from '@/components/ui/UiStateBanner.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import EmptyBlock from '@/components/ui/EmptyBlock.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

const statusTone = computed<any>(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'neutral'
})
const statusText = computed(() => {
  const s = app.value?.status
  return ({ online: '在线', offline: '离线', error: '错误', unknown: '未知' } as Record<string, string>)[s ?? ''] ?? (s ?? '—')
})

const dirs = ref<AppDirEntry[]>([])
const loadingDirs = ref(false)
const initializingDirs = ref(false)
const dirsError = ref('')

const dirColumns = [
  { colKey: 'name', title: '目录', width: 110 },
  { colKey: 'path', title: '路径', minWidth: 200 },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'size', title: '占用', width: 90 },
  { colKey: 'mtime', title: '修改时间', width: 160 },
]

const emptyEl = () => h(EmptyBlock, { title: '暂无数据', description: '点击「刷新」加载目录信息' })

async function fetchDirs() {
  if (!app.value?.base_dir) return
  loadingDirs.value = true
  dirsError.value = ''
  try {
    dirs.value = await getAppDirs(appId.value)
  } catch (e: any) {
    dirsError.value = e.message || '加载失败'
  } finally {
    loadingDirs.value = false
  }
}

async function handleInitDirs() {
  initializingDirs.value = true
  try {
    await initAppDirs(appId.value)
    MessagePlugin.success('目录初始化成功')
    await fetchDirs()
  } catch (e: any) {
    MessagePlugin.error(e.message || '初始化失败')
  } finally {
    initializingDirs.value = false
  }
}

async function handleDelete() {
  const dialog = DialogPlugin.confirm({
    header: '危险操作',
    body: `确认删除应用「${app.value?.name}」？此操作不可恢复。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      dialog.hide()
      try {
        await deleteApp(appId.value)
        MessagePlugin.success('已删除')
        await appStore.fetch()
        router.push('/dashboard')
      } catch { MessagePlugin.error('删除失败') }
    },
  })
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
  if (app.value?.base_dir) fetchDirs()
})
</script>

<style scoped>
.ov { display: flex; flex-direction: column; gap: var(--ui-space-4); padding: var(--ui-space-4) var(--ui-space-5); }
.ov__metrics { margin: 0; }

.ov__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--ui-space-2) var(--ui-space-5);
}
@media (max-width: 720px) { .ov__grid { grid-template-columns: 1fr; } }

.ov__cell {
  display: flex; align-items: center; gap: var(--ui-space-3);
  padding: 6px 0;
  border-bottom: 1px dashed var(--ui-border-subtle);
  min-width: 0;
}
.ov__cell:nth-last-child(-n+2) { border-bottom: none; }

.ov__lbl {
  flex-shrink: 0;
  width: 80px;
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-3);
}
.ov__val {
  font-size: var(--ui-fs-sm);
  color: var(--ui-fg);
  min-width: 0;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.ov__muted { color: var(--ui-fg-placeholder); }
.ov__time { font-size: var(--ui-fs-xs); color: var(--ui-fg-3); font-variant-numeric: tabular-nums; }
.ov__link {
  font-size: var(--ui-fs-sm);
  color: var(--ui-brand);
  text-decoration: none;
  display: inline-flex; align-items: center; gap: var(--ui-space-2);
  transition: color var(--ui-dur-fast);
}
.ov__link:hover { color: var(--ui-brand-hover); text-decoration: underline; }

.ov__code {
  font-family: var(--ui-font-mono);
  font-size: var(--ui-fs-xs);
  color: var(--ui-fg-2);
  background: var(--ui-bg-subtle);
  border: 1px solid var(--ui-border-subtle);
  padding: 1px 6px;
  border-radius: var(--ui-radius-sm);
}

.ov__dirs { padding: 0 var(--ui-space-5) var(--ui-space-4); }
.ov__dir-name { font-weight: var(--ui-fw-medium); color: var(--ui-fg); font-size: var(--ui-fs-sm); }

.ov__actions { display: flex; flex-wrap: wrap; gap: var(--ui-space-2); }
</style>
