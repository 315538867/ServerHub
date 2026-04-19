# 模块实现设计

## 模块一：服务器管理

### 功能范围
- 添加/编辑/删除服务器（SSH Key 或密码认证）
- 实时监控（CPU / 内存 / 磁盘 / 网络 / 负载）
- 历史趋势图（24 小时，每 5 分钟一点）
- SSH 心跳检测（30 秒），连续失败 3 次标记离线

### 实现思路

**添加服务器流程：**
```
用户填写 [IP / 端口 / 用户名 / 认证方式（Key or 密码）]
    → 测试 SSH 连接（超时 10s）
    → 成功 → AES-256-GCM 加密存储凭据 → 写入 DB → 加入连接池
    → 失败 → 返回详细错误（连接拒绝 / 认证失败 / 超时 / 主机不可达）
```

**指标采集脚本（纯 shell，无解释器开销，延迟 <10ms）：**

> **性能说明：** 直读 `/proc` 文件系统，无解释器冷启动（Python ~200ms → shell ~3ms）。
> CPU 百分比由 Go 侧对两次采集结果做差值，无需 `interval=1` 阻塞等待。
> 降级场景（非 Linux）再使用 psutil，但必须改为 `interval=0`。

```bash
# 主采集：纯 shell，从 /proc 直接读取
CPU_STAT=$(awk '/^cpu /{print $2+$4, $2+$3+$4+$5+$6+$7+$8}' /proc/stat)
MEM=$(awk '/MemTotal/{t=$2}/MemAvailable/{a=$2}END{print t,a}' /proc/meminfo)
DISK=$(df -B1 / | awk 'NR==2{print $2,$3}')
NET=$(awk 'NR>2{split($1,a,":");if(a[1]!="lo"){r+=$2;s+=$10}}END{print r,s}' /proc/net/dev)
LOAD=$(awk '{print $1,$2,$3}' /proc/loadavg)
UPTIME=$(awk '{print int($1)}' /proc/uptime)
echo "$CPU_STAT|$MEM|$DISK|$NET|$LOAD|$UPTIME"

# 降级（非 Linux，psutil 必须用 interval=0）：
# python3 -c "
# import psutil, time
# print(psutil.cpu_percent(interval=0),  # interval=0: 不阻塞
# psutil.virtual_memory().percent, ...)"
```
```

---

## 模块二：Web 终端

### 功能范围
- 浏览器内 SSH 终端（Xterm.js + PTY）
- 多 Tab 同时连接多台服务器（每 Tab 独立 WS + SSH Session）
- 终端尺寸自适应窗口
- 终端内容搜索（SearchAddon）
- 断线提示重连

### 实现思路

**后端 WebSocket Handler：**
```go
func TerminalHandler(c *gin.Context) {
    serverID := c.Query("serverId")
    ws, _ := upgrader.Upgrade(c.Writer, c.Request, nil)
    defer ws.Close()

    executor := pool.Get(serverID)
    if executor == nil {
        ws.WriteMessage(websocket.TextMessage, []byte("\r\n服务器未连接\r\n"))
        return
    }
    rows, _ := strconv.Atoi(c.DefaultQuery("rows", "24"))
    cols, _ := strconv.Atoi(c.DefaultQuery("cols", "80"))
    executor.OpenTerminal(ws, rows, cols)
}
```

**前端 xterm.js（TypeScript）：**
```typescript
const term = new Terminal({ cursorBlink: true, fontSize: 14 })
const fitAddon = new FitAddon()
term.loadAddon(fitAddon)

const ws = new WebSocket(
  `wss://host/panel/api/ws/terminal?serverId=${id}&token=${token}`
)
term.onData(data => ws.send(data))
ws.onmessage = e => term.write(e.data)

new ResizeObserver(() => {
    fitAddon.fit()
    ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
}).observe(termContainer)
```

---

## 模块三：网站 / Nginx 管理

### 功能范围
- 添加站点向导（静态文件 / 反向代理 / PHP）
- 自动生成 nginx 配置（Go template）
- nginx 配置在线编辑（CodeMirror + nginx 语法高亮）
- 保存前 `nginx -t` 验证，失败自动恢复 .bak
- 常用伪静态模板（Vue SPA / 反代 / Rewrite）
- 访问日志 / 错误日志查看（实时 + 历史）

### 配置保存流程

```
用户编辑 → 点击保存
    → 备份原文件（.bak.时间戳）
    → 写入新内容
    → Executor.Run("nginx -t")
    → 成功 → nginx reload → 返回 {code: 0}
    → 失败 → 恢复 .bak → 返回 {code: 2003, msg: "nginx 错误详情"}
```

**站点类型向导（三步）：**
```
步骤 1：基本信息（域名 / 目标服务器 / 站点类型）
步骤 2：类型配置
    - 静态：根目录路径
    - 反代：上游地址（http://127.0.0.1:8080）
    - PHP：PHP 版本（7.4 / 8.0 / 8.1 / 8.3）
步骤 3：SSL（不启用 / 自动申请 / 手动上传）
```

---

## 模块四：SSL 证书管理

### 功能范围
- 证书列表（域名 / 到期时间 / 颁发机构 / 状态）
- 自动申请 Let's Encrypt（HTTP-01 验证）
- 申请进度实时推送（SSE）
- 一键续期 / 自动续期（到期前 30 天）
- 证书到期告警
- 手动上传证书

### 自动申请流程

```
用户填写域名 → 点击申请
    → 检查 DNS 是否已解析到本机
    → 写 acme-challenge 临时 nginx 配置 → reload
    → 执行 certbot certonly --webroot -w /opt/serverhub/acme -d <domain>
    → 解析 openssl x509 获取到期时间、颁发机构
    → 记录到 DB → 删临时配置 → reload
    → SSE 推送完成事件
```

**自动续期（调度器，每天凌晨 2 点）：**
```go
func checkAndRenewCerts() {
    for _, cert := range db.GetCerts() {
        daysLeft := time.Until(cert.ExpireAt).Hours() / 24
        if daysLeft < 30 {
            runCertbotRenew(cert.Domain)
            updateCertExpiry(cert)
            sendNotification("SSL 证书已自动续期: " + cert.Domain)
        }
    }
}
```

---

## 模块五：Docker 容器管理

### 功能范围
- 容器列表（名称 / 镜像 / 状态 / 端口 / CPU / 内存实时）
- 启动 / 停止 / 重启 / 删除
- 实时日志流（WebSocket）
- 容器详情（环境变量，敏感字段脱敏）
- 镜像列表 / 拉取 / 删除

### 执行方式

```
本机 Docker → Docker SDK（github.com/docker/docker/client）
远程 Docker → SSH 执行 docker CLI，解析 JSON 输出

选 CLI 原因：
  - 无需配置 Docker Remote API TLS
  - 不暴露 Docker socket 到网络
  - 输出格式稳定
```

**远程命令示例：**
```bash
docker ps -a --format '{{json .}}'
docker stats <id> --no-stream --format '{{json .}}'
docker inspect <id>
docker logs -f --tail=100 <id>   # 日志流
```

**环境变量脱敏：**
```go
// 含 PASSWORD/SECRET/KEY/TOKEN/PASS/PRIVATE 的 key，值显示为 "***"
// 前端提供「显示」按钮，需要用户确认后再请求明文
```

---

## 模块六：应用部署

### 功能范围
- Git 仓库部署（GitHub / Gitee / GitLab / 私有 Git）
- Docker Compose 部署（粘贴或选服务模板）
- 内置服务模板库（PostgreSQL / MySQL / Redis / MongoDB / Minio 等）
- Push-to-Deploy Webhook（GitHub/Gitlab/Gitea/Gitee）
- 部署流水线视图（步骤状态 + 实时日志）
- 环境变量加密管理
- 部署历史（commit hash / 状态 / 耗时）
- 一键回滚

### 部署引擎（nohup 模式）

**核心原则：部署命令脱离 SSH Session 生命周期，SSH 断开不影响部署。**

```go
func (s *DeployService) Deploy(app *model.DeployApp, deployID string) {
    logFile := fmt.Sprintf("/opt/serverhub/data/deploy-logs/%s.log", deployID)

    script := fmt.Sprintf(`
set -e
cd %s
echo "[STAGE:pull]" >> %s
git fetch origin && git reset --hard origin/%s >> %s 2>&1
echo "[STAGE:build]" >> %s
%s >> %s 2>&1
echo "[STAGE:start]" >> %s
docker compose up -d >> %s 2>&1
echo "[STAGE:done]" >> %s
`, app.WorkDir, logFile, app.Branch, logFile,
       logFile, app.BuildCmd, logFile,
       logFile, logFile, logFile)

    // RunBackground：nohup 后台运行，返回 PID
    pid, _ := executor.RunBackground(ctx, script, logFile)

    // 异步流式推送日志（tail -f，单个长连接，非轮询）
    go s.streamLog(ctx, deployID, logFile, pid)
}

// streamLog 实现：用 tail -f 单命令持续输出，避免 500ms 轮询
// executor.RunStream(ctx, "tail -f -n 0 "+logFile, wsLineWriter)
// 同时监听 kill -0 <pid> 判断部署进程是否结束
```

**部署阶段识别（前端根据标记更新 UI）：**
```
[STAGE:pull]  → 步骤1「拉取代码」进行中
[STAGE:build] → 步骤2「构建镜像」进行中
[STAGE:start] → 步骤3「启动服务」进行中
[STAGE:done]  → 全部完成，状态变为成功
set -e 失败   → 当前阶段标记失败，停止后续步骤
```

**Push-to-Deploy Webhook 接收：**
```
POST /panel/webhooks/github/:appId
    → 验证 X-Hub-Signature-256 HMAC 签名
    → 解析 ref，匹配 app.Branch
    → 异步触发部署
    → 返回 202 Accepted

POST /panel/webhooks/gitlab/:appId   # GitLab
POST /panel/webhooks/gitea/:appId    # Gitea / Gitee
```

**内置服务模板（Compose 模板 + 变量声明）：**

| 分类 | 模板名称 |
|------|---------|
| 数据库 | PostgreSQL 16 / MySQL 8 / Redis 7 / MongoDB 7 |
| 存储 | Minio |
| 监控 | Grafana / Uptime Kuma |
| 工具 | Nginx（PHP 站点用） |

每个模板包含：`compose`（YAML 内容）+ `variables`（变量定义，含默认值/是否必填/是否脱敏）

---

## 模块七：文件管理

### 功能范围
- 文件浏览（树形目录 + 列表，多服务器切换）
- 上传 / 下载 / 创建目录 / 删除 / 重命名 / 移动
- 在线编辑（CodeMirror 6，支持 nginx/yaml/json/shell/toml）
- 权限查看与修改（chmod）
- 压缩（tar.gz）/ 解压

### 统一接口

```go
type FileSystem interface {
    List(path string) ([]FileInfo, error)
    Read(path string) (io.ReadCloser, error)
    Write(path string, content io.Reader, mode os.FileMode) error
    Delete(path string) error
    Rename(oldPath, newPath string) error
    Mkdir(path string) error
    Stat(path string) (FileInfo, error)
    Chmod(path string, mode os.FileMode) error
    Compress(paths []string, destPath string) error
    Extract(srcPath, destPath string) error
}
// LocalFS  → os 标准库
// RemoteFS → github.com/pkg/sftp
```

**在线编辑安全限制：**
- 非 UTF-8 文件：提示二进制，拒绝编辑
- 文件大小 > 2MB：提示过大，只允许下载
- 保存前备份：`.filename.bak.时间戳`

**文件下载必须流式传输（性能约束）：**
```go
// 禁止 io.ReadAll（下载大文件会 OOM）
// 正确：SFTP → HTTP Response，io.Copy 流式，内存消耗恒定
rc, _ := remoteFS.Read(path)
defer rc.Close()
io.Copy(c.Writer, rc)  // 内存 = 单次 buffer ~32KB，与文件大小无关
```

---

## 模块八：数据库管理

### MySQL

- 连接配置（密码 AES 加密存储）
- 数据库列表 / 建库 / 删库
- 用户列表 / 建用户 / 授权
- SQL 执行器：默认只读，写入需切换模式，单次返回 ≤1000 行
- 数据导出：mysqldump + gzip，流式下载，不落盘面板服务器
- 连接数 / 状态信息（SHOW STATUS）

### Redis

- 连接配置（密码 AES 加密）
- 状态（INFO ALL 解析：内存 / Key 数 / 命中率 / 版本）
- Key 浏览：SCAN 分页（避免 KEYS 阻塞）+ 前缀过滤
- Flushdb：需要二次确认 + 写入审计日志

### SQL 执行器安全

```go
// 只读模式：仅允许 SELECT / SHOW / DESCRIBE / EXPLAIN
// 写入模式：用户主动切换，有审计记录
// 结果限制：SELECT 自动追加 LIMIT 1000
```

---

## 模块九：系统工具

### 防火墙（自动检测 ufw / firewalld）

```go
func DetectFirewall(e Executor) string {
    if out, _, _ := e.Run(ctx, "which ufw"); out != "" { return "ufw" }
    if out, _, _ := e.Run(ctx, "which firewall-cmd"); out != "" { return "firewalld" }
    return "iptables"
}
// 操作时根据检测结果执行对应命令
```

### 计划任务（Cron）

```go
// 读取：crontab -l
// 添加：解析 → 追加 → 回写 crontab
// 展示：将 "0 2 * * *" 解析为可读文字 "每天 02:00"
// 最近执行记录：需面板侧单独记录（目标服务器 cron 日志位置不一）
```

### 进程管理

```bash
# Top 20 by CPU
ps aux --sort=-%cpu | head -21 | awk 'NR>1{printf "{\"user\":\"%s\",\"pid\":%s,\"cpu\":%s,\"mem\":%s,\"cmd\":\"%s\"}\n",$1,$2,$3,$4,$11}'
```

Kill 进程：需要二次确认 + 审计记录

### systemd 服务

```bash
# 列表
systemctl list-units --type=service --no-pager --output=json
# 操作
systemctl <start|stop|restart|enable|disable> <service>
# 日志
journalctl -u <service> -n 100 --no-pager
```

---

## 模块十：监控告警

### 告警规则（可配置）

| 指标 | 默认阈值 | 持续时间 | 表现 |
|------|---------|---------|------|
| CPU 使用率 | > 90% | 持续 5 分钟 | Dashboard 红点 + 通知 |
| 内存使用率 | > 85% | 持续 3 分钟 | Dashboard 红点 + 通知 |
| 磁盘使用率 | > 80% | 立即 | Dashboard 橙色警告 |
| SSL 证书到期 | < 30 天 | 立即 | Dashboard 橙色警告 |
| 服务器离线 | 心跳连续失败 3 次 | 立即 | Dashboard 灰色 + 通知 |

**去重机制：** 同一规则触发后，30 分钟内不重复推送通知

### 数据保留

| 数据 | 策略 |
|------|------|
| 实时指标 | 内存缓存最近 60 个采样点（5 分钟） |
| 历史指标 | SQLite 保留 24 小时（每 5 分钟聚合一条） |
| 告警记录 | SQLite 永久保留 |

**指标清理（每小时执行一次）：**
```sql
DELETE FROM server_metrics WHERE collected_at < datetime('now', '-24 hours');
```
> 调度器中每小时触发，而非每天。单次删除量约 5 台服务器 × 12 行 = 60 行，
> SQLite WAL 模式下不阻塞读取，清理耗时 <1ms。

---

## 模块十一：通知 / Webhook

### 功能范围
- **原生系统通知**（桌面模式，零配置）
- 外部 Webhook 渠道管理（企业微信 / 钉钉 / Telegram / 自定义 HTTP，服务器模式）
- 告警事件路由（指定哪类事件发哪个渠道）
- 消息模板（Go template 语法，变量替换）
- 测试发送
- 通知发送历史

### 通知分发器（双模式统一接口）

```go
type Notifier interface {
    Send(event AlertEvent) error
}

// DesktopNotifier（build tag: desktop）
// 调用 beeep，直接触发系统通知 + 更新托盘图标
type DesktopNotifier struct {
    tray *TrayManager
}
func (n *DesktopNotifier) Send(event AlertEvent) error {
    beeep.Notify("ServerHub", event.Message, "")
    n.tray.UpdateStatus(event)
    return nil
}

// WebhookNotifier（build tag: server）
// 遍历匹配的 notification_channels，发送 HTTP 请求
type WebhookNotifier struct {
    db *gorm.DB
}
```

### 原生通知（桌面模式）

```go
// github.com/gen2brain/beeep
// 跨平台：macOS 通知中心 / Linux libnotify / Windows Toast

beeep.Notify(
    "ServerHub — CPU 告警",
    "Server 1：CPU 持续 > 90%（当前 94%）",
    "",  // 可指定图标路径
)
```

**通知触发时机（与服务器模式相同的告警规则）：**
- 服务器离线（心跳连续失败 3 次）
- CPU > 90% 持续 5 分钟
- 内存 > 85% 持续 3 分钟
- 磁盘 > 80%（立即）
- SSL 证书 < 30 天到期
- 部署完成 / 部署失败

### 外部 Webhook 渠道（服务器模式 / 可选）

**企业微信机器人（Markdown 消息）：**
```go
// POST https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=<key>
// Body: {"msgtype":"markdown","markdown":{"content":"..."}}
```

**钉钉机器人：**
```go
// POST https://oapi.dingtalk.com/robot/send?access_token=<token>
// Body: {"msgtype":"text","text":{"content":"..."}}
// 可选 secret 签名验证
```

**Telegram：**
```go
// POST https://api.telegram.org/bot<token>/sendMessage
// Body: {"chat_id":"...","text":"...","parse_mode":"Markdown"}
```

**自定义 HTTP Webhook：**
```go
// POST 到用户配置的 URL，自定义 Headers + Body 模板
// Body 模板支持变量：{{.ServerName}} {{.MetricValue}} {{.Threshold}} {{.Time}}
```

**消息模板示例：**
```
服务器告警
服务器：{{.ServerName}}（{{.ServerIP}}）
指标：{{.MetricName}} 当前值 {{.MetricValue}}（阈值 {{.Threshold}}）
时间：{{.Time}}
```

---

## 模块十二：系统托盘（桌面模式）

### 功能范围
- 菜单栏 / 系统托盘图标（macOS / Linux / Windows）
- 图标状态反映全局服务器健康度
- 右键菜单：服务器快速状态 + 打开面板 + 退出
- 接收调度器告警事件，更新图标并触发系统通知
- 面板启动时自动打开浏览器（可配置关闭）

### 实现

```go
// tray/tray_desktop.go  (+build desktop)

func InitTray(alertCh <-chan AlertEvent) {
    systray.Run(func() {
        systray.SetTitle("ServerHub")
        systray.SetTooltip("ServerHub — 全部在线")
        setIcon(iconGreen)

        mOpen    := systray.AddMenuItem("打开面板", "")
        systray.AddSeparator()
        mStatus  := systray.AddMenuItem("全部在线 (3/3)", "")
        mStatus.Disable()
        systray.AddSeparator()
        mQuit    := systray.AddMenuItem("退出", "")

        go func() {
            for {
                select {
                case event := <-alertCh:
                    updateTrayState(event)
                case <-mOpen.ClickedCh:
                    browser.OpenURL("http://localhost:9999/panel/")
                case <-mQuit.ClickedCh:
                    systray.Quit()
                }
            }
        }()
    }, func() {
        // 清理资源
    })
}

func updateTrayState(event AlertEvent) {
    switch event.Severity {
    case SeverityCritical:
        setIcon(iconRed)
        systray.SetTooltip("ServerHub — " + event.Message)
    case SeverityWarn:
        setIcon(iconOrange)
    default:
        setIcon(iconGreen)
        systray.SetTooltip("ServerHub — 全部在线")
    }
    // 同时触发系统通知
    beeep.Notify("ServerHub", event.Message, "")
}
```

```go
// tray/tray_server.go  (+build server)
// 空实现，编译时完全不引入 systray/beeep 依赖

func InitTray(alertCh <-chan AlertEvent) {}
```

### 图标状态规则

| 状态 | 图标 | 条件 |
|------|------|------|
| 全部正常 | 🟢 绿色圆点 | 无告警，所有服务器在线 |
| 有警告 | 🟠 橙色圆点 | 有未处理告警（磁盘 / SSL 到期预警） |
| 有严重告警 | 🔴 红色圆点 | 服务器离线 / CPU/内存超阈值 |
| 连接中 | ⚪ 灰色圆点 | 面板启动中，连接池初始化 |
