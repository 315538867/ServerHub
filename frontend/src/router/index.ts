import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

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
            { path: 'overview', name: 'AppOverview', component: () => import('@/views/Apps/Overview.vue') },
            { path: 'domain', name: 'AppDomain', component: () => import('@/views/Apps/Domain.vue') },
            { path: 'service', name: 'AppService', component: () => import('@/views/Apps/Service.vue') },
            { path: 'deploy', name: 'AppDeploy', component: () => import('@/views/Apps/Deploy.vue') },
            { path: 'logs', name: 'AppLogs', component: () => import('@/views/Apps/Logs.vue') },
            { path: 'database', name: 'AppDatabase', component: () => import('@/views/Apps/Database.vue') },
            { path: 'env', name: 'AppEnv', component: () => import('@/views/Apps/Env.vue') },
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
            { path: 'nginx', name: 'ServerNginx', component: () => import('@/views/ServerDetail/Nginx.vue') },
            { path: 'docker', name: 'ServerDocker', component: () => import('@/views/ServerDetail/Docker.vue') },
            { path: 'system', name: 'ServerSystem', component: () => import('@/views/ServerDetail/System.vue') },
            { path: 'files', name: 'ServerFiles', component: () => import('@/views/ServerDetail/Files.vue') },
            { path: 'terminal', name: 'ServerTerminal', component: () => import('@/views/ServerDetail/Terminal.vue') },
          ],
        },

        // ── 全局管理（保留：功能未被应用视角完全替代） ──
        { path: 'deploy', name: 'Deploy', component: () => import('@/views/Deploy/index.vue') },
        { path: 'database', name: 'Database', component: () => import('@/views/Database/index.vue') },

        // ── 全局 ──
        { path: 'notifications', name: 'Notifications', component: () => import('@/views/Notifications/index.vue') },
        { path: 'settings', name: 'Settings', component: () => import('@/views/Settings/index.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.token) {
    return { name: 'Login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'Login' && auth.token) {
    return { name: 'Dashboard' }
  }
})

export default router
