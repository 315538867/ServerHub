<template>
  <el-container class="layout-container">
    <el-aside width="220px" class="layout-aside">
      <div class="logo">
        <span class="logo-text">ServerHub</span>
      </div>
      <el-scrollbar class="menu-scrollbar">
        <el-menu
          :default-active="activeMenu"
          router
          background-color="#1a1a2e"
          text-color="#c0c4cc"
          active-text-color="#409eff"
        >
          <el-menu-item index="/dashboard">
            <el-icon><Monitor /></el-icon>
            <span>工作台</span>
          </el-menu-item>

          <el-sub-menu index="apps">
            <template #title>
              <el-icon><Box /></el-icon>
              <span>应用</span>
            </template>
            <el-menu-item
              v-for="app in appStore.apps"
              :key="app.id"
              :index="`/apps/${app.id}/overview`"
            >
              <span class="status-dot" :class="app.status" />
              {{ app.name }}
            </el-menu-item>
            <el-menu-item index="/apps/create" class="add-item">
              <el-icon><Plus /></el-icon>
              <span>新建应用</span>
            </el-menu-item>
          </el-sub-menu>

          <el-sub-menu index="servers">
            <template #title>
              <el-icon><Connection /></el-icon>
              <span>服务器</span>
            </template>
            <el-sub-menu
              v-for="server in serverStore.servers"
              :key="server.id"
              :index="`server-${server.id}`"
            >
              <template #title>
                <span class="status-dot" :class="server.status" />
                {{ server.name }}
              </template>
              <el-menu-item :index="`/servers/${server.id}/overview`">概览</el-menu-item>
              <el-menu-item :index="`/servers/${server.id}/nginx`">Nginx 网关</el-menu-item>
              <el-menu-item :index="`/servers/${server.id}/docker`">Docker</el-menu-item>
              <el-menu-item :index="`/servers/${server.id}/system`">系统</el-menu-item>
              <el-menu-item :index="`/servers/${server.id}/files`">文件</el-menu-item>
              <el-menu-item :index="`/servers/${server.id}/terminal`">终端</el-menu-item>
            </el-sub-menu>
          </el-sub-menu>

          <el-menu-item index="/notifications">
            <el-icon><Bell /></el-icon>
            <span>通知</span>
          </el-menu-item>

          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>设置</span>
          </el-menu-item>
        </el-menu>
      </el-scrollbar>
    </el-aside>

    <el-container>
      <el-header class="layout-header">
        <div class="header-right">
          <span class="username">{{ authStore.user?.username }}</span>
          <el-button link @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-main :class="['layout-main', isFullscreen ? 'layout-main--fullscreen' : '']">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import {
  Monitor, Connection, Box, Bell, Setting, Plus,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const serverStore = useServerStore()
const appStore = useAppStore()

const activeMenu = computed(() => {
  const path = route.path
  const appMatch = path.match(/^\/apps\/(\d+)/)
  if (appMatch) return `/apps/${appMatch[1]}/overview`
  return path
})

const isFullscreen = computed(() => route.path.endsWith('/terminal'))

onMounted(() => {
  serverStore.fetch()
  appStore.fetch()
})

async function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout-container { height: 100vh; }
.layout-aside {
  background-color: #1a1a2e;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid #2a2a4a;
  flex-shrink: 0;
}
.logo-text {
  color: #409eff;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 1px;
}
.menu-scrollbar { flex: 1; }
.layout-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  border-bottom: 1px solid var(--el-border-color-light);
  background: #fff;
}
.header-right { display: flex; align-items: center; gap: 12px; }
.username { color: var(--el-text-color-regular); font-size: 14px; }
.layout-main {
  background: #f5f7fa;
  overflow-y: auto;
}
.layout-main--fullscreen {
  padding: 0 !important;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  background: #909399;
}
.status-dot.online { background: #67c23a; }
.status-dot.offline { background: #f56c6c; }
.status-dot.unknown { background: #909399; }
.status-dot.error { background: #e6a23c; }
.add-item { font-style: italic; opacity: 0.7; }
.add-item:hover { opacity: 1; }
</style>
