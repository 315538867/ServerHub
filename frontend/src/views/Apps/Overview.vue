<template>
  <div class="app-overview">
    <t-descriptions :column="2" bordered style="margin-bottom:20px">
      <t-descriptions-item label="描述">{{ app?.description || '—' }}</t-descriptions-item>
      <t-descriptions-item label="状态">
        <t-tag :theme="statusTheme" variant="light" size="small">{{ app?.status ?? '—' }}</t-tag>
      </t-descriptions-item>
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
      <t-descriptions-item label="创建时间">{{ app?.created_at }}</t-descriptions-item>
      <t-descriptions-item label="最后更新">{{ app?.updated_at }}</t-descriptions-item>
    </t-descriptions>

    <t-space>
      <t-button v-if="server" variant="outline" @click="$router.push(`/servers/${server.id}/terminal`)">打开终端</t-button>
      <t-button v-if="server" variant="outline" @click="$router.push(`/servers/${server.id}/files`)">文件管理</t-button>
      <t-button theme="danger" variant="outline" @click="handleDelete">删除应用</t-button>
    </t-space>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useAppStore } from '@/stores/app'
import { useServerStore } from '@/stores/server'
import { deleteApp } from '@/api/application'

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
})
</script>

<style scoped>
.app-overview { padding: 4px 0; }
.link { color: var(--td-brand-color); text-decoration: none; }
.link:hover { text-decoration: underline; }
</style>
