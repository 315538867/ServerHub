# 03 — 领域模型

> 范围: domain/ 层全部实体 + 关系 + 状态机
> 不变量: 见每实体 "Invariants" 段,所有不变量必须在 BeforeSave/Update GORM hook 或 usecase 入口强制

---

## 1. 实体关系图(ER)

```
┌──────────┐ 1     N ┌──────────┐
│  User    │────────▶│ Session  │
└──────────┘         └──────────┘

┌──────────┐ 1   N  ┌──────────┐ 1   N  ┌──────────┐
│  Server  │───────▶│ Network  │        │  Metric  │◀────┐
└────┬─────┘        └──────────┘        └──────────┘     │
     │ 1                                                  │ N
     │ N                                                  │
     ▼                                                    │
┌──────────┐                                              │
│ Service  │──────────────────────────────────────────────┘
│ ┌──────┐ │
│ │ Type │ │  enum: docker | compose | native | static
│ └──────┘ │
│ ┌──────┐ │
│ │State │ │  state machine: '' → Syncing → Synced / Error
│ └──────┘ │
└────┬─┬───┘
     │ │ 1     N ┌────────────┐ 1   1 ┌──────────┐
     │ └────────▶│  Release   │──────▶│ Artifact │
     │          │ ┌────────┐ │       └──────────┘
     │          │ │ Status │ │ 1   0..1 ┌──────────┐
     │          │ └────────┘ │─────────▶│ EnvVarSet│
     │          │   draft    │         └──────────┘
     │          │   active   │ 1   0..1 ┌──────────┐
     │          │   archived │─────────▶│ConfigSet │
     │          │ rolled_back│         └──────────┘
     │          │            │
     │          │ StartSpec  │ typed JSON
     │          │  ┌──────┐  │ docker  | compose | native | static
     │          │  └──────┘  │
     │          └─────┬──────┘
     │                │ 1     N
     │                ▼
     │          ┌────────────┐
     │          │ DeployRun  │  审计/派生 LastStatus 的源
     │          │ ┌────────┐ │
     │          │ │ Status │ │  running|success|failed|rolled_back
     │          │ └────────┘ │
     │          └────────────┘
     │
     │ N   1 ┌──────────────┐ 1   N ┌────────────────┐
     └──────▶│ Application  │──────▶│ IngressRoute   │
            │ ┌──────────┐ │       │ ┌────────────┐ │
            │ │ExposeMode│ │       │ │ Path/Site  │ │
            │ └──────────┘ │       │ │ Upstream   │ │
            │  none|path   │       │ └────────────┘ │
            │  |site       │       └────────────────┘
            └──────┬───────┘
                   │ 1   0..1
                   ▼
            ┌──────────────┐
            │ DBConnection │
            └──────────────┘
```

## 2. 实体目录(domain/ 包)

| 文件 | 实体 | 主键 | 备注 |
|---|---|---|---|
| `domain/user.go` | User, Session | id | TOTP secret 加密 |
| `domain/server.go` | Server, Network | id | Network 内嵌 JSON |
| `domain/service.go` | Service + State 状态机 | id | runtime/state 强类型 |
| `domain/release.go` | Release + StartSpec(typed) | id | StartSpec 4 类 builder |
| `domain/release_three.go` | Artifact, EnvVarSet, ConfigFileSet | id | 三维 |
| `domain/deployrun.go` | DeployRun + RunStatus | id | 不可变,只插入 |
| `domain/application.go` | Application + ExposeMode | id | Status 派生,不存 |
| `domain/ingress.go` | IngressRoute, NginxConfig | id | site/path 路由 |
| `domain/database.go` | DBConnection, Backup | id | |
| `domain/observability.go` | Metric, Audit, Alert | id | |

## 3. 关键状态机

### 3.1 Service.State

```
        ┌────────┐
   ─────│  ''    │ (初始,从未参与 reconcile)
        │ Initial│
        └───┬────┘
            │ enqueue reconcile
            ▼
        ┌────────┐  reconcile 失败   ┌────────┐
        │Syncing │──────────────────▶│ Error  │
        └───┬────┘                   └───┬────┘
            │ apply 成功                 │ retry
            ▼                            │
        ┌────────┐ ◀──────────────────── ┘
        │ Synced │
        └────────┘
```

**转移表**(`domain/service_state.go`):

| from | to | 触发者 | 校验 |
|---|---|---|---|
| `''` | `Syncing` | reconciler/usecase | 无 |
| `Synced` | `Syncing` | reconciler/usecase | 无 |
| `Error` | `Syncing` | reconciler retry/manual | 无 |
| `Syncing` | `Synced` | usecase apply 成功 | DeployRun=success |
| `Syncing` | `Error` | usecase apply 失败 | DeployRun=failed |
| `*` | `''` | **禁止** | 永不回退 |
| `Syncing` | `Syncing` | **禁止** | 原子守卫 |

**强制方式**:
- `domain.Service.CanTransitionTo(target State) error`
- repo.Service.Update 调用前必须 CanTransitionTo
- 加 `Service` GORM `BeforeUpdate` hook 兜底校验

### 3.2 Release.Status

```
       ┌────────┐  Apply        ┌────────┐
   ────│ draft  │──────────────▶│ active │
       └────────┘               └───┬────┘
                                    │
                ┌───────────────────┴───────────────┐
                │                                   │
                ▼ 用户回滚到本条                    ▼ 新 active 出现
        ┌──────────────┐                      ┌──────────┐
        │ rolled_back  │                      │ archived │
        └──────────────┘                      └──────────┘
```

**不变量**:
- Apply 后 StartSpec/ArtifactID/EnvSetID/ConfigSetID **永不可改**
- 同 Service 至多一条 `active`(BeforeSave 钩子校验)
- `archived` 是终态,不能复活;`rolled_back` 也是终态(回滚一次后再要恢复必须新建 Release)

**强制方式**:
- `Release.BeforeUpdate` hook:若 Old.Status != draft,拒绝修改 StartSpec/Artifact/EnvSet/ConfigSet 4 字段
- usecase 层兜底:`ReplaceCurrentRelease(svcID, newRelID)` 一次写两行 update 到 archived

### 3.3 DeployRun.Status

```
running ─▶ success
        ─▶ failed ─▶ rolled_back (autoRollback 触发)
```

DeployRun **永不更新**(只插入),回滚是新插入一条 `Status=rolled_back` 的 DeployRun + Service.CurrentReleaseID 指回上一条 active。

### 3.4 Application.Status(派生,不存)

来自下属 Service.State 聚合:
- 全部 Synced → `online`
- 任一 Error → `error`
- 任一 Syncing 且无 Error → `deploying`
- 全部 `''` 初始态 → `unknown`
- 无 Service → `empty`

实现位置:`derive/application.go::AppStatus(repo, ids)`,**不进 Application 表**。

### 3.5 Server.Status(派生,不存)

来自最近 N 秒的 Metric:
- 有最近 60 秒内 Metric → `online`
- 60s-300s 内 → `lagging`
- > 300s 或无 → `offline`

实现位置:`derive/server.go::ServerStatus(repo, ids)`,**不进 Server 表**。

## 4. 不变量清单(必须 hook/usecase 强制)

| ID | 不变量 | 强制位置 |
|---|---|---|
| INV-1 | Service.CurrentReleaseID 必须指向 Status=active 或 NULL | repo.Service.UpdateCurrentRelease |
| INV-2 | Release.StartSpec 等四字段在 active/archived 后不可改 | Release.BeforeUpdate hook |
| INV-3 | 同 Service 至多一条 Release.Status=active | usecase.ReplaceCurrentRelease(事务) |
| INV-4 | Service.State 转移合法 | Service.CanTransitionTo + BeforeUpdate hook |
| INV-5 | Server.Network 必须有恰好一条 kind=loopback | Server.BeforeSave hook(已有) |
| INV-6 | Application.RunServerID 与 ServerID 至少一边非零 | Application.BeforeSave hook(已有) |
| INV-7 | DeployRun 永不更新,只插入 | repo.DeployRun 不暴露 Update |
| INV-8 | takeover 创建 Service 后必须写一条 DeployRun(Status=success, Source=takeover) | usecase.Takeover 末尾 |

## 5. StartSpec typed builder(关键设计)

```go
// domain/startspec.go
type StartSpec interface {
    Kind() string        // "docker" | "compose" | "native" | "static"
    Marshal() ([]byte, error)
}

type DockerSpec struct {
    Image    string   `json:"image"`
    Cmd      string   `json:"cmd,omitempty"`
    Args     []string `json:"args,omitempty"`
    Ports    []string `json:"ports,omitempty"`
    Volumes  []string `json:"volumes,omitempty"`
    Restart  string   `json:"restart,omitempty"`
}

type ComposeSpec struct {
    FileName       string `json:"file_name"`
    ComposeProfile string `json:"compose_profile,omitempty"`
}

type NativeSpec struct {
    Cmd            string `json:"cmd"`
    WorkdirSubpath string `json:"workdir_subpath,omitempty"`
}

type StaticSpec struct {
    IndexFile string `json:"index_file,omitempty"`
}

func UnmarshalStartSpec(kind, raw string) (StartSpec, error)  // dispatch by kind
```

收益:
- `release_apply.go::buildStartPart` 不再用 `map[string]any`,编译期类型检查
- 新增字段(eg `DockerSpec.HealthCheck`)零分散修改
- adapter 拿到 typed spec,switch 在 builder 一处,不外溢

## 6. 历史字段下线总账(已发生,作记录)

| Phase | 下线字段 | 派生入口 |
|---|---|---|
| P-D | Service.{compose_file,start_cmd,runtime,config_files} | StartSpec / ConfigFileSet |
| P-E | Service.{desired_version,actual_version} | Release.Label |
| P-F | Service.env_vars | EnvVarSet |
| P-G | Service.{last_status,last_run_at} | DeployRun(svcstatus) |
| P-I | Service.image_name | Release.StartSpec.image(svcstatus) |
| **R3**(本次) | **Application.status, Server.status** | **derive/application + derive/server** |

R3 后 model 上**不再有任何"摘要/缓存/历史"字段**,只剩真值。
