# 操作脉络（方案 D：Service 一等公民 + App 可选编组）

> 生成时间：2026-04-24  
> 与 `architecture-deploy.md` 配对阅读。本文回答"用户怎么用 ServerHub"。

---

## 一、核心心智

```
Server   是"地"   → 资源提供者（机器）
Service  是"物"   → 一个跑起来的东西（可独立部署）
App      是"袋子" → 可选，把多个协同的 Service 打包，获得统一域名/路由/编排
```

**规则**：

- 建 Service 不需要先建 App
- 一个 Service 可以属于 0 或 1 个 App（不可多属）
- 一个 App 下可有 1..N 个 Service，**通常同服务器**（跨服务器编排不支持）

---

## 二、三条典型路径

### 路径 A：单服务快速上线（最低仪式）

```
注册服务器 ──► 服务器详情 / "服务" Tab ──► 新增服务
                                           └─ 选类型 (static/native/docker/compose)
                                           └─ 创建初始 Release (Artifact + Env + Config + StartSpec)
                                           └─ Apply
```

适用：一个独立的定时任务、工具、基础设施服务（redis/nginx）等。

### 路径 B：多服务业务应用

```
注册服务器 ──► 应用列表 / 新建应用 (选服务器)
              ├─► 应用详情 / Services / 新增服务 → 前端
              ├─► 应用详情 / Services / 新增服务 → 后端
              └─► 应用详情 / Network / 新增路由：
                   /api/* → 后端:8080
                   /*     → 前端:5173
                   绑定域名 example.com + SSL
```

适用：有统一域名 / 需要 Nginx 路由分流 / 需要一起启停的一组服务。

### 路径 C：接管服务器上已有的服务

```
服务器详情 / 扫描发现 ──► 候选列表 (docker / compose / systemd / nginx)
                         └─► 对每个候选选择：
                              (a) 浮动纳管       → 只生成 Service，不建 App
                              (b) 归入已有应用   → 下拉选 App
                              (c) 新建应用并归入 → 输入 App 名称 + 自动建 App
                         └─► 接管成功：Service.current_release_id 指向"initial-import" Release
                                      Artifact 记录为 provider=imported（只读），不支持再部署
                                      要升级就新建 Release 指向新 Artifact
```

适用：老机器 / 别人已经部署好的系统。

---

## 三、Service 详情页结构

```
Service: my-backend                                     [状态徽标]
├─ Overview       运行状态、current_release、最近一次 deploy 结果、资源占用
├─ Releases       版本列表（label / 创建时间 / 引用 Artifact / Env / Config / 状态）
│                  操作：新建 Release、Apply、回滚（= Apply 历史 Release）
├─ Artifacts      制品列表（upload 上传、script/git/http 配置、docker image）
├─ Env            环境变量集列表（新建/编辑产生新 EnvVarSet）
├─ Config         配置文件集列表（新建/编辑产生新 ConfigFileSet）
├─ Deploys        部署历史（每次 Apply 一条 DeployRun，含日志）
├─ Logs           运行日志（journald / docker logs / 自定义路径）
└─ Settings       名称、服务器、类型、启动策略、自动回滚开关
```

### 3.1 新建 Release 向导（最常用入口）

```
Step 1: 选 Artifact           [已有下拉 | 新建 Artifact (上传 / 脚本 / docker / git / http)]
Step 2: 选 Env Set            [已有下拉 | 新建（JSON 编辑器） | 无]
Step 3: 选 Config Set         [已有下拉 | 新建（文件列表编辑器） | 无]
Step 4: StartSpec             [类型感知表单：docker 是 image+cmd+ports；native 是 cmd；static 是 index_file]
Step 5: Label + Note          [空 → 系统填 YYYY-MM-DD-N]
─────────────────────────────────────────
[ 保存为 Draft ]  [ 保存并 Apply ]
```

### 3.2 回滚

```
Releases Tab → 选历史某行 → [ 回滚到此 ] 按钮
              └─► 弹窗：显示新 Release label=「回滚自 vX」+ 继承原 Release 的三维
              └─► 确认 → Apply
```

实际执行：新建 Release（复制源 Release 三维）→ Apply → current_release_id 切换。保留完整审计链。

### 3.3 自动回滚

Service Settings：

```
☐ 部署失败时自动回滚到上一 active Release
   ( 默认关闭；开启后需设置健康检查超时 )
```

触发：DeployRun.status=failed → 查 Service.current_release_id 之前的 active Release → 以 trigger_source=auto_rollback Apply。

---

## 四、App 详情页结构

```
App: my-website                                [域名 example.com | 3 services]
├─ Overview       聚合状态、资源、近期事件
├─ Services       下属服务列表；新增 / 添加已有浮动服务 / 移出应用
├─ Network        Nginx 路由表 + 域名 + SSL
├─ Releases       AppReleaseSet 列表（M3）——业务版本
├─ Data           关联的数据库连接、备份
├─ Logs           跨服务聚合日志
└─ Settings       名称、BaseDir、主服务
```

### 4.1 Nginx 路由示例

```
server_name: example.com  (绑定 SSL cert id=12)

Routes:
┌─ path ──────┬─ upstream ──────────────────┬─ extra ────────┐
│ /api/       │ Service[后端 my-backend:8080] │ proxy_http 1.1 │
│ /admin/     │ Service[后台 my-admin:3000]   │                │
│ /           │ Service[前端 my-web:5173]     │ (静态化配置)    │
└─────────────┴─────────────────────────────┴────────────────┘

[ 保存 ] → 面板渲染 /etc/nginx/conf.d/my-website.conf → nginx -t && reload
```

upstream 字段内部存 `service:{id}:{port}`，展示时解析；也支持手写字面量。

---

## 五、服务器详情页结构

服务器降级为"资源视角 + 诊断"，但仍然是一级入口（导航："应用 / 服务器"并列）。

```
Server: prod-01 (127.0.0.1)              [online | capability=full]
├─ Overview       机器指标、capability、SSH 状态
├─ Services       本机已管理的 Service（含浮动）；扫描发现入口
├─ Docker         容器/镜像列表
├─ Nginx          站点文件、证书
├─ System         服务列表（systemd）、进程
├─ LogSearch      跨文件日志搜索
├─ Files          SFTP 目录浏览
├─ Terminal       交互式 shell
└─ Discover       扫描发现（容器/compose/systemd/nginx）
```

---

## 六、FAQ

**Q: 我只想跑一个 docker redis，不想建 App 怎么办？**  
A: 服务器详情 / Services / 新增服务；类型 docker，镜像 redis:7；跳过 Artifact 上传（provider=docker 直接 pull）。不用建 App。

**Q: 环境变量只改一个值，需要重新上传代码吗？**  
A: 不需要。Releases Tab / 新建 Release → Artifact 选"与当前一致"；Env 选"新建"编辑一个值；Apply 即可。三维正交。

**Q: 接管老系统后想升级怎么办？**  
A: 接管时生成的 initial Release 里 Artifact 是 `provider=imported`（只读）。升级时新建 Release 换成真实的 Artifact（比如 docker 新 tag / 上传新包）即可。

**Q: App 能跨服务器吗？**  
A: 不支持。App 强制 `server_id` 绑定。跨机编排超出 ServerHub 定位。

**Q: Release 最多保留多少？**  
A: 默认每 Service 保留最近 10 个；超出 FIFO 清理（释放磁盘）。可按 Service 配置。
