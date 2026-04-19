# 前端设计

## 一、技术选型

```
框架          Vue 3 (Composition API) + TypeScript 5（strict: true）
UI 组件       Element Plus 2.7+
状态管理      Pinia 2.1+
路由          Vue Router 4.3+
HTTP          Axios 1.6+（统一拦截器）
图表          ECharts 5.5+
终端          Xterm.js 5.3+ + FitAddon + SearchAddon
代码编辑      CodeMirror 6（nginx / yaml / json / shell / toml）
工具库        dayjs 1.11+ / @vueuse/core 10+
构建          Vite 5.2+
TypeScript    strict: true，禁止隐式 any
```

---

## 二、TypeScript 规范

```json
// tsconfig.json 关键配置
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "noUncheckedIndexedAccess": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  }
}
```

**API 响应类型约定：**
```typescript
// src/types/api.ts
export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
}
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  size: number
}
```

---

## 三、路由设计与导航守卫

```typescript
// src/router/index.ts
const routes = [
  { path: '/login', component: () => import('@/views/Login/index.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard',    component: () => import('@/views/Dashboard/index.vue') },
      { path: 'servers',      component: () => import('@/views/Servers/index.vue') },
      { path: 'terminal',     component: () => import('@/views/Terminal/index.vue') },
      { path: 'websites',     component: () => import('@/views/Websites/index.vue') },
      { path: 'ssl',          component: () => import('@/views/SSL/index.vue') },
      { path: 'docker',       component: () => import('@/views/Docker/index.vue') },
      { path: 'deploy',       component: () => import('@/views/Deploy/index.vue') },
      { path: 'database',     component: () => import('@/views/Database/index.vue') },
      { path: 'files',        component: () => import('@/views/Files/index.vue') },
      { path: 'system',       component: () => import('@/views/System/index.vue') },
      { path: 'notifications',component: () => import('@/views/Notifications/index.vue') },
      { path: 'settings',     component: () => import('@/views/Settings/index.vue') },
    ],
  },
]

// 全局导航守卫
router.beforeEach((to) => {
  const authStore = useAuthStore()
  if (!to.meta.public && !authStore.token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.path === '/login' && authStore.token) {
    return { path: '/dashboard' }
  }
})
```

---

## 四、全局 Axios 拦截器

```typescript
// src/api/request.ts
const instance = axios.create({ baseURL: '/panel/api/v1', timeout: 30000 })

// 请求拦截：注入 JWT
instance.interceptors.request.use((config) => {
  const token = useAuthStore().token
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应拦截：统一错误处理
instance.interceptors.response.use(
  (res) => {
    if (res.data.code !== 0) {
      ElMessage.error(res.data.msg)
      return Promise.reject(res.data)
    }
    return res.data.data
  },
  (err) => {
    if (err.response?.status === 401) {
      useAuthStore().logout()
      router.push('/login')
    } else {
      ElMessage.error(err.response?.data?.msg || '网络错误')
    }
    return Promise.reject(err)
  }
)
```

---

## 五、页面规划

### 登录页 `/login`

```
┌────────────────────────────────────────────────┐
│                                                │
│         ServerHub                              │
│         SSH-native Server Console             │
│                                                │
│   ┌────────────────────────────────────────┐   │
│   │  用户名  [________________]            │   │
│   │  密码    [________________]            │   │
│   │                                        │   │
│   │  [ 记住登录 ]  [ 登 录 ]               │   │
│   └────────────────────────────────────────┘   │
│                                                │
│   v1.0.0  ·  github.com/xxx/serverhub         │
└────────────────────────────────────────────────┘
```

MFA 第二步：弹出 Dialog 输入 6 位 TOTP 验证码

---

### 主布局（登录后）

```
┌──────────┬───────────────────────────────────────────┐
│          │  顶部栏：ServerHub  服务器选择器  告警铃铛  │
│  侧边栏  │──────────────────────────────────────────  │
│          │                                           │
│ Dashboard│              主内容区域                    │
│ 服务器   │                                           │
│ 终端     │                                           │
│ 网站     │                                           │
│ SSL      │                                           │
│ Docker   │                                           │
│ 部署     │                                           │
│ 数据库   │                                           │
│ 文件     │                                           │
│ 系统工具 │                                           │
│ 通知设置 │                                           │
│ 面板设置 │                                           │
└──────────┴───────────────────────────────────────────┘
```

---

### Dashboard `/dashboard`

```
┌─────────────────────────────────────────────────────────┐
│  服务器概览                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Server 1 │  │ Server 2 │  │ Server 3 │              │
│  │ 在线 🟢  │  │ 在线 🟢  │  │ 离线 🔴  │              │
│  │ CPU 23%  │  │ CPU 45%  │  │ ---      │              │
│  │ MEM 65%  │  │ MEM 80%  │  │ ---      │              │
│  │ DISK 40% │  │ DISK 60% │  │ ---      │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│                                                         │
│  告警信息                        SSL 证书到期           │
│  ┌──────────────────────────┐  ┌──────────────────────┐ │
│  │ 🔴 Server 2 内存 > 85%   │  │ example.com  剩 28天  │ │
│  │ 🟠 磁盘 /dev/sda > 80%   │  │ api.xxx.com  剩 5天   │ │
│  └──────────────────────────┘  └──────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

### 服务器详情（实时监控）

```
┌─────────────────────────────────────────────────────────┐
│  [Server 1]  82.156.201.5  Ubuntu 22.04  在线 ↑ 3d 5h  │
│                                                         │
│  CPU  23%  [========>           ]                       │
│  内存 65%  [===============>    ]  4.2GB / 8GB          │
│  磁盘 40%  [=========>          ]  40GB / 100GB         │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │  CPU 历史（24h）                                  │   │
│  │  [ECharts 折线图，横轴时间，纵轴百分比]            │   │
│  └──────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────┐   │
│  │  内存 / 网络 历史图（同上）                        │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

### Web 终端 `/terminal`

```
┌─────────────────────────────────────────────────────────┐
│  [ Server 1 × ] [ + 新标签页 ]                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │                                                    │  │
│  │  root@server1:~# █                                 │  │
│  │                                                    │  │
│  │  （Xterm.js 黑色背景终端）                         │  │
│  │                                                    │  │
│  └────────────────────────────────────────────────────┘  │
│  字体大小: [14] ▲▼   [ 复制 ]  [ 清屏 ]                  │
└─────────────────────────────────────────────────────────┘
```

多 Tab：每个 Tab 对应一台服务器，Tab 关闭时断开对应 WebSocket

---

### 部署页面 `/deploy`

```
┌─────────────────────────────────────────────────────────┐
│  应用部署  [ + 创建部署 ]  [ 服务模板 ]                  │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │ myapp  •  Server 1  •  git@github.com/xxx/app    │   │
│  │ 分支: main  最后部署: 2 小时前  状态: ✅ 成功      │   │
│  │ [ 部署 ]  [ 回滚 ]  [ 历史 ]  [ 设置 ]            │   │
│  └──────────────────────────────────────────────────┘   │
│                                                         │
│  部署流水线（进行中）：                                  │
│  ✅ 拉取代码  →  ⟳ 构建镜像  →  ○ 启动服务             │
│  ┌──────────────────────────────────────────────────┐   │
│  │ [2026-04-17 14:30:01] git fetch origin           │   │
│  │ [2026-04-17 14:30:02] HEAD is now at a1b2c3d    │   │
│  │ [2026-04-17 14:30:05] Step 1/8 : FROM node:20   │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

## 六、关键组件设计

### DeployPipeline.vue（部署流水线）

```typescript
interface Stage {
  id: string
  label: string
  status: 'pending' | 'running' | 'success' | 'failed'
}
const stages = ref<Stage[]>([
  { id: 'pull',  label: '拉取代码', status: 'pending' },
  { id: 'build', label: '构建镜像', status: 'pending' },
  { id: 'start', label: '启动服务', status: 'pending' },
])
// WebSocket 消息 type=stage 时更新对应 stage 状态
```

### EnvEditor.vue（环境变量编辑器）

```typescript
interface EnvVar {
  key: string
  value: string
  sensitive: boolean  // 是否隐藏值
  _visible: boolean   // 是否显示明文（前端状态）
}
// 使用 Element Plus ElTable 渲染
// sensitive=true 时 value 列显示 "***"，点击「显示」切换 _visible
```

### useWebSocket.ts（带重连的 WS 封装）

```typescript
export function useWebSocket(url: string) {
  const ws = ref<WebSocket | null>(null)
  let reconnectTimer: ReturnType<typeof setTimeout>

  const connect = () => {
    ws.value = new WebSocket(url)
    ws.value.onclose = () => {
      reconnectTimer = setTimeout(connect, 3000) // 3s 后重连
    }
  }
  connect()
  onUnmounted(() => {
    clearTimeout(reconnectTimer)
    ws.value?.close()
  })
  return ws
}
```

---

## 七、主题配色

```css
/* Element Plus CSS 变量覆盖 */
:root {
  --el-color-primary: #0066FF;    /* 主色：蓝 */
  --el-bg-color: #FFFFFF;
  --el-bg-color-page: #F5F7FA;
  --el-border-color: #E4E7ED;
  --sidebar-bg: #1A1A2E;          /* 侧边栏深色 */
  --sidebar-text: #B0B8C1;
  --sidebar-active: #0066FF;
}
/* 暗色模式（跟随系统）：使用 Element Plus dark class */
```

---

## 八、构建配置

```typescript
// vite.config.ts 关键配置
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': '/src' } },
  server: {
    proxy: {
      '/panel/api': { target: 'http://localhost:9999', ws: true },
      '/panel/webhooks': { target: 'http://localhost:9999' },
    }
  },
  build: {
    outDir: '../backend/web/dist',  // 直接输出到 go embed 目录
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-charts': ['echarts'],
          'vendor-terminal': ['xterm'],
          'vendor-editor': ['@codemirror/view'],
        }
      }
    }
  }
})
```
