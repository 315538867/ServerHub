// router/legacy-redirects.ts
//
// 集中放所有"老 URL → 新 URL"的永久 redirect。每条都是某次架构演进留下的
// 兼容残留:旧版前端发出去的链接、用户书签、邮件通知里的深链都还会命中,这
// 个文件保证它们落到当前正确的页面。
//
// 维护规则:
//   - 新写代码不许往这里加 redirect;新功能直接定义 canonical 路由就行。
//   - 任何条目都必须带 `EXPIRES:` 注释,日期一到该条删除,不再为已经离开的
//     旧前端兜底。
//   - 删除时不需要改任何 view —— 此文件就是唯一的引用点。
//
// 当前所有条目的下线日期统一定在 2026-10-01 (P3-F 之后 6 个月)。届时如果
// 监控显示访问量已经归零,直接清空 arrays,这个模块就只剩骨架。

import type { RouteRecordRaw } from 'vue-router'

// EXPIRES: 2026-10-01
// 应用详情下的旧标签页 URL,M2/M3 重构后被新的 5-Tab 结构替换。
export const appLegacyRedirects: RouteRecordRaw[] = [
  { path: 'nginx', redirect: (to) => `/apps/${to.params.appId}/network/routes` },
  { path: 'domain', redirect: (to) => `/apps/${to.params.appId}/network/domain` },
  { path: 'service', redirect: (to) => `/apps/${to.params.appId}/ops/service` },
  { path: 'logs', redirect: (to) => `/apps/${to.params.appId}/ops/logs` },
  { path: 'terminal', redirect: (to) => `/apps/${to.params.appId}/ops/terminal` },
  { path: 'database', redirect: (to) => `/apps/${to.params.appId}/data` },
  // M3: /apps/:id/deploy & /env 退役,统一去 Releases Tab。
  { path: 'deploy', redirect: (to) => `/apps/${to.params.appId}/releases` },
  { path: 'env', redirect: (to) => `/apps/${to.params.appId}/releases` },
]

// EXPIRES: 2026-10-01
// 应用 network 子树下的 legacy 子页:Domain.vue 已删,统一跳 ingresses。
export const appNetworkLegacyRedirects: RouteRecordRaw[] = [
  { path: 'domain', redirect: (to) => `/apps/${to.params.appId}/network/ingresses` },
]

// EXPIRES: 2026-10-01
// 服务器详情:Nginx 子页(Nginx.vue)已删,Ingress 模型接管。
export const serverLegacyRedirects: RouteRecordRaw[] = [
  { path: 'nginx', redirect: (to) => `/servers/${to.params.serverId}/ingresses` },
]
