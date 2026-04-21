<template>
  <div class="sh-shell" :class="{ 'sh-shell--collapsed': collapsed }">

    <!-- ══════════════ 侧边栏 ══════════════ -->
    <aside class="sh-aside">
      <!-- Logo -->
      <div class="sh-logo" :title="collapsed ? 'ServerHub' : ''">
        <div class="sh-logo-icon">
          <span class="sh-logo-icon__glyph">S</span>
        </div>
        <transition name="sh-fade">
          <span v-if="!collapsed" class="sh-logo-text">ServerHub</span>
        </transition>
      </div>

      <!-- 导航 -->
      <div class="sh-nav">
        <!-- 工作台 -->
        <SidebarItem
          to="/dashboard"
          label="工作台"
          :collapsed="collapsed"
          :active="route.path === '/dashboard'"
        >
          <template #icon><dashboard-icon /></template>
        </SidebarItem>

        <!-- 应用 -->
        <div class="sh-nav__group" v-show="!collapsed">应用</div>
        <SidebarItem
          v-for="app in appStore.apps"
          :key="app.id"
          :to="`/apps/${app.id}/overview`"
          :label="app.name"
          :collapsed="collapsed"
          :active="route.path.startsWith(`/apps/${app.id}`)"
          sub
        >
          <template #icon>
            <StatusDot :status="app.status" :size="8" :ring="false" />
          </template>
        </SidebarItem>
        <SidebarItem
          to="/apps/create"
          label="新建应用"
          :collapsed="collapsed"
          add
        >
          <template #icon><add-icon /></template>
        </SidebarItem>

        <!-- 服务器 -->
        <div class="sh-nav__group" v-show="!collapsed">服务器</div>
        <template v-for="server in serverStore.servers" :key="server.id">
          <div
            class="sh-nav__item sh-nav__item--server"
            :class="{ 'is-active': route.path.startsWith(`/servers/${server.id}`) }"
            @click="toggleServer(server.id)"
          >
            <StatusDot :status="server.status" :size="8" :ring="false" />
            <span v-show="!collapsed" class="sh-nav__label">{{ server.name }}</span>
            <chevron-right-icon
              v-show="!collapsed"
              class="sh-nav__chev"
              :class="{ 'is-open': expandedServers.has(server.id) }"
            />
          </div>
          <transition name="sh-expand">
            <div v-if="!collapsed && expandedServers.has(server.id)" class="sh-nav__sub">
              <router-link
                v-for="item in serverSubItems"
                :key="item.path"
                :to="`/servers/${server.id}/${item.path}`"
                class="sh-nav__item sh-nav__item--subsub"
                :class="{ 'is-active': route.path === `/servers/${server.id}/${item.path}` }"
              >
                <span class="sh-nav__label">{{ item.label }}</span>
              </router-link>
            </div>
          </transition>
        </template>
        <SidebarItem
          to="/servers"
          label="添加服务器"
          :collapsed="collapsed"
          add
        >
          <template #icon><add-icon /></template>
        </SidebarItem>

        <!-- 管理 -->
        <div class="sh-nav__group" v-show="!collapsed">管理</div>
        <SidebarItem to="/deploy" label="部署管理" :collapsed="collapsed" :active="route.path === '/deploy'">
          <template #icon><swap-icon /></template>
        </SidebarItem>
        <SidebarItem to="/database" label="数据库" :collapsed="collapsed" :active="route.path === '/database'">
          <template #icon><server-icon /></template>
        </SidebarItem>
        <SidebarItem
          to="/notifications"
          label="通知"
          :collapsed="collapsed"
          :active="route.path === '/notifications'"
          :badge="unreadCount"
        >
          <template #icon><notification-icon /></template>
        </SidebarItem>
        <SidebarItem to="/settings" label="设置" :collapsed="collapsed" :active="route.path === '/settings'">
          <template #icon><setting-icon /></template>
        </SidebarItem>
      </div>

      <!-- 底部 -->
      <div class="sh-aside__foot">
        <UiThemeToggle />
        <button class="sh-collapse-btn" @click="toggleCollapsed" :title="collapsed ? '展开' : '折叠'">
          <chevron-left-icon :style="{ transform: collapsed ? 'rotate(180deg)' : 'none' }" />
        </button>
      </div>
    </aside>

    <!-- ══════════════ 主体 ══════════════ -->
    <div class="sh-main">
      <header class="sh-header">
        <div class="sh-header__left">
          <t-breadcrumb v-if="breadcrumbs.length" class="sh-bc">
            <t-breadcrumb-item
              v-for="(crumb, i) in breadcrumbs"
              :key="i"
              :to="crumb.path || undefined"
            >{{ crumb.label }}</t-breadcrumb-item>
          </t-breadcrumb>
          <span v-else class="sh-header__title">工作台</span>
        </div>

        <div class="sh-header__right">
          <button class="sh-cmd" @click="cmdPaletteRef?.open()" title="命令面板">
            <search-icon class="sh-cmd__icon" />
            <span class="sh-cmd__text">搜索或执行…</span>
            <UiKbd>⌘K</UiKbd>
          </button>

          <UiIconButton variant="ghost" size="md" @click="router.push('/notifications')" :badge="unreadCount || ''">
            <notification-icon />
          </UiIconButton>

          <UiThemeToggle />

          <t-dropdown :options="userMenuOptions" @click="onUserMenu" trigger="click" placement="bottom-right">
            <button class="sh-user">
              <div class="sh-avatar">{{ avatarLetter }}</div>
              <span class="sh-user__name">{{ authStore.user?.username }}</span>
              <chevron-down-icon class="sh-user__chev" />
            </button>
          </t-dropdown>
        </div>
      </header>

      <main :class="['sh-content', isFullscreen && 'sh-content--fullscreen']">
        <router-view v-slot="{ Component, route: r }">
          <transition name="route-fade" mode="out-in">
            <component :is="Component" :key="r.fullPath" />
          </transition>
        </router-view>
      </main>
    </div>

    <ChangeProfile v-model:visible="profileDialogVisible" />
    <CommandPalette ref="cmdPaletteRef" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted, h } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import {
  DashboardIcon, AddIcon, ServerIcon, NotificationIcon, SettingIcon,
  SwapIcon, ChevronRightIcon, ChevronDownIcon, ChevronLeftIcon, SearchIcon,
} from 'tdesign-icons-vue-next'
import ChangeProfile from '@/views/Profile/ChangeProfile.vue'
import CommandPalette from '@/components/global/CommandPalette.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import UiIconButton from '@/components/ui/UiIconButton.vue'
import UiThemeToggle from '@/components/ui/UiThemeToggle.vue'
import UiKbd from '@/components/ui/UiKbd.vue'

// ─── Sidebar Item 内联组件 ───
const SidebarItem = (props: {
  to: string; label: string; collapsed?: boolean; active?: boolean;
  sub?: boolean; add?: boolean; badge?: number | string;
}, { slots }: any) => {
  const classes = [
    'sh-nav__item',
    props.sub && 'sh-nav__item--sub',
    props.add && 'sh-nav__item--add',
    props.active && 'is-active',
  ].filter(Boolean)
  return h(RouterLink, { to: props.to, class: classes, custom: false }, () => [
    h('span', { class: 'sh-nav__icon' }, slots.icon?.()),
    !props.collapsed && h('span', { class: 'sh-nav__label' }, props.label),
    !props.collapsed && props.badge ? h('span', { class: 'sh-nav__badge' }, String(props.badge)) : null,
  ])
}

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

// ── Collapse state ──
const COLLAPSE_KEY = 'sh-sidebar-collapsed'
const collapsed = ref<boolean>((() => {
  try { return localStorage.getItem(COLLAPSE_KEY) === '1' } catch { return false }
})())
function toggleCollapsed() {
  collapsed.value = !collapsed.value
  try { localStorage.setItem(COLLAPSE_KEY, collapsed.value ? '1' : '0') } catch {}
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

watch(() => route.params.serverId, (id) => {
  if (id) {
    const s = new Set(expandedServers.value)
    s.add(Number(id))
    expandedServers.value = s
  }
}, { immediate: true })

// ⌘K
function onGlobalKey(e: KeyboardEvent) {
  const isMod = e.metaKey || e.ctrlKey
  if (isMod && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    cmdPaletteRef.value?.open()
  }
}

onMounted(() => {
  serverStore.fetch()
  appStore.fetch()
  window.addEventListener('keydown', onGlobalKey)
})
</script>

<style scoped>
/* ══════ Shell ══════ */
.sh-shell {
  display: grid;
  grid-template-columns: var(--ui-sidebar-w) 1fr;
  height: 100vh;
  overflow: hidden;
  background: var(--ui-bg-canvas);
  transition: grid-template-columns var(--ui-dur-base) var(--ui-ease-standard);
}
.sh-shell--collapsed { grid-template-columns: var(--ui-sidebar-w-collapsed) 1fr; }

/* ══════ Sidebar ══════ */
.sh-aside {
  background: var(--ui-sidebar-bg);
  border-right: 1px solid var(--ui-sidebar-border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

.sh-logo {
  height: var(--ui-header-height);
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 14px;
  border-bottom: 1px solid var(--ui-sidebar-border);
  flex-shrink: 0;
}
.sh-logo-icon {
  width: 26px; height: 26px;
  background: var(--ui-brand-grad);
  border-radius: 7px;
  display: grid;
  place-items: center;
  font-size: 13px;
  font-weight: 800;
  color: #fff;
  flex-shrink: 0;
  box-shadow: 0 2px 6px rgba(94, 106, 210, .28), inset 0 1px 0 rgba(255,255,255,.2);
  animation: ui-grad-drift 12s ease-in-out infinite;
  background-size: 200% 200%;
}
.sh-logo-text {
  font-size: 13.5px;
  font-weight: 700;
  color: var(--ui-fg);
  letter-spacing: .2px;
}

.sh-nav {
  flex: 1;
  overflow-y: auto;
  padding: 8px 8px 16px;
  display: flex; flex-direction: column;
}
.sh-nav::-webkit-scrollbar { width: 4px; }

.sh-nav__group {
  font-size: 10.5px;
  color: var(--ui-sidebar-group);
  letter-spacing: .1em;
  text-transform: uppercase;
  padding: 12px 10px 4px;
  font-weight: 600;
}

.sh-nav__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 10px;
  height: 30px;
  border-radius: 6px;
  color: var(--ui-sidebar-fg);
  font-size: 12.5px;
  cursor: pointer;
  text-decoration: none;
  transition: background var(--ui-dur-fast) var(--ui-ease-standard),
              color var(--ui-dur-fast) var(--ui-ease-standard),
              transform var(--ui-dur-fast) var(--ui-ease-standard);
  position: relative;
  user-select: none;
  margin: 1px 0;
}
.sh-nav__item:hover {
  background: var(--ui-sidebar-hover);
  color: var(--ui-fg);
}
.sh-nav__item.is-active {
  color: var(--ui-sidebar-fg-active);
  background: var(--ui-sidebar-active-bg);
  font-weight: var(--ui-fw-medium);
}
.sh-nav__item.is-active::before {
  content: '';
  position: absolute;
  left: -8px; top: 6px; bottom: 6px;
  width: 3px;
  background: var(--ui-sidebar-active-line);
  border-radius: 0 3px 3px 0;
  animation: ui-slide-right var(--ui-dur-base) var(--ui-ease-standard);
}
.sh-nav__item--sub { padding-left: 14px; }
.sh-nav__item--subsub { padding-left: 34px; height: 28px; font-size: 12px; }
.sh-nav__item--server { padding-left: 14px; }
.sh-nav__item--add { opacity: .6; font-style: normal; }
.sh-nav__item--add:hover { opacity: 1; }

.sh-nav__icon {
  display: inline-flex; align-items: center; justify-content: center;
  width: 16px; height: 16px; font-size: 14px; flex-shrink: 0;
  color: var(--ui-fg-3);
}
.sh-nav__item.is-active .sh-nav__icon { color: var(--ui-brand); }

.sh-nav__label {
  flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.sh-nav__chev {
  font-size: 12px;
  opacity: .5;
  transition: transform var(--ui-dur-base) var(--ui-ease-standard);
  flex-shrink: 0;
}
.sh-nav__chev.is-open { transform: rotate(90deg); opacity: .9; }

.sh-nav__badge {
  background: var(--ui-danger);
  color: #fff;
  font-size: 10px;
  min-width: 16px; height: 16px; padding: 0 5px;
  border-radius: 8px;
  display: inline-flex; align-items: center; justify-content: center;
  font-weight: 600;
  line-height: 1;
  box-shadow: 0 0 0 2px var(--ui-sidebar-bg);
}

.sh-nav__sub { display: flex; flex-direction: column; gap: 1px; overflow: hidden; }

/* 折叠态 —— 图标居中、隐藏文字 */
.sh-shell--collapsed .sh-nav__item {
  justify-content: center;
  padding: 0;
  width: 40px; margin: 1px auto;
}
.sh-shell--collapsed .sh-nav__item.is-active::before { left: -4px; }
.sh-shell--collapsed .sh-logo { justify-content: center; padding: 0; }

/* Aside foot */
.sh-aside__foot {
  display: flex; align-items: center; justify-content: space-between;
  gap: 8px; padding: 10px 12px;
  border-top: 1px solid var(--ui-sidebar-border);
  flex-shrink: 0;
}
.sh-shell--collapsed .sh-aside__foot { flex-direction: column; }
.sh-collapse-btn {
  width: 26px; height: 26px;
  display: grid; place-items: center;
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-surface);
  color: var(--ui-fg-3);
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: all var(--ui-dur-fast) var(--ui-ease-standard);
}
.sh-collapse-btn:hover {
  color: var(--ui-brand);
  border-color: var(--ui-brand);
  transform: translateY(-1px);
  box-shadow: var(--ui-shadow-sm);
}

/* ══════ Main ══════ */
.sh-main {
  min-width: 0;
  display: flex; flex-direction: column;
  overflow: hidden;
}

/* ── Header 玻璃态 ── */
.sh-header {
  height: var(--ui-header-height);
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 16px;
  background: var(--ui-header-bg);
  backdrop-filter: var(--ui-header-blur);
  -webkit-backdrop-filter: var(--ui-header-blur);
  border-bottom: 1px solid var(--ui-header-border);
  flex-shrink: 0;
  position: relative;
  z-index: 10;
}

.sh-header__left { display: flex; align-items: center; min-width: 0; }
.sh-header__right { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.sh-header__title { font-size: 13px; font-weight: var(--ui-fw-semibold); color: var(--ui-fg); }

.sh-bc { font-size: 12.5px; }
.sh-bc :deep(.t-breadcrumb__item) { font-size: 12.5px; }

.sh-cmd {
  display: inline-flex; align-items: center; gap: 8px;
  height: 28px; padding: 0 8px 0 10px;
  margin-right: 4px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  background: var(--ui-bg-subtle);
  color: var(--ui-fg-3);
  cursor: pointer;
  transition: all var(--ui-dur-fast) var(--ui-ease-standard);
  min-width: 200px;
}
.sh-cmd:hover {
  border-color: var(--ui-brand);
  color: var(--ui-fg);
  background: var(--ui-bg-surface);
  box-shadow: 0 0 0 3px var(--ui-brand-ring);
}
.sh-cmd__icon { font-size: 13px; color: var(--ui-fg-4); }
.sh-cmd__text { font-size: 12px; flex: 1; text-align: left; }
@media (max-width: 900px) {
  .sh-cmd { min-width: 0; }
  .sh-cmd__text { display: none; }
}

.sh-user {
  display: inline-flex; align-items: center; gap: 8px;
  height: 30px; padding: 0 8px 0 4px;
  border: none; background: transparent;
  border-radius: var(--ui-radius-pill);
  cursor: pointer;
  transition: background var(--ui-dur-fast) var(--ui-ease-standard);
  margin-left: 4px;
}
.sh-user:hover { background: var(--ui-bg-hover); }
.sh-avatar {
  width: 24px; height: 24px;
  border-radius: 50%;
  background: var(--ui-brand-grad);
  background-size: 200% 200%;
  color: #fff;
  font-size: 11px; font-weight: 700;
  display: grid; place-items: center;
  flex-shrink: 0;
  box-shadow: 0 1px 3px rgba(94,106,210,.3);
  animation: ui-grad-drift 14s ease-in-out infinite;
}
.sh-user__name {
  font-size: 12.5px;
  color: var(--ui-fg-2);
  max-width: 100px;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.sh-user__chev { font-size: 12px; color: var(--ui-fg-4); }

/* ── Content ── */
.sh-content {
  background: var(--ui-bg-canvas);
  overflow-y: auto;
  flex: 1;
  position: relative;
}
.sh-content--fullscreen {
  overflow: hidden;
  display: flex; flex-direction: column;
}

/* ── 路由过渡 ── */
.route-fade-enter-active {
  animation: ui-slide-up var(--ui-dur-base) var(--ui-ease-standard);
}
.route-fade-leave-active {
  transition: opacity var(--ui-dur-fast) var(--ui-ease-standard);
}
.route-fade-leave-to { opacity: 0; }

/* ── 侧栏文字 fade ── */
.sh-fade-enter-active, .sh-fade-leave-active {
  transition: opacity var(--ui-dur-fast) var(--ui-ease-standard);
}
.sh-fade-enter-from, .sh-fade-leave-to { opacity: 0; }

/* ── 子菜单展开动画 ── */
.sh-expand-enter-active, .sh-expand-leave-active {
  transition: max-height var(--ui-dur-base) var(--ui-ease-standard), opacity var(--ui-dur-fast);
  overflow: hidden;
  max-height: 400px;
}
.sh-expand-enter-from, .sh-expand-leave-to { max-height: 0; opacity: 0; }
</style>
