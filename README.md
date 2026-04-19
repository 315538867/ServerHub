# ServerHub

> 自研多服务器管理面板 —— 融合 1Panel 的现代 UI、Coolify 的多服务器架构、宝塔的操作习惯

## 项目目标

一个**中文**、**多服务器**、**功能完整**的服务器管理面板，运行在主控服务器上，
通过 SSH 统一管理所有远程服务器。

## 核心功能

- 多服务器实时监控（CPU / 内存 / 磁盘 / 网络）
- Web SSH 终端（多 Tab，支持多台服务器）
- 网站管理 + 自动申请/续期 SSL 证书
- Docker 容器管理（日志 / 环境变量 / 启停）
- 应用一键部署（Git / Docker Compose）
- 数据库管理（MySQL / Redis）
- 文件管理器（支持多服务器）
- 系统工具（防火墙 / 计划任务 / 进程管理）

## 文档索引

| 文档 | 描述 |
|------|------|
| [架构设计](docs/architecture.md) | 整体技术架构、分层设计、数据流 |
| [模块设计](docs/modules.md) | 各模块详细实现思路 |
| [API 设计](docs/api-design.md) | 后端接口规范 |
| [前端设计](docs/frontend-design.md) | UI 组件设计、页面规划 |
| [数据库设计](docs/database-design.md) | 面板自身数据模型 |
| [部署方案](docs/deployment.md) | 生产环境部署指南 |

## 技术栈

```
后端    Go 1.22+  (Gin + gorilla/websocket + gorm)
前端    Vue 3 + Element Plus + Xterm.js
数据库  SQLite（面板配置）
SSH     golang.org/x/crypto/ssh
构建    Makefile + Docker
```

## 快速开始

```bash
# 开发环境
make dev

# 构建
make build

# 运行
./serverhub --port 9999 --data /opt/serverhub
```
