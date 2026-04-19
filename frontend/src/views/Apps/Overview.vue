<template>
  <div class="app-overview">
    <el-descriptions :column="2" border class="desc-block">
      <el-descriptions-item label="描述">{{ app?.description || '—' }}</el-descriptions-item>
      <el-descriptions-item label="状态">
        <el-tag :type="statusType" size="small">{{ app?.status ?? '—' }}</el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="域名">{{ app?.domain || '—' }}</el-descriptions-item>
      <el-descriptions-item label="所属服务器">
        <router-link v-if="server" :to="`/servers/${server.id}/overview`" class="link">{{ server.name }} ({{ server.host }})</router-link>
        <span v-else>—</span>
      </el-descriptions-item>
      <el-descriptions-item label="Nginx 站点">
        <router-link v-if="app?.site_name && server" :to="`/servers/${server.id}/nginx`" class="link">{{ app.site_name }}</router-link>
        <span v-else>{{ app?.site_name || '未关联' }}</span>
      </el-descriptions-item>
      <el-descriptions-item label="容器名">
        <router-link v-if="app?.container_name && server" :to="`/servers/${server.id}/docker`" class="link">{{ app.container_name }}</router-link>
        <span v-else>{{ app?.container_name || '未关联' }}</span>
      </el-descriptions-item>
      <el-descriptions-item label="创建时间">{{ app?.created_at }}</el-descriptions-item>
      <el-descriptions-item label="最后更新">{{ app?.updated_at }}</el-descriptions-item>
    </el-descriptions>

    <div class="quick-links">
      <el-button v-if="server" @click="$router.push(`/servers/${server.id}/terminal`)">打开终端</el-button>
      <el-button v-if="server" @click="$router.push(`/servers/${server.id}/files`)">文件管理</el-button>
      <el-button type="danger" plain @click="handleDelete">删除应用</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
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

const statusType = computed(() => {
  const s = app.value?.status
  if (s === 'online') return 'success'
  if (s === 'offline' || s === 'error') return 'danger'
  return 'info'
})

async function handleDelete() {
  await ElMessageBox.confirm(`确认删除应用「${app.value?.name}」？`, '危险操作', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
  try {
    await deleteApp(appId.value)
    ElMessage.success('已删除')
    await appStore.fetch()
    router.push('/dashboard')
  } catch { ElMessage.error('删除失败') }
}

onMounted(async () => {
  if (!appStore.apps.length) await appStore.fetch()
  if (!serverStore.servers.length) await serverStore.fetch()
})
</script>

<style scoped>
.app-overview { padding: 4px 0; }
.desc-block { margin-bottom: 20px; }
.quick-links { display: flex; gap: 8px; flex-wrap: wrap; }
.link { color: #409eff; text-decoration: none; }
.link:hover { text-decoration: underline; }
</style>
