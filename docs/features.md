# 功能清单

按模块枚举当前面板提供的能力。

## 服务器

- 添加/编辑/删除受管服务器（密码或 SSH 私钥）
- 连通性测试 + 即时指标采集
- 自动周期采集（默认 5s）：CPU、内存、磁盘、Load1、uptime
- 历史趋势查询（折线图）
- 在线/离线状态自动判定（基于采集失败）

## Web 终端

- 浏览器内 SSH 终端（xterm.js）
- WS 鉴权走 `?token=` query
- 支持 resize、复制、查找

## Docker

- 容器列表（状态、镜像、端口）
- 启停 / 重启 / 删除
- 实时日志流（WS + 行过滤）
- inspect JSON 查看
- 镜像列表 / 拉取（流式日志）/ 删除

## 文件

- SFTP 浏览（按目录）
- 上传 / 下载 / 删除 / 重命名
- 在线编辑（CodeMirror 6 多语言高亮）

## 系统

- 防火墙规则（ufw / firewalld 自动识别）
- cron 任务（增删改）
- 进程列表 + kill
- systemd 服务列表 + 启停 + 实时日志（WS）

## Nginx

- 站点列表（available / enabled 区分）
- 配置在线编辑（自动 `nginx -t` 校验）
- enable / disable / reload / restart
- access / error 日志实时流（WS + 过滤）

## SSL

- 证书列表（含到期天数）
- Let's Encrypt 申请（webroot，certbot 流式日志）
- 手动上传证书（cert + key）
- 续期 / 删除
- 扫描已有 letsencrypt 目录批量导入
- 周期性到期巡检（scheduler）

## 应用（Application）

- 应用 = 服务器 + 部署 + 数据库 + 域名 + 反代路由 的聚合视图
- 5 Tab：概览 / 部署 / 网络 / 运维 / 数据
- 路由配置自动写回 nginx 站点片段

## 部署

- 四类执行器：docker、docker-compose、原生命令、静态站点（static）
- 静态站点：上传 `dist.zip` / `dist.tar.gz` 到 `_pending/`，解压归档到 `releases/<ts>/`，原子切换 `current` 软链；回滚即切换软链（秒级）
- env_vars AES 加密存入库，运行时注入远端
- 手动触发 + Webhook 触发（密钥校验）
- 实时输出（WS）+ 历史日志列表
- 期望版本 / 实际版本对账（reconciler 自动同步）
- 版本历史快照：每次成功部署写入 `deploy_versions`（类型、目录、compose、镜像、runtime、config_files、env_vars 密文），每个 deploy 保留最近 7 条，FIFO 淘汰
- 回滚：`POST /deploys/:id/rollback` 回到上一快照；`POST /deploys/:id/versions/:vid/rollback` 按指定快照重新部署（SSE 输出）
- 定时清理日志（保留期可配置）

## 数据库

- DBConn CRUD（MySQL / Redis）
- 连接测试

## 告警

- 规则：CPU、内存、磁盘、离线
- 操作符：gt / lt / eq；持续时间触发
- 通知渠道：企业微信、钉钉、Telegram、自定义 Webhook
- 渠道 URL AES 加密
- 事件历史 + 当前 firing/resolved 状态

## 远端日志搜索

- 源：docker 容器、systemd journald、nginx access、nginx error
- 选项：正则 / 大小写 / 时间窗（30m~7d）/ 上下文行（0~10）/ 上限（≤2000）
- 服务端 8 并发上限，超出 `429`
- 前端 .txt 一键导出

## 审计

- 全请求异步入库（user/ip/method/path/body/status/duration）
- 查询：username 与 path 前缀匹配（走索引）
- 90 天滚动保留

## 用户与安全

- JWT 登录（支持 MFA / TOTP）
- 登录限流（IP 滑窗，超限锁定）
- 修改密码
- 单 admin 角色（暂无细粒度 RBAC）

## 系统设置

- KV 表：当前已用 `deploy_log_keep_days`
- 可扩展任意全局开关

## ServerHub 自身

- `/system/self`：本进程 CPU/内存/goroutines/uptime/连接数（gopsutil）
- Dashboard 工作台展示自身资源占用

## 本机即主服务器

- 启动时自动创建 `id=1, type=local` 的本机服务器记录（不可编辑/删除）
- 本机执行通过 `os/exec` + `creack/pty`，不走 SSH 回环
- 指标采集走 gopsutil（CPU/内存/磁盘/Load/Uptime）
- 终端、Docker/systemd/防火墙/cron、Nginx/SSL/文件、部署均支持本机作为目标
