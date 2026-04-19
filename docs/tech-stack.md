# 技术选型

## 一、后端：Go 1.22+

### 选型理由

| 维度 | 说明 |
|------|------|
| 并发模型 | goroutine 天然映射 SSH 连接池：每个 SSH 连接一个 goroutine，数千并发不需要线程池调度 |
| 单二进制部署 | `go embed` 将前端 dist/ 打包进二进制，发布只需一个文件，无运行时依赖 |
| 跨平台编译 | `GOOS=linux GOARCH=amd64 go build` 一行命令，在 macOS 上构建 Linux 二进制 |
| 内存占用 | 空载 < 30MB RSS，满载（5台服务器实时监控）< 80MB |
| 标准库完备 | `crypto/ssh`、`net/http`、`encoding/json` 开箱即用，无需重量级框架 |
| 生态成熟 | Docker SDK、SFTP、GORM、Gin 均有成熟 Go 实现 |

### 对比其他语言

| 语言 | 排除理由 |
|------|---------|
| Rust | 性能过剩，SSH/Docker/ORM 生态不如 Go 成熟，开发效率低 |
| Python | 无法单二进制分发，GIL 限制并发，内存占用高 |
| Node.js | 无法静态编译，npm 依赖地狱，不适合系统运维场景 |
| Java/Kotlin | JVM 启动慢，内存占用 300MB+，部署重 |

### 核心依赖库

```
框架/路由    github.com/gin-gonic/gin v1.9+
ORM         gorm.io/gorm + gorm.io/driver/sqlite
SSH         golang.org/x/crypto/ssh
SFTP        github.com/pkg/sftp
WebSocket   github.com/gorilla/websocket
Docker SDK  github.com/docker/docker/client
JWT         github.com/golang-jwt/jwt/v5
AES 加密    标准库 crypto/aes + crypto/cipher
任务调度    github.com/robfig/cron/v3
配置文件    gopkg.in/yaml.v3
前端嵌入    标准库 embed（Go 1.16+）
系统托盘    github.com/fyne-io/systray（跨平台：macOS/Windows/Linux）
原生通知    github.com/gen2brain/beeep（跨平台系统通知）
```

---

## 二、前端：Vue 3 + TypeScript（严格模式）

### 选型理由

| 维度 | 说明 |
|------|------|
| Composition API | 复杂逻辑（终端/实时图表/文件树）用 composable 封装，比 Options API 更清晰 |
| Element Plus | 中文生态最完善的企业级组件库，Table/Form/Tree/Dialog 覆盖所有运维场景 |
| TypeScript | 严格模式 (`strict: true`)，API 响应类型全部定义，减少运行时错误 |
| Vite 5 | HMR < 100ms，构建速度比 Webpack 快 10x |
| Pinia | Vue 官方推荐状态管理，比 Vuex 更轻量，TypeScript 支持更好 |

### 对比其他框架

| 框架 | 排除理由 |
|------|---------|
| React + Ant Design | 学习曲线更陡，Ant Design 中文文档质量不如 Element Plus |
| Svelte | 生态较小，Xterm.js/CodeMirror 集成需要额外适配 |
| Vue 3 + Vuetify | Material Design 风格不符合运维面板审美 |

### 核心依赖库

```
框架        vue@3.4+  typescript@5.4+
构建        vite@5.2+
UI 组件     element-plus@2.7+  @element-plus/icons-vue
状态管理    pinia@2.1+
路由        vue-router@4.3+
HTTP        axios@1.6+
图表        echarts@5.5+
终端        xterm@5.3+  @xterm/addon-fit  @xterm/addon-search
代码编辑    @codemirror/view@6+  @codemirror/lang-nginx  @codemirror/lang-yaml
工具        dayjs@1.11+  @vueuse/core@10+
```

---

## 三、数据库：SQLite（via GORM）

### 选型理由

- **零依赖**：单文件数据库，无需安装 MySQL/PostgreSQL
- **容量足够**：5 台服务器 × 5 分钟一条指标 × 24 小时 = 1440 条/天，SQLite 轻松支持
- **备份简单**：`cp serverhub.db serverhub.db.bak` 即完成备份
- **升级路径**：GORM 支持多数据库驱动，未来换 PostgreSQL 只需改 DSN 和驱动包

### 数据保留策略

| 数据类型 | 保留时长 |
|---------|---------|
| 实时指标（每 5 秒） | 内存缓存，不持久化 |
| 历史指标（每 5 分钟聚合） | SQLite，保留 24 小时 |
| 部署历史 | SQLite，永久保留（可手动清理） |
| 审计日志 | SQLite，永久保留（可手动清理） |
| 告警记录 | SQLite，永久保留 |

---

## 四、构建与分发

### 单二进制方案（go embed）

```go
//go:embed web/dist
var staticFiles embed.FS
```

构建流程：
```
npm run build  →  frontend/dist/
go build       →  serverhub（内嵌 dist/）
```

最终产物：一个 `serverhub` 二进制文件（约 20-40MB），包含：
- Go 运行时
- 后端逻辑
- 前端静态资源（HTML/JS/CSS）

### 构建目标

| 平台 | 模式 | 命令 |
|------|------|------|
| macOS（桌面 App） | 托盘 App | `GOOS=darwin go build -tags desktop` |
| Linux（桌面 App） | 托盘 App | `GOOS=linux GOARCH=amd64 go build -tags desktop` |
| Linux amd64（服务器） | 服务器模式 | `GOOS=linux GOARCH=amd64 go build -tags server` |
| Linux arm64（服务器） | 服务器模式 | `GOOS=linux GOARCH=arm64 go build -tags server` |
| Windows（桌面 App） | 托盘 App | `GOOS=windows go build -tags desktop` |

**Build Tags 区分两种模式：**

```go
// tray_desktop.go （+build desktop）
// 初始化 systray，监听告警事件，驱动原生通知

// tray_server.go  （+build server）
// 空实现，编译时零开销
```

macOS 应用打包：二进制 + `Info.plist` + 图标，打包为 `ServerHub.app`，放入 `/Applications` 即完成安装。

---

## 五、基础设施

| 组件 | 适用模式 | 职责 |
|------|---------|------|
| OpenResty / Nginx | 服务器模式 | 反向代理，处理 TLS，转发 `/panel/` 到后端 |
| WireGuard（可选） | 两种模式 | 内网服务器组网，使内网机器通过 VPN 可达 |
| certbot | 两种模式 | Let's Encrypt 证书申请（在目标服务器上执行） |
| systemd | 服务器模式 | 进程守护，开机自启 |
| launchd（macOS） | 桌面模式 | 开机自启（可选），或直接开机启动 App |
