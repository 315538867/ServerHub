# 部署与发布模型（Service / Release 重设）

> 生成时间：2026-04-24  
> 状态：模型定稿（讨论结果）。实现按 M1/M2/M3 推进。

---

## 一、为什么重设

旧模型把 "可部署的一切" 塞在 `Service` 表 + `DeployVersion`（整表快照）里：

- `Service` 同时持有代码路径、启动命令、环境变量、配置文件、镜像名、compose 文件等 8+ 字段；字段边界与 Service.Type 耦合强，用户不知道什么时候该填哪个。
- `DeployVersion` 是"成功后整表快照"，改环境变量就得改 `Service` 本体，版本概念不稳定。
- 静态/二进制上传的文件没有"制品"概念，只有 `DesiredVersion` 字符串。
- 自定义拉取脚本（git pull / curl / 私有仓库）没有落脚点，只能硬塞 `startup.sh`。
- 回滚 = 覆盖 `Service` 字段回老快照，目标机上的实际产物可能已漂移，不是真原子。

重设的核心想法：**把"发布" = Release 立为一等公民**，由三维独立可版本化的组件组合而成。

---

## 二、领域模型

```
Server  ── 资源提供者（机器）
Service ── 一等公民（跑起来的一个东西；可独立部署，也可挂在 App 下）
App     ── 可选编组（多 Service 协同时提供统一域名 / Nginx 路由 / 编排）
```

### 2.1 部署领域：5 个一等公民

```
Service
  ├─ id / name / server_id / application_id (nullable)
  ├─ type: static | native | docker | compose
  ├─ current_release_id   ← 当前在跑的发布
  └─ 运行策略（健康检查 / 启停超时 / 失败自动回滚开关）

Release  ── 可部署的最小完整单位，是回滚的唯一单位
  ├─ id / service_id / label (v1 / 2026-04-24-1 / git-sha7)
  ├─ artifact_id     → Artifact
  ├─ env_set_id      → EnvVarSet  (nullable)
  ├─ config_set_id   → ConfigFileSet (nullable)
  ├─ start_spec      → JSON: 启动规约
  │                     docker: { image, cmd, args, ports, volumes, restart }
  │                     compose: { file_name, compose_profile }
  │                     static:  { index_file }
  │                     native:  { cmd, workdir_subpath }
  ├─ note / created_by / created_at
  └─ status: draft | active | rolled_back | archived

Artifact  ── 制品（类型化 ref + 可选拉取脚本）
  ├─ id / service_id
  ├─ provider: upload | script | git | http | docker
  ├─ ref           → upload: 面板本地相对路径
  │                  docker: image:tag[@sha256:digest]
  │                  git:    repo@ref
  │                  http:   url
  │                  script: 无 ref（由 pull_script 产生）
  ├─ pull_script   → provider=script 时的 bash（目标机上执行）
  ├─ checksum      → sha256（upload/http/script 产物；docker 存 digest）
  ├─ size_bytes
  └─ created_at

EnvVarSet  ── 环境变量集（独立版本，可被多 Release 复用）
  ├─ id / service_id / label
  ├─ content       → AES-GCM 加密的 JSON: [{key, value, secret}]
  └─ created_at

ConfigFileSet  ── 配置文件集（独立版本，可被多 Release 复用）
  ├─ id / service_id / label
  ├─ files         → JSON: [{name, content_b64, mode}]
  └─ created_at

DeployRun  ── 一次部署执行的记录（每 Apply 一次就一条）
  ├─ id / service_id / release_id
  ├─ status: running | success | failed | rolled_back
  ├─ trigger_source: manual | webhook | schedule | api | auto_rollback
  ├─ started_at / finished_at / duration_sec
  ├─ output        → 部署日志
  └─ rollback_from_run_id  → 若本次是回滚，指向源 run
```

### 2.2 组合规则

| 规则 | 说明 |
|---|---|
| `Release` 不可变 | 一旦 Apply 成功就不再修改；只能基于它再生新 Release |
| `Artifact / EnvVarSet / ConfigFileSet` 不可变 | 内容改动 = 生成新行 |
| 三维正交 | 改代码 → 新 Artifact；改环境变量 → 新 EnvSet；改配置 → 新 ConfigSet |
| 组合复用 | 新 Release 可以复用任意已存在的 Artifact/EnvSet/ConfigSet |
| `Service.current_release_id` 唯一反映"正在跑" | 回滚也是改它 + 重新 Apply |

---

## 三、Release 生命周期

```
[新建 Release (draft)]
   └── 用户选三维：Artifact + EnvSet + ConfigSet + StartSpec
[Apply]
   ├── 创建 DeployRun (status=running)
   ├── 目标机 workdir 准备
   ├── 按 Artifact.provider 拉取制品到目标机
   │     upload/http → 面板 SFTP 推送 或 目标机 curl
   │     docker      → docker pull image:tag (锁 digest 到 ref)
   │     git         → git clone/pull 到目标目录
   │     script      → 执行 pull_script；产物校验 sha256 反写 Artifact.checksum
   ├── 写 ConfigFileSet.files 到目标目录
   ├── 按 StartSpec 构建并执行启动命令（env 来自 EnvVarSet）
   ├── 健康检查（可选，超时视为失败）
   ├── 成功 → Service.current_release_id = release.id; Release.status = active
   └── 失败 → DeployRun.status = failed
              └── 若 Service.auto_rollback_on_fail=true 且有上一 active Release
                   → 自动触发对上一 Release 的 Apply（rollback_from_run_id 指向本 run）
                   → 成功则 DeployRun(新).trigger_source=auto_rollback,
                     原 DeployRun.status=rolled_back
```

### 3.1 回滚 = 重新 Apply 老 Release（Q6 选 B）

- 用户在版本列表点某个历史 Release → "回滚到此" → Apply 该 Release
- 产生新的 DeployRun，`rollback_from_run_id` 指向**当前失败或想替换的 run**
- 与普通部署同路径：保证目标机实际状态与 Release 定义一致，避免纸面切指针

### 3.2 部署失败自动回滚（Q2.2 确认）

- `Service.AutoRollbackOnFail bool`（默认 false）
- 触发条件：当次 DeployRun 失败 AND 存在上一条 `status=active` 的 Release
- 动作：以 `trigger_source=auto_rollback` 再启一个 DeployRun Apply 上一 Release
- 日志两次 run 都保留；UI 给失败 run 挂"已自动回滚到 vX"徽标

---

## 四、类型差异化：Artifact.provider × Service.type 矩阵

| Service.type | 常用 provider | ref 内容 | pull 阶段动作 | 启动阶段动作 |
|---|---|---|---|---|
| **static** | `upload` | `artifacts/${sid}/${sha}.tgz` | SFTP 推到目标机 → tar -xzf 到 workdir | nginx 指向 workdir（无进程） |
| **static** | `script` | — | 目标机执行 pull_script → 产物放到 workdir | 同上 |
| **native** | `upload` | `artifacts/${sid}/${sha}.bin` | SFTP 推到 workdir → chmod +x | `./bin ${cmd_args}` + env |
| **native** | `git` | `git@repo.git@ref` | git clone/pull → 执行构建脚本（可选） | 按 StartSpec.cmd |
| **docker** | `docker` | `image:tag@sha256:...` | `docker pull image:tag` → 记录 digest | `docker run --name {svc} ...` |
| **compose** | `upload` / `script` | compose.yml 内容 | 写到 workdir/docker-compose.yml | `docker compose up -d --build` |

---

## 五、面板存储与清理

- 面板数据目录：`${data_dir}/artifacts/${service_id}/${sha256}.${ext}`
- 每 Service 保留最近 N 个 Release（默认 10）；超出 FIFO 清理关联 Artifact（如果 Artifact 无其他 Release 引用）
- EnvVarSet / ConfigFileSet 跟随 Release 清理节奏；全量依赖计数策略保证不误删

---

## 六、App（应用）功能清单

App 仍然是**可选编组**。当 Service 独立使用时不需要它；多 Service 协同时建 App 获得下列增值：

| 能力 | 说明 |
|---|---|
| **域名 / 站点** | `App.Domain` + `App.SiteName` → 对应一张 Nginx server 配置 |
| **应用级 Nginx 路由表** | `AppNginxRoute{app_id, path, upstream_service_id?, upstream_port, raw, sort}`；upstream 可直接下拉选 Service + 端口 |
| **HTTPS / 证书** | 与 Domain 绑定；已有 SSL 模块 |
| **应用级发布集（AppReleaseSet）** | 打包下属多个 Service 的 Release 组合成"业务版本 v1.2"，支持一键 Apply / 一键回滚 |
| **一键启停** | 按依赖顺序启停所有下属 Service |
| **日志聚合** | 跨 Service 时间合流视图 |
| **指标卡片** | 累加下属 Service 的 CPU/Mem/Net |
| **应用工作区** | `App.BaseDir` 作为根目录约定；Service.WorkDir 默认 `${App.BaseDir}/${Service.Name}` |

### 6.1 AppReleaseSet（Q7 选 A，M3 已落地）

```
AppReleaseSet
  ├─ id / application_id / label
  ├─ items: [{service_id, release_id}]              JSON 文本列
  ├─ note / created_by / created_at / updated_at
  ├─ applied_at                                      最近一次 Apply/Rollback 完成时刻
  ├─ last_summary: [{service_id, run_id?,            JSON：每 Service 的本次结局
  │                  status, error?}]
  └─ status: draft | applying | success | partial | failed | rolled_back
```

- **创建** `CreateFromCurrent`：扫 App 下所有 Service，取非空 `Service.CurrentReleaseID` 拍快照；忽略无 CurrentRelease 的 Service；Label 留空时按 `YYYY-MM-DD-N` 兜底（同 App 当日序号递增）
- **Apply**：按 items 串行调用 `deployer.ApplyRelease`；**continue-on-failure**——单 Service 失败不中断后续，最终按成功/失败比例决算 `status`：
  - `fail == 0` → `success`
  - `success == 0` → `failed`
  - 其余 → `partial`
- **Rollback**：粒度落到 Service。对 last_summary 中 `status=success` 的每条，在该 Service 的 Release 时间线上取"排除当前 Release 后最近一条 active/archived"作为目标，串行反向 Apply；找不到历史的 Service 标 `skipped`。结束后整 set 状态置 `rolled_back`
- **并发幂等**：`Apply`/`Rollback` 入口走 GORM CAS（`status <> 'applying'` → `'applying'`），重复触发返回 `ErrAlreadyApplying`，前端走错误事件 `{code:"already_applying"}`

### 6.2 实现要点：与设计稿的差异（M3 落地）

| 维度 | 原设计稿（§6.1 旧版） | 实际落地 | 原因 |
|---|---|---|---|
| status 枚举 | `draft \| active \| rolled_back` | 6 态（增 `applying/success/partial/failed`） | 需要承载 SSE 进度与多 Service 决算 |
| 失败策略 | "任一失败整体 failed" | continue-on-failure + 比例决算 | 串行 N 个 Service，前面成功的不应因后面失败被忽略；用 partial 留观察窗口 |
| 回滚粒度 | "上一个 set 重走 Apply" | 本 set 内每 Service 各自找上一条历史 Release | set 级回滚要求"上一个 set"必然存在且 items 完全对齐，约束太强；service 级回滚总能定义 |
| 推送方式 | 未指定 | SSE（POST + `text/event-stream`） | `deployer.ApplyRelease` 已有 `onLine` 回调；多 Service 串行易超 Nginx `proxy_read_timeout`；EventSource 不支持 POST，前端走 fetch+ReadableStream |

**SSE 事件序列**（每次 Apply / Rollback）：

```
event: set_started      data: {set_id,total,items}
event: service_started  data: {service_id,release_id,idx,total}
event: service_line     data: {service_id,line}                 ← 多次
event: service_done     data: {service_id,run_id?,status,duration_sec,error?}
…
event: set_done         data: {status,summary}
event: done             data: {}
```

源码定位：`backend/api/apprelease/{service.go,handler.go}`、`backend/pkg/sse/writer.go`、前端 `frontend/src/api/apprelease.ts` (`runSseStream` 解析器)。

---

## 七、迁移路径

| 阶段 | 旧数据处置 |
|---|---|
| **M1** | 新表与旧 `deploy_versions` 并存；新建 Service 走新路径；现有 Service 仅展示旧 `DeployVersion` |
| **M2** | 迁移脚本：把历史 `DeployVersion` 拆成 Artifact（占位）+ EnvSet + ConfigSet + Release；保持旧快照表只读 |
| **M3** ✅ | 已删 `pkg/deployer/runner.go`、旧 deploy 链路 handler 收敛为只读；`AppReleaseSet` 上线（`backend/api/apprelease/`）；前端 `views/Apps/Releases.vue` 接 SSE。`deploy_versions` 表保留只读供历史查阅，新链路完全不再写入 |

---

## 八、API 草案（概览）

```
# Service
POST   /services                              新建 Service（不含 release；进空状态）
GET    /services/:id
GET    /services/:id/releases                 历史 Release 列表
POST   /services/:id/releases                 新建 Release（draft，需传 artifact/env/config/start_spec）
POST   /services/:id/releases/:rid/apply      部署/回滚统一入口
GET    /services/:id/deploy-runs              部署历史
GET    /services/:id/deploy-runs/:runid/log   部署日志流（WebSocket）

# Artifact
POST   /services/:id/artifacts                上传制品（multipart） or 声明 docker/git/script
GET    /services/:id/artifacts
POST   /services/:id/artifacts/:aid/probe     对 script/git/http 做一次试拉取（可选）

# EnvSet / ConfigSet
POST   /services/:id/env-sets
POST   /services/:id/config-sets

# App
POST   /apps                                  新建应用
GET    /apps/:id/services                     下属服务

# AppReleaseSet（M3 已落地）
GET    /apps/:id/release-sets                 列表（按 id desc）
POST   /apps/:id/release-sets                 从当前 Service.CurrentReleaseID 拍快照
GET    /apps/:id/release-sets/:rsid           详情
POST   /apps/:id/release-sets/:rsid/apply     SSE 流式应用（Content-Type: text/event-stream）
POST   /apps/:id/release-sets/:rsid/rollback  SSE 流式回滚（按 service 级历史定位上一条）
```

---

## 九、前端导航（方案 D-2）

```
Dashboard / 应用 / 服务器 / 通知 / 设置
```

- 无全局 Deploy 顶层入口；"部署"是 Service 详情里的一级 Tab
- 服务器详情新增 **服务** Tab：列该服务器上所有 Service（含浮动），"新增服务"入口
- 应用详情：Overview / Services / Network / Releases / Data / Ops
- Service 详情：Overview / Releases / Env / Config / Deploys / Logs / Settings

---

## 十、开放问题（已定稿）

| 问题 | 选择 |
|---|---|
| Q1 制品存储位置 | 面板本地 `data_dir/artifacts/` |
| Q2 版本标签策略 | 用户自填，系统兜底时间戳 `YYYY-MM-DD-N` |
| Q3 EnvSet/ConfigSet 是否独立表 | 独立表，三维正交 |
| Q4 Release 与 Service 关系 | Service.current_release_id 指针 |
| Q5 脚本执行位置 | 目标服务器 |
| Q6 回滚是"切指针"还是"重新 Apply" | 重新 Apply |
| Q7 AppReleaseSet | M3 已落地（apply/rollback 走 SSE，service 级回滚） |
| 失败自动回滚 | 需要（Service 级开关） |
| 灰度/定时更新 | 暂不做 |
