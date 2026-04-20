<template>
  <t-layout class="sh-layout">

    <!-- ══════════════ 侧边栏 ══════════════ -->
    <t-aside class="sh-aside" width="208px">
      <!-- Logo -->
      <div class="sh-logo">
        <div class="sh-logo-icon">S</div>
        <span class="sh-logo-text">ServerHub</span>
      </div>

      <!-- 导航菜单 -->
      <div class="sh-nav">
        <!-- 工作台 -->
        <router-link to="/dashboard" class="sh-nav-item" :class="{ active: route.path === '/dashboard' }">
          <dashboard-icon class="sh-nav-icon" />
          <span>工作台</span>
        </router-link>

        <!-- 应用 -->
        <div class="sh-nav-group-label">应用</div>
        <router-link
          v-for="app in appStore.apps"
          :key="app.id"
          :to="`/apps/${app.id}/overview`"
          class="sh-nav-item sh-nav-item--sub"
          :class="{ active: route.path.startsWith(`/apps/${app.id}`) }"
        >
          <span class="status-dot" :class="app.status" />
          <span class="sh-nav-sub-text">{{ app.name }}</span>
        </router-link>
        <router-link to="/apps/create" class="sh-nav-item sh-nav-item--add">
          <add-icon class="sh-nav-icon" />
          <span>新建应用</span>
        </router-link>

        <!-- 服务器 -->
        <div class="sh-nav-group-label">服务器</div>
        <template v-for="server in serverStore.servers" :key="server.id">
          <div
            class="sh-nav-item sh-nav-item--server"
            :class="{ active: route.path.startsWith(`/servers/${server.id}`) }"
            @click="toggleServer(server.id)"
          >
            <span class="status-dot" :class="server.status" />
            <span class="sh-nav-sub-text">{{ server.name }}</span>
            <chevron-right-icon
              class="sh-nav-chevron"
              :class="{ expanded: expandedServers.has(server.id) }"
            />
          </div>
          <template v-if="expandedServers.has(server.id)">
            <router-link
              v-for="item in serverSubItems"
              :key="item.path"
              :to="`/servers/${server.id}/${item.path}`"
              class="sh-nav-item sh-nav-item--subsub"
              :class="{ active: route.path === `/servers/${server.id}/${item.path}` }"
            >
              {{ item.label }}
            </router-link>
          </template>
        </template>

        <!-- 全局管理 -->
        <div class="sh-nav-group-label">管理</div>
        <router-link to="/deploy" class="sh-nav-item" :class="{ active: route.path === '/deploy' }">
          <swap-icon class="sh-nav-icon" />
          <span>部署管理</span>
        </router-link>
        <router-link to="/database" class="sh-nav-item" :class="{ active: route.path === '/database' }">
          <server-icon class="sh-nav-icon" />
          <span>数据库</span>
        </router-link>
        <router-link to="/notifications" class="sh-nav-item" :class="{ active: route.path === '/notifications' }">
          <notification-icon class="sh-nav-icon" />
          <span>通知</span>
          <span v-if="unreadCount" class="sh-badge">{{ unreadCount }}</span>
        </router-link>
        <router-link to="/settings" class="sh-nav-item" :class="{ active: route.path === '/settings' }">
          <setting-icon class="sh-nav-icon" />
          <span>设置</span>
        </router-link>
      </div>
    </t-aside>

    <!-- ══════════════ 主体区域 ══════════════ -->
    <t-layout class="sh-main-layout">

      <!-- 顶部 Header -->
      <t-header class="sh-header">
        <div class="sh-header-left">
          <t-breadcrumb v-if="breadcrumbs.length" class="sh-breadcrumb">
            <t-breadcrumb-item
              v-for="(crumb, i) in breadcrumbs"
              :key="i"
              :to="crumb.path || undefined"
            >{{ crumb.label }}</t-breadcrumb-item>
          </t-breadcrumb>
          <span v-else class="sh-header-title">工作台</span>
        </div>
        <div class="sh-header-right">
          <t-tooltip content="命令面板 (⌘K / Ctrl+K)" placement="bottom">
            <div class="sh-cmd-trigger" @click="cmdPaletteRef?.open()">
              <span class="sh-cmd-icon">⌘</span>
              <span class="sh-cmd-text">搜索或执行…</span>
              <kbd class="sh-cmd-kbd">⌘K</kbd>
            </div>
          </t-tooltip>
          <t-tooltip content="通知" placement="bottom">
            <div class="sh-header-btn" @click="router.push('/notifications')">
              <notification-icon />
              <span v-if="unreadCount" class="sh-header-badge">{{ unreadCount }}</span>
            </div>
          </t-tooltip>
          <t-dropdown :options="userMenuOptions" @click="onUserMenu" trigger="click" placement="bottom-right">
            <div class="sh-user-btn">
              <div class="sh-avatar">{{ avatarLetter }}</div>
              <span class="sh-username">{{ authStore.user?.username }}</span>
              <chevron-down-icon style="font-size:14px;color:#666" />
            </div>
          </t-dropdown>
        </div>
      </t-header>

      <!-- 内容区 -->
      <t-content :class="['sh-content', isFullscreen && 'sh-content--fullscreen']">
        <router-view />
      </t-content>

    </t-layout>
  </t-layout>

  <ChangeProfile v-model:visible="profileDialogVisible" />
  <CommandPalette ref="cmdPaletteRef" />
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import {
  DashboardIcon, AddIcon, ServerIcon, NotificationIcon, SettingIcon,
  SwapIcon, ChevronRightIcon, ChevronDownIcon,
} from 'tdesign-icons-vue-next'
import ChangeProfile from '@/views/Profile/ChangeProfile.vue'
import CommandPalette from '@/components/global/CommandPalette.vue'

const cmdPaletteRef = ref<InstanceType<typeof CommandPalette> | null>(null)

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const serverStore = useServerStore()
const appStore = useAppStore()

const serverSubItems = [
  { path: 'overview', label: '概览' },
  { path: 'nginx',    label: 'Nginx 网关' },
  { path: 'docker',   label: 'Docker' },
  { path: 'system',   label: '系统' },
  { path: 'files',    label: '文件' },
  { path: 'terminal', label: '终端' },
]

const expandedServers = ref<Set<number>>(new Set())

function toggleServer(id: number) {
  const s = new Set(expandedServers.value)
  if (s.has(id)) { s.delete(id) } else { s.add(id) }
  expandedServers.value = s
}

const isFullscreen = computed(() => route.path.endsWith('/terminal'))
const avatarLetter = computed(() => (authStore.user?.username?.[0] ?? 'U').toUpperCase())
const unreadCount = computed(() => 0)

const breadcrumbs = computed(() => {
  const path = route.path
  if (path === '/dashboard') return []
  if (route.params.appId) {
    const appId = Number(route.params.appId)
    const app = appStore.getById(appId)
    const tab = path.split('/').pop()
    const tabLabel: Record<string, string> = {
      overview: '概览', domain: '域名', service: '服务',
      deploy: '部署', logs: '日志', database: '数据库', env: '环境变量',
    }
    return [
      { label: '应用', path: '/apps' },
      { label: app?.name ?? `应用 ${appId}`, path: `/apps/${appId}/overview` },
      ...(tab && tabLabel[tab] ? [{ label: tabLabel[tab], path: '' }] : []),
    ]
  }
  if (route.params.serverId) {
    const sid = Number(route.params.serverId)
    const srv = serverStore.getById(sid)
    const seg = path.split('/').pop()
    const segLabel: Record<string, string> = {
      overview: '概览', nginx: 'Nginx', docker: 'Docker',
      system: '系统', files: '文件', terminal: '终端',
    }
    return [
      { label: '服务器', path: '/servers' },
      { label: srv?.name ?? `服务器 ${sid}`, path: `/servers/${sid}/overview` },
      ...(seg && segLabel[seg] ? [{ label: segLabel[seg], path: '' }] : []),
    ]
  }
  if (path === '/apps/create') {
    return [
      { label: '应用', path: '/apps' },
      { label: '新建应用', path: '' },
    ]
  }
  const topLabels: Record<string, string> = {
    '/servers': '服务器管理', '/deploy': '部署管理',
    '/database': '数据库', '/notifications': '通知',
    '/settings': '设置', '/apps': '应用',
  }
  return topLabels[path] ? [{ label: topLabels[path], path: '' }] : []
})

const profileDialogVisible = ref(false)
const userMenuOptions = [
  { content: '账号设置', value: 'profile' },
  { content: '退出登录', value: 'logout' },
]

function onUserMenu(item: { value: string }) {
  if (item.value === 'profile') {
    profileDialogVisible.value = true
  } else if (item.value === 'logout') {
    authStore.logout()
    router.push('/login')
  }
}

// Auto-expand server nav when navigating to a server route
watch(() => route.params.serverId, (id) => {
  if (id) {
    const s = new Set(expandedServers.value)
    s.add(Number(id))
    expandedServers.value = s
  }
}, { immediate: true })

onMounted(() => {
  serverStore.fetch()
  appStore.fetch()
})
</script>

<style scoped>
.sh-layout { height: 100vh; overflow: hidden; }

/* ── Sidebar ── */
.sh-aside {
  background: var(--sh-sidebar-bg) !important;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 0;
}

.sh-logo {
  height: var(--sh-header-height);
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: 0 var(--sh-space-lg);
  border-bottom: 1px solid rgba(255,255,255,.06);
  flex-shrink: 0;
}
.sh-logo-icon {
  width: 28px;
  height: 28px;
  background: var(--sh-blue);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
}
.sh-logo-text {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  letter-spacing: .5px;
}

.sh-nav {
  flex: 1;
  overflow-y: auto;
  padding: var(--sh-space-sm) 0 var(--sh-space-lg);
}
.sh-nav::-webkit-scrollbar { width: 0; }

.sh-nav-group-label {
  font-size: 11px;
  color: var(--sh-sidebar-group);
  letter-spacing: .08em;
  text-transform: uppercase;
  padding: var(--sh-space-md) var(--sh-space-lg) var(--sh-space-xs);
  font-weight: 500;
}

.sh-nav-item {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: 0 var(--sh-space-lg);
  height: 38px;
  color: var(--sh-sidebar-text);
  font-size: 13.5px;
  cursor: pointer;
  text-decoration: none;
  transition: background .12s, color .12s;
  position: relative;
  user-select: none;
}
.sh-nav-item:hover { background: var(--sh-sidebar-hover); color: #fff; }
.sh-nav-item.active {
  color: #fff;
  background: rgba(0, 82, 217, 0.28);
}
.sh-nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 7px;
  bottom: 7px;
  width: 3px;
  background: var(--sh-blue);
  border-radius: 0 2px 2px 0;
}

.sh-nav-icon { font-size: 15px; flex-shrink: 0; }

.sh-nav-item--sub    { padding-left: var(--sh-space-xl); height: 34px; font-size: 13px; }
.sh-nav-item--subsub { padding-left: var(--sh-space-xl); height: 32px; font-size: 12.5px; }
.sh-nav-item--server { cursor: pointer; padding-left: var(--sh-space-xl); height: 34px; font-size: 13px; }
.sh-nav-item--add {
  padding-left: var(--sh-space-xl);
  height: 32px;
  font-size: 12.5px;
  opacity: .55;
}
.sh-nav-item--add:hover { opacity: 1; }

.sh-nav-sub-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sh-nav-chevron {
  font-size: 13px;
  opacity: .45;
  transition: transform .2s;
  flex-shrink: 0;
}
.sh-nav-chevron.expanded { transform: rotate(90deg); }

.sh-badge {
  background: var(--sh-red);
  color: #fff;
  font-size: 10px;
  min-width: 16px;
  height: 16px;
  border-radius: 8px;
  padding: 0 var(--sh-space-xs);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

/* ── Header ── */
.sh-main-layout { overflow: hidden; display: flex; flex-direction: column; }
.sh-header {
  height: var(--sh-header-height) !important;
  min-height: var(--sh-header-height) !important;
  background: var(--sh-header-bg) !important;
  border-bottom: var(--sh-header-border);
  display: flex !important;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--sh-space-lg) !important;
  flex-shrink: 0;
  box-shadow: 0 1px 4px rgba(0,0,0,.04);
}

.sh-header-left  { display: flex; align-items: center; }
.sh-header-right { display: flex; align-items: center; gap: var(--sh-space-xs); }

.sh-cmd-trigger {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-sm) var(--sh-space-sm) var(--sh-space-sm) var(--sh-space-md);
  margin-right: var(--sh-space-sm);
  border: 1px solid var(--sh-border);
  border-radius: 8px;
  background: var(--sh-bg);
  color: var(--sh-text-secondary);
  cursor: pointer;
  transition: all 0.12s;
  user-select: none;
}
.sh-cmd-trigger:hover {
  border-color: var(--sh-blue);
  color: var(--sh-text-primary);
}
.sh-cmd-icon { font-weight: 600; font-size: 13px; }
.sh-cmd-text { font-size: 12px; }
.sh-cmd-kbd {
  font-size: 10px;
  padding: 1px 6px;
  border: 1px solid var(--sh-border);
  border-radius: 3px;
  background: var(--sh-card-bg);
  color: var(--sh-text-secondary);
}
@media (max-width: 720px) {
  .sh-cmd-trigger .sh-cmd-text { display: none; }
}
.sh-header-title { font-size: 14px; font-weight: 600; color: var(--sh-text-primary); }
.sh-breadcrumb :deep(.t-breadcrumb__item) { font-size: 13px; }

.sh-header-btn {
  position: relative;
  width: 36px;
  height: 36px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #555;
  font-size: 18px;
  transition: background .12s;
}
.sh-header-btn:hover { background: #f2f3f5; color: #0d0d0d; }

.sh-header-badge {
  position: absolute;
  top: 4px;
  right: 4px;
  background: var(--sh-red);
  color: #fff;
  font-size: 10px;
  min-width: 14px;
  height: 14px;
  border-radius: 7px;
  padding: 0 var(--sh-space-xs);
  display: flex;
  align-items: center;
  justify-content: center;
}

.sh-user-btn {
  display: flex;
  align-items: center;
  gap: var(--sh-space-sm);
  padding: var(--sh-space-xs) var(--sh-space-sm) var(--sh-space-xs) var(--sh-space-sm);
  border-radius: 20px;
  cursor: pointer;
  transition: background .12s;
  margin-left: var(--sh-space-xs);
}
.sh-user-btn:hover { background: #f2f3f5; }

.sh-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: linear-gradient(135deg, #0052d9 0%, #1a66e8 100%);
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.sh-username {
  font-size: 13px;
  color: var(--sh-text-primary);
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── Content ── */
.sh-content {
  background: var(--sh-page-bg) !important;
  overflow-y: auto;
  flex: 1;
}
.sh-content--fullscreen {
  overflow: hidden !important;
  display: flex;
  flex-direction: column;
}
</style>
