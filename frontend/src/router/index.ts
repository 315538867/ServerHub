import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getSetupStatus } from '@/api/setup'

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

        // ── 应用 ──
        { path: 'apps', name: 'AppList', component: () => import('@/views/Apps/List.vue') },
        { path: 'apps/create', name: 'AppCreate', component: () => import('@/views/Apps/Create.vue') },
        {
          path: 'apps/:appId',
          component: () => import('@/layouts/AppLayout.vue'),
          redirect: (to) => `${to.path}/overview`,
          children: [
            // ── 5 Tab 新结构 ──
            { path: 'overview', name: 'AppOverview', component: () => import('@/views/Apps/Overview.vue') },
            { path: 'releases', name: 'AppReleases', component: () => import('@/views/Apps/Releases.vue') },
            {
              path: 'network',
              component: () => import('@/views/Apps/Network.vue'),
              redirect: (to) => `${to.path}/routes`,
              children: [
                { path: 'routes', name: 'AppNetworkRoutes', component: () => import('@/views/Apps/NginxRoutes.vue') },
                { path: 'domain', name: 'AppNetworkDomain', component: () => import('@/views/Apps/Domain.vue') },
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

            // ── 旧 URL 向后兼容（永久重定向，保护书签/历史链接） ──
            { path: 'nginx', redirect: (to) => `/apps/${to.params.appId}/network/routes` },
            { path: 'domain', redirect: (to) => `/apps/${to.params.appId}/network/domain` },
            { path: 'service', redirect: (to) => `/apps/${to.params.appId}/ops/service` },
            { path: 'logs', redirect: (to) => `/apps/${to.params.appId}/ops/logs` },
            { path: 'terminal', redirect: (to) => `/apps/${to.params.appId}/ops/terminal` },
            { path: 'database', redirect: (to) => `/apps/${to.params.appId}/data` },
            // M3: /apps/:id/deploy 已退役，统一跳到 Releases Tab
            { path: 'deploy', redirect: (to) => `/apps/${to.params.appId}/releases` },
            { path: 'env', redirect: (to) => `/apps/${to.params.appId}/releases` },
          ],
        },

        // ── 服务器 ──
        { path: 'servers', name: 'Servers', component: () => import('@/views/Servers/index.vue') },
        {
          path: 'servers/:serverId',
          component: () => import('@/layouts/ServerLayout.vue'),
          redirect: (to) => `${to.path}/overview`,
          children: [
            { path: 'overview', name: 'ServerOverview', component: () => import('@/views/ServerDetail/Overview.vue') },
            { path: 'services', name: 'ServerServices', component: () => import('@/views/ServerDetail/Services.vue') },
            { path: 'nginx', name: 'ServerNginx', component: () => import('@/views/ServerDetail/Nginx.vue') },
            { path: 'ingresses', name: 'ServerIngresses', component: () => import('@/views/ServerDetail/Ingresses.vue') },
            { path: 'networks', name: 'ServerNetworks', component: () => import('@/views/ServerDetail/Networks.vue') },
            { path: 'docker', name: 'ServerDocker', component: () => import('@/views/ServerDetail/Docker.vue') },
            { path: 'system', name: 'ServerSystem', component: () => import('@/views/ServerDetail/System.vue') },
            { path: 'logs-search', name: 'ServerLogSearch', component: () => import('@/views/ServerDetail/LogSearch.vue') },
            { path: 'files', name: 'ServerFiles', component: () => import('@/views/ServerDetail/Files.vue') },
            { path: 'terminal', name: 'ServerTerminal', component: () => import('@/views/ServerDetail/Terminal.vue') },
            { path: 'discover', name: 'ServerDiscover', component: () => import('@/views/ServerDetail/Discover.vue') },
          ],
        },

        // ── Service 详情（Phase M1 新链路） ──
        { path: 'services/:id', name: 'ServiceDetail', component: () => import('@/views/service/ServiceDetail.vue') },

        // ── 全局管理（保留：功能未被应用视角完全替代） ──
        { path: 'database', name: 'Database', component: () => import('@/views/Database/index.vue') },

        // ── 全局 ──
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
