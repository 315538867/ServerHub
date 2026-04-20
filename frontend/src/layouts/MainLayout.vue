<template>
  <t-layout class="layout-root">
    <t-aside class="layout-aside" width="220px">
      <div class="logo">
        <span class="logo-text">ServerHub</span>
      </div>
      <t-menu
        :value="activeMenu"
        theme="dark"
        class="side-menu"
        :collapsed="false"
        @change="onMenuChange"
      >
        <t-menu-item value="/dashboard">
          <template #icon><dashboard-icon /></template>
          工作台
        </t-menu-item>

        <t-submenu value="apps">
          <template #icon><app-icon /></template>
          <template #title>应用</template>
          <t-menu-item
            v-for="app in appStore.apps"
            :key="app.id"
            :value="`/apps/${app.id}/overview`"
          >
            <span class="status-dot" :class="app.status" />
            {{ app.name }}
          </t-menu-item>
          <t-menu-item value="/apps/create" class="add-item">
            <template #icon><add-icon /></template>
            新建应用
          </t-menu-item>
        </t-submenu>

        <t-submenu value="servers">
          <template #icon><server-icon /></template>
          <template #title>服务器</template>
          <t-submenu
            v-for="server in serverStore.servers"
            :key="server.id"
            :value="`server-${server.id}`"
          >
            <template #title>
              <span class="status-dot" :class="server.status" />
              {{ server.name }}
            </template>
            <t-menu-item :value="`/servers/${server.id}/overview`">概览</t-menu-item>
            <t-menu-item :value="`/servers/${server.id}/nginx`">Nginx 网关</t-menu-item>
            <t-menu-item :value="`/servers/${server.id}/docker`">Docker</t-menu-item>
            <t-menu-item :value="`/servers/${server.id}/system`">系统</t-menu-item>
            <t-menu-item :value="`/servers/${server.id}/files`">文件</t-menu-item>
            <t-menu-item :value="`/servers/${server.id}/terminal`">终端</t-menu-item>
          </t-submenu>
        </t-submenu>

        <t-menu-item value="/notifications">
          <template #icon><notification-icon /></template>
          通知
        </t-menu-item>

        <t-menu-item value="/settings">
          <template #icon><setting-icon /></template>
          设置
        </t-menu-item>
      </t-menu>
    </t-aside>

    <t-layout>
      <t-header class="layout-header">
        <div class="header-right">
          <span class="username">{{ authStore.user?.username }}</span>
          <t-button variant="text" @click="handleLogout">退出</t-button>
        </div>
      </t-header>
      <t-content :class="['layout-main', isFullscreen ? 'layout-main--fullscreen' : '']">
        <router-view />
      </t-content>
    </t-layout>
  </t-layout>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import {
  DashboardIcon, AppIcon, AddIcon, ServerIcon, NotificationIcon, SettingIcon,
} from 'tdesign-icons-vue-next'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const serverStore = useServerStore()
const appStore = useAppStore()

const activeMenu = computed(() => {
  const path = route.path
  const appMatch = path.match(/^\/apps\/(\d+)/)
  if (appMatch) return `/apps/${appMatch[1]}/overview`
  const serverMatch = path.match(/^\/servers\/(\d+)\/(.+)$/)
  if (serverMatch) return `/servers/${serverMatch[1]}/${serverMatch[2]}`
  return path
})

const isFullscreen = computed(() => route.path.endsWith('/terminal'))

function onMenuChange(value: string) {
  router.push(value)
}

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
.layout-root { height: 100vh; }
.layout-aside {
  background: #1a2332 !important;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid rgba(255,255,255,.08);
  flex-shrink: 0;
}
.logo-text {
  color: #4285f4;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 1px;
}
.side-menu {
  flex: 1;
  background: #1a2332 !important;
  border-right: none !important;
  overflow-y: auto;
}
.layout-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 24px;
  border-bottom: 1px solid var(--td-component-border);
  background: #fff;
  height: 56px;
}
.header-right { display: flex; align-items: center; gap: 12px; }
.username { color: var(--td-text-color-secondary); font-size: 14px; }
.layout-main {
  background: #f0f2f5;
  overflow-y: auto;
  padding: 20px;
}
.layout-main--fullscreen {
  padding: 0 !important;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.status-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  margin-right: 6px;
  background: #8a94a6;
  flex-shrink: 0;
}
.status-dot.online { background: #00a870; }
.status-dot.offline { background: #e34d59; }
.status-dot.unknown { background: #8a94a6; }
.status-dot.error { background: #ed7b2f; }
.add-item { opacity: 0.75; }
.add-item:hover { opacity: 1; }
</style>
