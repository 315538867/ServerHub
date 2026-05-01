import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getSetupStatus } from '@/api/setup'
import {
  appLegacyRedirects,
  appNetworkLegacyRedirects,
  serverLegacyRedirects,
} from './legacy-redirects'

const router = createRouter({
  history: createWebHistory('/panel/'),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login/index.vue'),
      meta: { public: true },
    },
    {
      path: '/setup',
      name: 'Setup',
      component: () => import('@/views/Setup/Wizard.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard/index.vue') },

        // ── 项目（主路由 /apps，/projects 作为别名） ──
        { path: 'apps', name: 'AppList', component: () => import('@/views/Apps/List.vue') },
        { path: 'projects', redirect: '/apps' },
        { path: 'apps/create', name: 'AppCreate', component: () => import('@/views/Apps/Create.vue') },
        { path: 'projects/create', redirect: '/apps/create' },
        {
          path: 'apps/:appId',
          component: () => import('@/layouts/AppLayout.vue'),
          redirect: (to) => `${to.path}/overview`,
          children: [
            // ── 5 Tab 新结构 ──
            { path: 'overview', name: 'AppOverview', component: () => import('@/views/Apps/Overview.vue') },
            { path: 'services', name: 'AppServices', component: () => import('@/views/Apps/Services.vue') },
            { path: 'releases', redirect: (to) => `/apps/${to.params.appId}/services` },
            {
              path: 'network',
              component: () => import('@/views/Apps/Network.vue'),
              // ingresses 反向视图对所有 app 都有意义(无 site_name 也能用),
              // 而 domain 子页仅在 app.site_name 存在时才有内容,因此默认跳 ingresses。
              redirect: (to) => `${to.path}/ingresses`,
              children: [
                { path: 'ingresses', name: 'AppNetworkIngresses', component: () => import('@/views/Apps/Ingresses.vue') },
                ...appNetworkLegacyRedirects,
              ],
            },
            {
              path: 'ops',
              component: () => import('@/views/Apps/Ops.vue'),
              redirect: (to) => `${to.path}/logs`,
              children: [
                { path: 'service', name: 'AppOpsService', component: () => import('@/views/Apps/Service.vue') },
                { path: 'logs', name: 'AppOpsLogs', component: () => import('@/views/Apps/Logs.vue') },
                { path: 'terminal', name: 'AppOpsTerminal', component: () => import('@/views/Apps/Terminal.vue') },
              ],
            },
            { path: 'data', name: 'AppData', component: () => import('@/views/Apps/Database.vue') },

            ...appLegacyRedirects,
          ],
        },

        // ── 项目别名 (/projects/* → /apps/*) ──
        { path: 'projects/:id', redirect: (to) => `/apps/${to.params.id}` },
        { path: 'projects/:id/overview', redirect: (to) => `/apps/${to.params.id}/overview` },
        { path: 'projects/:id/deploy', redirect: (to) => `/apps/${to.params.id}/services` },
        { path: 'projects/:id/traffic', redirect: (to) => `/apps/${to.params.id}/network/ingresses` },
        { path: 'projects/:id/ops', redirect: (to) => `/apps/${to.params.id}/ops/logs` },
        { path: 'projects/:id/data', redirect: (to) => `/apps/${to.params.id}/data` },

        // ── 服务器 ──
        { path: 'servers', name: 'Servers', component: () => import('@/views/Servers/index.vue') },
        {
          path: 'servers/:serverId',
          component: () => import('@/layouts/ServerLayout.vue'),
          redirect: (to) => `${to.path}/overview`,
          children: [
            { path: 'overview', name: 'ServerOverview', component: () => import('@/views/ServerDetail/Overview.vue') },
            { path: 'services', name: 'ServerServices', component: () => import('@/views/ServerDetail/Services.vue') },
            { path: 'ingresses', name: 'ServerIngresses', component: () => import('@/views/ServerDetail/Ingresses.vue') },
            { path: 'networks', name: 'ServerNetworks', component: () => import('@/views/ServerDetail/Networks.vue') },
            { path: 'docker', name: 'ServerDocker', component: () => import('@/views/ServerDetail/Docker.vue') },
            { path: 'system', name: 'ServerSystem', component: () => import('@/views/ServerDetail/System.vue') },
            { path: 'logs-search', name: 'ServerLogSearch', component: () => import('@/views/ServerDetail/LogSearch.vue') },
            { path: 'files', name: 'ServerFiles', component: () => import('@/views/ServerDetail/Files.vue') },
            { path: 'terminal', name: 'ServerTerminal', component: () => import('@/views/ServerDetail/Terminal.vue') },
            { path: 'discover', name: 'ServerDiscover', component: () => import('@/views/ServerDetail/Discover.vue') },

            ...serverLegacyRedirects,
          ],
        },

        // ── Service 详情（Phase M1 新链路） ──
        { path: 'services/:id', name: 'ServiceDetail', component: () => import('@/views/service/ServiceDetail.vue') },

        // ── 全局管理（保留：功能未被应用视角完全替代） ──
        { path: 'database', name: 'Database', component: () => import('@/views/Database/index.vue') },

        // ── 全局 ──
        // /ingresses 是跨 Edge 总览(只读),编辑入口仍在 /servers/:id/ingresses,
        // 因为 apply 必须按 Edge 走(每台 nginx 独立 reload),没法在总览里安全批量。
        { path: 'ingresses', name: 'Ingresses', component: () => import('@/views/Ingresses/index.vue') },
        { path: 'notifications', name: 'Notifications', component: () => import('@/views/Notifications/index.vue') },
        { path: 'settings', name: 'Settings', component: () => import('@/views/Settings/index.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

// Cache the setup-status probe for the session so the guard doesn't hammer
// the endpoint on every navigation. Cleared on logout via the auth store.
let setupChecked = false
let setupPending = false
let setupNeeded = false

async function checkSetup(): Promise<boolean> {
  if (setupChecked) return setupNeeded
  if (setupPending) return false
  setupPending = true
  try {
    const st = await getSetupStatus()
    setupNeeded = st.needs_admin
    setupChecked = true
    return setupNeeded
  } catch {
    return false
  } finally {
    setupPending = false
  }
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Let the wizard page load freely; it fetches status on its own.
  if (to.name === 'Setup') return

  // Before login, a fresh install should route visitors to the wizard
  // instead of the login page.
  if (!auth.token) {
    if (await checkSetup()) {
      return { name: 'Setup' }
    }
    if (!to.meta.public) {
      return { name: 'Login', query: { redirect: to.fullPath } }
    }
    return
  }

  // Logged in: if backend still reports wizard steps pending (e.g. admin
  // created but local-server step abandoned), push user back to the wizard.
  if (await checkSetup()) {
    return { name: 'Setup' }
  }

  if (to.name === 'Login' && auth.token) {
    return { name: 'Dashboard' }
  }
})

// Exposed so the wizard can invalidate the cache once it finishes,
// forcing the next navigation to re-check the backend.
export function invalidateSetupCache() {
  setupChecked = false
  setupNeeded = false
}

export default router
