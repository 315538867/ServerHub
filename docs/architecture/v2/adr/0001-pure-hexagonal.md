# ADR 0001: Pure Hexagonal Architecture 重构

| 属性 | 值 |
|---|---|
| **编号** | 0001 |
| **日期** | 2026-04-29 |
| **作者** | chen.haowei |
| **状态** | accepted |
| **影响范围** | backend/ 全量 |

## 动机

ServerHub v1 架构存在 5 类技术债：

1. **真值/派生混杂** — model 层持有 `last_status`/`image_name`/`desired_version` 等派生字段，真值源不唯一
2. **写入扇出失控** — Service 更新触发 5 处写入，`SyncStatus` 更新触发 3 处写入
3. **adapter 缺位** — runtime/source/ingress/notify 全部硬编码 switch 分支，无法插件化扩展
4. **handler 越权** — API handler 直接操作 GORM，同时承担业务编排和状态机
5. **domain 不存在** — model 兼作业务方法宿主，领域不变量散落各处

## 决策

采用 **Pure Hexagonal + Plugin Registry** 架构，通过 8 个 Phase (R0-R8) 渐进重构：

| Phase | 名称 | 核心变更 |
|---|---|---|
| R0 | 基线冻结 | tag v1-final |
| R1 | core/ 接口建立 | 5 port + 4 registry |
| R2 | adapters/runtime 迁出 | 4 runtime 全插件化 |
| R3 | derive/ 真值派生 | ServerProbe 时序表，摘要字段清零 |
| R4 | adapters/source 迁出 | 4×4 发现接管 |
| R5 | adapters/ingress 迁出 | nginx 字节级一致 |
| R6 | usecase + repo 收口 | api/ 0 处 GORM 调用 |
| R7 | domain/ 提纯 | domain 仅 import stdlib，355 test |
| R8 | StartSpec typed + GA | StartSpec string→typed interface |

### 关键架构原则

- **domain/** — 纯领域实体，仅 import stdlib。定义 port interface，不依赖 model/adapter
- **model/** — GORM-tagged DB 实体，通过 converter.go 双向转换 domain↔model
- **usecase/** — 业务编排，仅依赖 domain port + repo，不直接操作 DB
- **adapters/** — 实现 port interface，含 runtime/docker|compose|native|static、source/http|script|git|upload
- **api/** — HTTP handler，仅做参数绑定/序列化，编排委托 usecase

### StartSpec 类型化 (R8)

`Release.StartSpec` 从 `string` (JSON) 改为 typed interface：

- `DockerSpec{Image, Cmd}` — docker 单容器
- `ComposeSpec{FileName}` — docker-compose
- `NativeSpec{Cmd}` — 裸进程
- `StaticSpec{}` — 纯静态资源

持久化保持 JSON string，model/converter 透明转换。

## 替代方案

**方案 A: 渐进修补** — 不动架构，逐处修 bug。被拒原因：扇出问题无法根治，每修一处引入新问题。

**方案 B: DDD Aggregate** — 完整 DDD 战术模式。被拒原因：过度设计，团队规模和项目复杂度不匹配。

## 后果

### 正向

- 添加新 runtime（如 podman）仅需实现 `RuntimeAdapter` port + 注册
- handler 精简为参数绑定，业务逻辑集中在 usecase
- domain 纯 stdlib 可单独测试，不依赖 DB
- 每个 adapter 可独立测试和替换

### 负向

- 文件数增加（converter.go 作为 model↔domain 转换层）
- 新人需理解 Hexagonal 分层（domain/core/usecase/adapter/api）
- R5 nginx 改造风险高，需 golden fixture 字节级对比

### 缓解

- converter 模式统一，格式机械可自动生成
- v2 文档 6 篇覆盖全架构（00-06）
- R5 staging 灰度 + e2e 验证
- 准出条件每 Phase 9 项 checklist

## 参考

- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [docs/architecture/v2/00-overview.md](../00-overview.md)
- [.zcf/plan/current/架构重构-Hexagonal.md](../../../.zcf/plan/current/架构重构-Hexagonal.md)
