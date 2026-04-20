# ServerHub

<p align="center">
  <strong>自研多服务器管理面板</strong><br>
  通过 SSH 统一管理所有远程服务器，融合现代 UI 与运维最佳实践
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://github.com/315538867/ServerHub/actions/workflows/release.yml"><img src="https://github.com/315538867/ServerHub/actions/workflows/release.yml/badge.svg" alt="Build"></a>
</p>

---

## 简介

ServerHub 是一个运行在**主控服务器**上的轻量多服务器管理面板，通过 SSH 连接并统一管理所有远程服务器。

- **单二进制**：前端静态文件嵌入 Go 二进制，无需额外部署 Web 服务器
- **多服务器**：在一个面板中管理任意数量的远程服务器
- **应用化管理**：以「应用」为单位组织资源，关联服务器、部署、域名、数据库

## 功能特性

| 模块 | 功能 |
|------|------|
| **服务器监控** | 实时 CPU / 内存 / 磁盘指标，历史图表 |
| **Web 终端** | 浏览器内 SSH 终端，支持多 Tab 多服务器 |
| **文件管理** | 远程文件浏览、上传、下载、在线编辑 |
| **Docker 管理** | 容器列表、启停、日志、环境变量查看 |
| **Nginx 管理** | 站点配置查看/编辑，SSL 证书申请与自动续期 |
| **应用管理** | 应用全生命周期管理：路由/部署/域名/数据库/环境变量 |
| **Nginx 路由** | 每个应用独立配置 path 转发或独立站点 |
| **部署管理** | Docker Compose / Docker / 原生命令，支持版本管理和 Webhook 触发 |
| **数据库** | 数据库连接管理（MySQL / PostgreSQL / Redis） |
| **系统信息** | 内核、网络接口、磁盘分区、进程概览 |
| **用户安全** | JWT 认证，MFA（TOTP），登录审计，操作日志 |

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.23 · Gin · GORM · gorilla/websocket |
| 前端 | Vue 3 · TypeScript · TDesign Vue Next · Xterm.js · ECharts |
| 存储 | SQLite（面板自身配置和数据） |
| SSH | `golang.org/x/crypto/ssh` + 连接池 |
| 嵌入 | `embed.FS`（前端静态文件打入二进制） |

## 快速开始

### 下载预编译二进制

从 [Releases](https://github.com/315538867/ServerHub/releases) 下载对应平台的压缩包：

```bash
# Linux amd64
tar -xzf serverhub_linux_amd64.tar.gz
chmod +x serverhub

# 首次运行（自动生成配置文件）
./serverhub --config /opt/serverhub/config.yaml
```

浏览器访问 `http://<your-ip>:9999/panel/`

### 从源码构建

**前提：** Go 1.23+、Node.js 20+、pnpm

```bash
git clone https://github.com/315538867/ServerHub.git
cd ServerHub

# 构建前端并打入后端
make build

# 运行
./serverhub
```

### 开发环境

```bash
# 启动后端（热重载）
make dev-backend

# 启动前端（Vite 开发服务器）
make dev-frontend
```

## 配置

首次运行会在 `--config` 指定路径生成默认配置文件，参考 [`backend/config.example.yaml`](backend/config.example.yaml)：

```yaml
server:
  port: 9999
  data_dir: /opt/serverhub

security:
  jwt_secret: "your-64-char-random-secret"
  aes_key: "your-32-byte-hex-key"
  allow_register: false

certbot:
  email: "admin@yourdomain.com"
```

## 目录结构

```
ServerHub/
├── backend/              # Go 后端
│   ├── api/              # HTTP 路由处理器
│   ├── model/            # 数据模型（GORM）
│   ├── pkg/              # 内部工具包（sshpool, crypto, deployer…）
│   ├── config/           # 配置加载
│   └── web/dist/         # 编译后的前端（由 Makefile 自动填充）
├── frontend/             # Vue 3 前端
│   └── src/
│       ├── views/        # 页面组件
│       ├── api/          # HTTP 客户端
│       ├── stores/       # Pinia 状态
│       └── layouts/      # 布局组件
└── docs/                 # 设计文档
```

## Contributing

欢迎 PR 和 Issue！请遵循以下规范：

- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)
- 前端使用 TypeScript，后端保持 Go 惯用风格
- 新功能请先开 Issue 讨论

## License

[MIT](LICENSE) © 2026 ServerHub Contributors
