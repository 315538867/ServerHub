# 前端设计

## 工程结构

```
frontend/src/
├── api/             # axios 封装 + 各模块接口（servers / docker / system / logsearch ...）
├── components/
│   └── ui/          # 基础 UI 组件库（UiCard/UiSection/UiButton/UiBadge/UiTabs ...）
├── composables/     # 复用 hook（如 useTheme, useResize）
├── layouts/         # MainLayout / AppLayout / ServerLayout
├── router/          # index.ts（路由表）+ app.ts（应用 detail 嵌套）
├── stores/          # Pinia: auth / server / theme
├── styles/          # 全局 CSS 变量与重置（深色模式由 CSS 变量切换）
├── types/           # 共享 TS 类型
├── utils/           # 工具函数
└── views/           # 页面（按业务模块分目录）
```

## 路由组织

`/panel/` 为基路径（vue-router 的 `createWebHistory('/panel/')`）。

顶层布局 `MainLayout`（侧栏 + 主区）。两条业务嵌套布局：

- `AppLayout`（`/apps/:appId`）：5 Tab — 概览 / 部署 / 网络（路由 + 域名）/ 运维（服务 + 日志 + 终端）/ 数据
- `ServerLayout`（`/servers/:serverId`）：6 Tab — 概览 / Nginx / Docker / 系统 / 日志搜索 / 文件 / 终端

旧 URL（如 `/apps/:id/nginx`、`/apps/:id/database`）配置 301 重定向，保护书签。

路由守卫：未登录跳 `/login`，已登录访问 `/login` 跳 `/dashboard`。

## 状态管理

| Store | 状态 | 触发 |
|---|---|---|
| `auth` | `token`, `user` | 登录后写 localStorage；axios 拦截 401 → 清空 |
| `server` | `servers[]` 缓存 | `fetch()` 拉列表；详情页通过 `getById` |
| `theme` | `mode (light|dark|auto)` | 持久化 localStorage；切换写 `<html data-theme>` |

## API 客户端

`api/request.ts` 统一封装：

- `baseURL: '/panel/api/v1'`
- 请求拦截器：注入 `Authorization`
- 响应拦截器：拆封 `{code,msg,data}` 直接返回 `data`；非 0 抛错 + `naive-ui message.error`
- 401 → 跳登录

## UI 组件约定

`components/ui/` 提供基础集合，所有视图优先复用：

`UiCard / UiSection / UiTableCard / UiPageHeader / UiToolbar / UiTabs / UiButton / UiIconButton / UiBadge / UiKbd / UiStatCard / UiSparkline / UiSkeleton / UiStateBanner / StatusDot / StatusTag / EmptyBlock / LogOutput / UiThemeToggle`

设计约束：

- 颜色/间距/字号通过 CSS 变量（`--ui-bg`、`--space-*`、`--fs-*`、`--ui-brand` 等），便于深浅色一键切换
- 文案中文为主；表单 hint 用 `--fs-xs / --ui-fg-3`
- 表格：`naive-ui` 的 `NDataTable`，包一层 `UiTableCard` 统一 padding
- 实时日志展示：`<pre>` + `--font-mono`，长内容 max-height + overflow-auto

## WebSocket 客户端

WS URL 由 `api/*.ts` 内 `xxxWsUrl(sid, token)` 拼装，例：

```ts
ws://<host>/panel/api/v1/servers/:id/system/services/:name/logs?token=<jwt>
```

业务消息体由 `pkg/wsstream` 定义：

```json
{ "type": "output", "data": "line content" }
{ "type": "error",  "data": "..." }
{ "type": "dropped","count": 5 }
{ "type": "done" }
```

终端走 xterm.js + 自定义协议（输入/resize/数据帧）。

## 国际化与主题

当前仅中文。深色模式通过 `--ui-*` 变量切换，组件无需改写。
