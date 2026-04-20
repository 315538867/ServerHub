<template>
  <div class="page-container">
    <!-- 应用信息 -->
    <div class="section-block">
      <div class="section-title">
        <span class="title-text">应用信息</span>
        <t-tag :theme="statusTheme" variant="light" size="small">{{ app?.status ?? '—' }}</t-tag>
      </div>
      <div class="desc-wrap">
        <t-descriptions :column="2">
          <t-descriptions-item label="描述">{{ app?.description || '—' }}</t-descriptions-item>
          <t-descriptions-item label="域名">{{ app?.domain || '—' }}</t-descriptions-item>
          <t-descriptions-item label="所属服务器">
            <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="link">{{ server.name }} ({{ server.host }})</router-link>
            <span v-else>—</span>
          </t-descriptions-item>
          <t-descriptions-item label="Nginx 站点">
            <router-link v-if="app?.site_name && server" :to="`/servers/${server.id}/nginx`" class="link">{{ app.site_name }}</router-link>
            <span v-else>{{ app?.site_name || '未关联' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="容器名">
            <router-link v-if="app?.container_name && server" :to="`/servers/${server.id}/docker`" class="link">{{ app.container_name }}</router-link>
            <span v-else>{{ app?.container_name || '未关联' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="基础目录">
            <code v-if="app?.base_dir" class="dir-code">{{ app.base_dir }}</code>
            <span v-else>—</span>
          </t-descriptions-item>
          <t-descriptions-item label="创建时间">{{ app?.created_at }}</t-descriptions-item>
          <t-descriptions-item label="最后更新">{{ app?.updated_at }}</t-descriptions-item>
        </t-descriptions>
      </div>
    </div>

    <!-- 目录结构 -->
    <div class="section-block" v-if="app?.base_dir">
      <div class="section-title">
        <span class="title-text">目录结构</span>
        <div class="title-actions">
          <t-button
            size="small" variant="outline" :loading="loadingDirs"
            @click="fetchDirs">刷新</t-button>
          <t-button
            size="small" theme="primary" variant="outline" :loading="initializingDirs"
            @click="handleInitDirs">初始化目录</t-button>
        </div>
      </div>
      <div class="dirs-wrap">
        <div v-if="dirsError" class="dirs-error">{{ dirsError }}</div>
        <t-table
          v-else
          :data="dirs"
          :columns="dirColumns"
          row-key="name"
          size="small"
          :loading="loadingDirs"
          empty="暂无数据，请点击「刷新」加载"
        >
          <template #name="{ row }">
            <span class="dir-name">{{ row.name }}</span>
          </template>
          <template #path="{ row }">
            <code class="dir-code">{{ row.path }}</code>
          </template>
          <template #status="{ row }">
            <t-tag :theme="row.status === 'ok' ? 'success' : 'danger'" variant="light" size="small">
              {{ row.status === 'ok' ? '正常' : '缺失' }}
            </t-tag>
          </template>
          <template #size="{ row }">{{ row.size || '—' }}</template>
          <template #mtime="{ row }">{{ row.mtime || '—' }}</template>
        </t-table>
      </div>
    </div>

    <!-- 快捷操作 -->
    <div class="section-block">
      <div class="section-title">
        <span class="title-text">快捷操作</span>
      </div>
      <div class="actions-wrap">
        <t-button v-if="server" variant="outline" size="small" @click="$router.push(`/servers/${server.id}/terminal`)">打开终端</t-button>
        <t-button v-if="server" variant="outline" size="small" @click="$router.push(`/servers/${server.id}/files`)">文件管理</t-button>
        <t-button theme="danger" variant="outline" size="small" @click="handleDelete">删除应用</t-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp, getAppDirs, initAppDirs } from '@/api/application'
import type { AppDirEntry } from '@/types/api'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const serverStore = useServerStore()

const appId = computed(() => Number(route.params.appId))
const app = computed(() => appStore.getById(appId.value))
const server = computed(() => app.value ? serverStore.getById(app.value.server_id) : undefined)

const statusTheme = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'default'
})

const dirs = ref<AppDirEntry[]>([])
const loadingDirs = ref(false)
const initializingDirs = ref(false)
const dirsError = ref('')

const dirColumns = [
  { colKey: 'name', title: '目录', width: 100 },
  { colKey: 'path', title: '路径', minWidth: 200 },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'size', title: '占用', width: 90 },
  { colKey: 'mtime', title: '修改时间', width: 160 },
]

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
    body: `确认删除应用「${app.value?.name}」？`,
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
.desc-wrap {
  padding: 16px 20px 20px;
}
:deep(.t-descriptions__label) {
  color: var(--sh-text-secondary);
  font-size: 13px;
  width: 90px;
}
:deep(.t-descriptions__content) {
  font-size: 13px;
}
.title-actions {
  display: flex;
  gap: 8px;
}
.dirs-wrap {
  padding: 4px 20px 20px;
}
.dirs-error {
  color: var(--sh-danger);
  font-size: 13px;
  padding: 8px 0;
}
.dir-name {
  font-weight: 500;
  color: var(--sh-text-primary);
}
.dir-code {
  font-family: var(--sh-font-mono, monospace);
  font-size: 12px;
  color: var(--sh-blue);
  background: var(--sh-code-bg, rgba(0,0,0,.04));
  padding: 1px 5px;
  border-radius: 3px;
}
.actions-wrap {
  display: flex;
  gap: 10px;
  padding: 12px 20px 20px;
  flex-wrap: wrap;
}
.link {
  color: var(--sh-blue);
  text-decoration: none;
}
.link:hover { text-decoration: underline; }
</style>
