# 技术栈

## 后端 (`backend/go.mod`)

| 类别 | 库 | 版本 | 用途 |
|---|---|---|---|
| 语言 | Go | 1.25 | 单二进制 |
| Web | `github.com/gin-gonic/gin` | 1.9.1 | HTTP 框架 |
| ORM | `gorm.io/gorm` | 1.25.7 | 数据访问 |
| DB driver | `gorm.io/driver/sqlite` | 1.5.5 | SQLite + WAL |
| WS | `github.com/gorilla/websocket` | 1.5.3 | 终端 / 日志流 |
| SSH | `golang.org/x/crypto/ssh` | 0.41 | 远程命令执行 |
| SFTP | `github.com/pkg/sftp` | 1.13.10 | 远程文件 |
| JWT | `github.com/golang-jwt/jwt/v5` | 5.2.1 | 签发/校验 |
| 限流 | `golang.org/x/sync/semaphore` | latest | 并发上限 |
| 系统指标 | `github.com/shirou/gopsutil/v3` | 3.24.5 | 自身资源采集 |
| 加密 | `golang.org/x/crypto` | 0.41 | AES-GCM / bcrypt |
| TOTP | `github.com/pquerna/otp` | latest | MFA |
| YAML | `gopkg.in/yaml.v3` | 3.x | 配置文件 |

构建要求：`CGO_ENABLED=1`（SQLite 依赖）。

## 前端 (`frontend/package.json`)

| 类别 | 库 | 版本 |
|---|---|---|
| 框架 | vue | 3.4.21 |
| 构建 | vite | 5.2.0 |
| 类型 | typescript | 5.4.2 / vue-tsc 2.0.6 |
| 路由 | vue-router | 4.3.0 |
| 状态 | pinia | 2.1.7 |
| UI | naive-ui | 2.40.4 |
| HTTP | axios | 1.6.8 |
| 工具 | @vueuse/core | 10.9.0 |
| 时间 | dayjs | 1.11.10 |
| 图表 | echarts | 5.5.0 |
| 图标 | lucide-vue-next | 0.453.0 |
| 终端 | @xterm/xterm + addon-fit + addon-search | 6.0.0 |
| 编辑器 | codemirror 6 (lang-css/html/js/json/sql/xml/yaml) | 6.x |

包管理器：npm（项目根含 `package-lock.json`）。

## 工程约束

- 后端目录：仓库根 `backend/`，Go 模块名 `github.com/serverhub/serverhub`
- 前端目录：仓库根 `frontend/`，构建输出至 `backend/web/dist/`，由 `embed.FS` 嵌入
- SQLite 数据：`<DataDir>/serverhub.db`（默认 `/opt/serverhub/serverhub.db`）
- 日志：当前用 `log.Printf` + systemd journald；未引入 slog/lumberjack
- 测试：未规模化，部分包有 _test.go，无 CI 覆盖率门禁
