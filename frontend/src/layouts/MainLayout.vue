<template>
  <div class="shell" :class="{ 'shell--collapsed': collapsed }">
    <!-- ══════════ 侧边栏 ══════════ -->
    <aside class="aside">
      <!-- Logo -->
      <div class="logo" :title="collapsed ? 'ServerHub' : ''">
        <div class="logo__mark">S</div>
        <transition name="fade">
          <span v-if="!collapsed" class="logo__text">ServerHub</span>
        </transition>
      </div>

      <!-- 导航 -->
      <nav class="nav">
        <NavLink to="/dashboard" label="工作台" :collapsed="collapsed">
          <template #icon><LayoutDashboard :size="16" /></template>
        </NavLink>

        <div v-show="!collapsed" class="nav__group">应用</div>
        <NavLink
          v-for="app in appStore.apps"
          :key="`app-${app.id}`"
          :to="`/apps/${app.id}/overview`"
          :label="app.name"
          :collapsed="collapsed"
          indent
          :match="`/apps/${app.id}`"
        >
          <template #icon><StatusDot :status="app.status" :size="8" :ring="false" /></template>
        </NavLink>
        <NavLink to="/apps/create" label="新建应用" :collapsed="collapsed" muted>
          <template #icon><Plus :size="16" /></template>
        </NavLink>

        <div v-show="!collapsed" class="nav__group">服务器</div>
        <template v-for="server in serverStore.servers" :key="`srv-${server.id}`">
          <button
            class="nav__row nav__row--expandable"
            :class="{ 'is-active': route.path.startsWith(`/servers/${server.id}`) }"
            @click="toggleServer(server.id)"
          >
            <span class="nav__icon"><StatusDot :status="server.status" :size="8" :ring="false" /></span>
            <span v-show="!collapsed" class="nav__label">{{ server.name }}</span>
            <ChevronRight
              v-show="!collapsed"
              :size="14"
              class="nav__chev"
              :class="{ 'is-open': expandedServers.has(server.id) }"
            />
          </button>
          <transition name="expand">
            <div v-if="!collapsed && expandedServers.has(server.id)" class="nav__sub">
              <RouterLink
                v-for="item in serverSubItems"
                :key="item.path"
                :to="`/servers/${server.id}/${item.path}`"
                class="nav__row nav__row--sub"
                :class="{ 'is-active': route.path === `/servers/${server.id}/${item.path}` }"
              >
                <span class="nav__label">{{ item.label }}</span>
              </RouterLink>
            </div>
          </transition>
        </template>
        <NavLink to="/servers" label="添加服务器" :collapsed="collapsed" muted>
          <template #icon><Plus :size="16" /></template>
        </NavLink>

        <div v-show="!collapsed" class="nav__group">管理</div>
        <NavLink to="/deploy" label="部署管理" :collapsed="collapsed">
          <template #icon><GitBranch :size="16" /></template>
        </NavLink>
        <NavLink to="/database" label="数据库" :collapsed="collapsed">
          <template #icon><Database :size="16" /></template>
        </NavLink>
        <NavLink to="/notifications" label="通知" :collapsed="collapsed" :badge="unreadCount || undefined">
          <template #icon><Bell :size="16" /></template>
        </NavLink>
        <NavLink to="/settings" label="设置" :collapsed="collapsed">
          <template #icon><Settings :size="16" /></template>
        </NavLink>
      </nav>

      <div class="aside__foot">
        <UiThemeToggle />
        <button class="collapse-btn" @click="toggleCollapsed" :title="collapsed ? '展开' : '折叠'">
          <ChevronLeft :size="14" :style="{ transform: collapsed ? 'rotate(180deg)' : 'none' }" />
        </button>
      </div>
    </aside>

    <!-- ══════════ 主体 ══════════ -->
    <div class="main">
      <header class="header">
        <div class="header__left">
          <nav v-if="breadcrumbs.length" class="bc" aria-label="breadcrumb">
            <template v-for="(crumb, i) in breadcrumbs" :key="i">
              <RouterLink v-if="crumb.path" :to="crumb.path" class="bc__item">{{ crumb.label }}</RouterLink>
              <span v-else class="bc__item bc__item--current">{{ crumb.label }}</span>
              <span v-if="i < breadcrumbs.length - 1" class="bc__sep">/</span>
            </template>
          </nav>
          <span v-else class="header__title">工作台</span>
        </div>

        <div class="header__right">
          <button class="cmd" @click="cmdPaletteRef?.open()" title="命令面板 ⌘K">
            <Search :size="14" class="cmd__icon" />
            <span class="cmd__text">搜索或执行…</span>
            <UiKbd>⌘K</UiKbd>
          </button>

          <UiIconButton variant="ghost" size="md" @click="router.push('/notifications')" :badge="unreadCount || ''">
            <Bell :size="16" />
          </UiIconButton>

          <NDropdown :options="userMenuOptions" @select="onUserMenu" trigger="click" placement="bottom-end">
            <button class="user">
              <div class="user__avatar">{{ avatarLetter }}</div>
              <span class="user__name">{{ authStore.user?.username }}</span>
              <ChevronDown :size="14" class="user__chev" />
            </button>
          </NDropdown>
        </div>
      </header>

      <main :class="['content', isFullscreen && 'content--fullscreen']">
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
import { NDropdown } from 'naive-ui'
import {
  LayoutDashboard, Plus, GitBranch, Database, Bell, Settings,
  ChevronRight, ChevronDown, ChevronLeft, Search,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useServerStore } from '@/stores/server'
import { useAppStore } from '@/stores/app'
import ChangeProfile from '@/views/Profile/ChangeProfile.vue'
import CommandPalette from '@/components/global/CommandPalette.vue'
import StatusDot from '@/components/ui/StatusDot.vue'
import UiIconButton from '@/components/ui/UiIconButton.vue'
import UiThemeToggle from '@/components/ui/UiThemeToggle.vue'
import UiKbd from '@/components/ui/UiKbd.vue'

// 内联导航项 —— 活跃态自动匹配前缀
const NavLink = (props: {
  to: string; label: string; collapsed?: boolean; indent?: boolean; muted?: boolean;
  badge?: number | string; match?: string;
}, { slots }: any) => {
  const active = props.match ? route.path.startsWith(props.match) : route.path === props.to
  const classes = [
    'nav__row',
    props.indent && 'nav__row--indent',
    props.muted && 'nav__row--muted',
    active && 'is-active',
  ].filter(Boolean)
  return h(RouterLink, { to: props.to, class: classes }, () => [
    h('span', { class: 'nav__icon' }, slots.icon?.()),
    !props.collapsed && h('span', { class: 'nav__label' }, props.label),
    !props.collapsed && props.badge ? h('span', { class: 'nav__badge' }, String(props.badge)) : null,
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
  s.has(id) ? s.delete(id) : s.add(id)
  expandedServers.value = s
}

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

const breadcrumbs = computed<Array<{ label: string; path?: string }>>(() => {
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
      ...(tab && tabLabel[tab] ? [{ label: tabLabel[tab] }] : []),
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
      ...(seg && segLabel[seg] ? [{ label: segLabel[seg] }] : []),
    ]
  }
  if (path === '/apps/create') {
    return [
      { label: '应用', path: '/apps' },
      { label: '新建应用' },
    ]
  }
  const topLabels: Record<string, string> = {
    '/servers': '服务器管理', '/deploy': '部署管理',
    '/database': '数据库', '/notifications': '通知',
    '/settings': '设置', '/apps': '应用',
  }
  return topLabels[path] ? [{ label: topLabels[path] }] : []
})

const profileDialogVisible = ref(false)
const userMenuOptions = [
  { label: '账号设置', key: 'profile' },
  { label: '退出登录', key: 'logout' },
]
function onUserMenu(key: string) {
  if (key === 'profile') profileDialogVisible.value = true
  else if (key === 'logout') { authStore.logout(); router.push('/login') }
}

watch(() => route.params.serverId, (id) => {
  if (id) {
    const s = new Set(expandedServers.value)
    s.add(Number(id))
    expandedServers.value = s
  }
}, { immediate: true })

function onGlobalKey(e: KeyboardEvent) {
  const isMod = e.metaKey || e.ctrlKey
  if (isMod && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    cmdPaletteRef.value?.open()
  }
}

onMounted(() => {
  serverStore.ensure()
  appStore.ensure()
  window.addEventListener('keydown', onGlobalKey)
})
</script>

<style scoped>
/* ══════════ Shell ══════════ */
.shell {
  display: grid;
  grid-template-columns: var(--layout-sidebar-w) 1fr;
  height: 100vh;
  overflow: hidden;
  background: var(--ui-bg);
  transition: grid-template-columns var(--dur-base) var(--ease);
}
.shell--collapsed { grid-template-columns: var(--layout-sidebar-w-collapsed) 1fr; }

/* ══════════ Sidebar ══════════ */
.aside {
  background: var(--ui-bg-1);
  border-right: 1px solid var(--ui-border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}
.logo {
  height: var(--layout-header-h);
  display: flex; align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--ui-border);
  flex-shrink: 0;
}
.logo__mark {
  width: 28px; height: 28px;
  border-radius: var(--radius-sm);
  display: grid; place-items: center;
  font-size: 14px; font-weight: var(--fw-bold);
  color: var(--ui-fg-on-brand);
  background: var(--ui-brand);
  flex-shrink: 0;
}
.logo__text {
  font-size: var(--fs-md);
  font-weight: var(--fw-semibold);
  color: var(--ui-fg);
  letter-spacing: -0.01em;
}

.nav {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-2) var(--space-2) var(--space-4);
  display: flex; flex-direction: column;
}
.nav__group {
  font-size: 11px;
  color: var(--ui-fg-4);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  padding: var(--space-3) var(--space-3) var(--space-1);
  font-weight: var(--fw-semibold);
}
.nav__row {
  display: flex; align-items: center;
  gap: var(--space-2);
  height: 32px;
  padding: 0 var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--ui-fg-2);
  font-size: var(--fs-sm);
  cursor: pointer;
  text-decoration: none;
  transition: background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
  user-select: none;
  background: none;
  border: 0;
  width: 100%;
  text-align: left;
  font-family: inherit;
}
.nav__row:hover { background: var(--ui-bg-2); color: var(--ui-fg); }
.nav__row.is-active {
  background: var(--ui-bg-2);
  color: var(--ui-fg);
  font-weight: var(--fw-medium);
}
.nav__row--indent { padding-left: var(--space-5); }
.nav__row--sub { padding-left: var(--space-10); height: 28px; font-size: var(--fs-xs); color: var(--ui-fg-3); }
.nav__row--sub.is-active { color: var(--ui-brand-fg); }
.nav__row--muted { color: var(--ui-fg-3); opacity: 0.8; }
.nav__row--muted:hover { opacity: 1; }
.nav__row--expandable { padding-left: var(--space-5); }

.nav__icon {
  display: inline-flex; align-items: center; justify-content: center;
  width: 16px; height: 16px; flex-shrink: 0;
  color: var(--ui-fg-3);
}
.nav__row.is-active .nav__icon { color: var(--ui-brand); }
.nav__label {
  flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.nav__chev {
  opacity: 0.6;
  transition: transform var(--dur-base) var(--ease);
  flex-shrink: 0;
}
.nav__chev.is-open { transform: rotate(90deg); opacity: 1; }
.nav__badge {
  background: var(--ui-danger);
  color: #fff;
  font-size: 10px;
  min-width: 18px; height: 18px; padding: 0 5px;
  border-radius: var(--radius-pill);
  display: inline-flex; align-items: center; justify-content: center;
  font-weight: var(--fw-semibold);
  line-height: 1;
}
.nav__sub { display: flex; flex-direction: column; overflow: hidden; }

.shell--collapsed .nav__row { justify-content: center; padding: 0; width: 40px; margin: 1px auto; }
.shell--collapsed .logo { justify-content: center; padding: 0; }

.aside__foot {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-2); padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--ui-border);
  flex-shrink: 0;
}
.shell--collapsed .aside__foot { flex-direction: column; }
.collapse-btn {
  width: 28px; height: 28px;
  display: grid; place-items: center;
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-1);
  color: var(--ui-fg-3);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--dur-fast) var(--ease);
}
.collapse-btn:hover {
  color: var(--ui-brand);
  border-color: var(--ui-brand);
  background: var(--ui-brand-soft);
}

/* ══════════ Main ══════════ */
.main {
  min-width: 0;
  display: flex; flex-direction: column;
  overflow: hidden;
}
.header {
  height: var(--layout-header-h);
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 var(--space-5);
  background: var(--ui-bg-1);
  border-bottom: 1px solid var(--ui-border);
  flex-shrink: 0;
  z-index: var(--z-sticky);
}
.header__left { display: flex; align-items: center; min-width: 0; }
.header__right { display: flex; align-items: center; gap: var(--space-2); flex-shrink: 0; }
.header__title { font-size: var(--fs-md); font-weight: var(--fw-semibold); color: var(--ui-fg); }

.bc { display: flex; align-items: center; gap: var(--space-2); font-size: var(--fs-sm); }
.bc__item {
  color: var(--ui-fg-3);
  text-decoration: none;
  transition: color var(--dur-fast) var(--ease);
}
.bc__item:hover { color: var(--ui-fg); }
.bc__item--current { color: var(--ui-fg); font-weight: var(--fw-medium); }
.bc__sep { color: var(--ui-fg-4); }

.cmd {
  display: inline-flex; align-items: center; gap: var(--space-2);
  height: var(--control-h-sm); padding: 0 var(--space-2) 0 var(--space-3);
  border: 1px solid var(--ui-border);
  border-radius: var(--radius-sm);
  background: var(--ui-bg-2);
  color: var(--ui-fg-3);
  cursor: pointer;
  transition: all var(--dur-fast) var(--ease);
  min-width: 220px;
  font-family: inherit;
}
.cmd:hover {
  border-color: var(--ui-border-strong);
  color: var(--ui-fg);
  background: var(--ui-bg-1);
}
.cmd__icon { color: var(--ui-fg-4); flex-shrink: 0; }
.cmd__text { font-size: var(--fs-sm); flex: 1; text-align: left; }
@media (max-width: 900px) {
  .cmd { min-width: 0; }
  .cmd__text { display: none; }
}

.user {
  display: inline-flex; align-items: center; gap: var(--space-2);
  height: 32px; padding: 0 var(--space-2) 0 var(--space-1);
  border: none; background: transparent;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease);
  font-family: inherit;
}
.user:hover { background: var(--ui-bg-2); }
.user__avatar {
  width: 26px; height: 26px;
  border-radius: 50%;
  background: var(--ui-brand);
  color: var(--ui-fg-on-brand);
  font-size: 11px; font-weight: var(--fw-bold);
  display: grid; place-items: center;
  flex-shrink: 0;
}
.user__name {
  font-size: var(--fs-sm);
  color: var(--ui-fg-2);
  max-width: 100px;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.user__chev { color: var(--ui-fg-4); }

.content {
  background: var(--ui-bg);
  overflow-y: auto;
  flex: 1;
  position: relative;
}
.content--fullscreen {
  overflow: hidden;
  display: flex; flex-direction: column;
}

.route-fade-enter-active {
  transition: opacity var(--dur-base) var(--ease), transform var(--dur-base) var(--ease);
}
.route-fade-enter-from { opacity: 0; transform: translateY(4px); }
.route-fade-leave-active { transition: opacity var(--dur-fast) var(--ease); }
.route-fade-leave-to { opacity: 0; }

.fade-enter-active, .fade-leave-active { transition: opacity var(--dur-fast) var(--ease); }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.expand-enter-active, .expand-leave-active {
  transition: max-height var(--dur-base) var(--ease), opacity var(--dur-fast);
  overflow: hidden;
  max-height: 400px;
}
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; }
</style>
