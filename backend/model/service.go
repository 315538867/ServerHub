package model

import (
	"time"

	"github.com/serverhub/serverhub/domain"
	"gorm.io/gorm"
)

// Service represents a managed runtime entity (one start/stop unit) on a
// Server. Replaces the former Deploy model; on-disk table name kept as
// "services" via TableName(). Existing "deploys" table is renamed by the
// AutoMigrate hook in database/db.go.
//
// A Service may belong to an Application (ApplicationID set) or float
// independently (ApplicationID nil) — the latter typically right after
// import-only discovery before takeover.
type Service struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null" json:"name"`
	ServerID uint   `gorm:"not null" json:"server_id"`
	// Type 是启动命令模板调度键 —— usecase/deploy.go::buildStartPart
	// 按 Type switch 选择 docker run / docker compose / native shell / static 四套
	// 模板组装最终 ExecStart。Release.StartSpec 表达"这次部署用什么命令"，
	// Type 表达"这个 Service 属于哪类 runtime"，二者职责不重叠。
	//
	// 取值见下方 ServiceType* 常量。GORM default 字符串 'docker-compose' 是
	// 历史新装库的隐式默认,运行路径(takeover/importer/discovery)均显式指定 Type,
	// 不会真的落到这个默认值上;改它没有迁移收益,故 P-J 只动代码字面量、不动 tag。
	Type string `gorm:"default:docker-compose" json:"type"`

	// Application binding (nullable: floating services allowed)
	ApplicationID *uint `gorm:"index" json:"application_id"`

	// Execution config
	//
	// WorkDir 是真值字段:
	//   - release_apply.go::buildReleaseCmd 在启动每个 Release 前 mkdir -p && cd
	//     这里(为空时退化为 /tmp/serverhub-svc-<id>);所以改 WorkDir 立刻影响下一次
	//     reconcile 的实际工作目录,不必新建 Release。
	//   - discovery.Fingerprint(docker / nginx kind)把它纳入指纹算子,改 WorkDir 会让
	//     下次扫描视为"新候选" → 用户被迫再接管一次。生产中不要轻易改。
	//   - 前端展示用,Service 列表/详情都直读这个值。
	//
	// ImageName 在 P-I 起已下线 —— 字段从 schema 移除,真值由 Service.CurrentReleaseID
	// 指向的 Release.StartSpec.image 派生(见 pkg/svcstatus.LatestByService)。这样
	// 跟 buildStartPart 实际启动用的镜像对齐,而不是 takeover 一次性快照下来后
	// 永不更新的死值。Discovery 指纹仍读 Candidate.Suggested.ImageName(那是扫描
	// 当下从 docker inspect 拿的活值,与 Service 行无关)。
	WorkDir string `gorm:"default:''" json:"work_dir"`

	// ExposedPort 是 Service 对外提供的主端口（供 Nginx upstream 使用）。
	// 0 表示未暴露或纯静态服务。discovery 阶段会尽量从 docker ports / compose
	// 端口映射 / systemd env 中推断填入；用户也可以在 UI 里手工修改。
	ExposedPort int `gorm:"default:0" json:"exposed_port"`

	// Auth & secrets
	WebhookSecret string `gorm:"default:''" json:"-"`

	// Release 新模型指针（Phase M1 引入）。指向 releases.id；为 nil 表示
	// Service 还没有创建过 Release（空壳）。版本语义从 P-E 起完全由 Release.Label
	// 表达,旧 DesiredVersion/ActualVersion 已下线。
	CurrentReleaseID *uint `gorm:"index" json:"current_release_id"`
	// 部署失败时是否自动回滚到上一条 Status=active 的 Release（默认关闭）。
	AutoRollbackOnFail bool `gorm:"default:false" json:"auto_rollback_on_fail"`

	// Reconcile loop
	//
	// SyncStatus 取值 ''(初始,从未参与 reconcile) | 'synced' | 'syncing' | 'error'。
	// 历史曾经存在 'drifted' 枚举,对应 P-D 之前 DesiredVersion ↔ ActualVersion
	// 漂移检测的语义,M3 起 reconciler 改为"无条件重放 CurrentReleaseID",漂移
	// 概念整体退役,该值已不再写入。
	//
	// 写入侧分两类:
	//   - reconciler(pkg/scheduler/reconciler.go):syncing → synced/error,
	//     "syncing" 同时充当原子守卫(Where sync_status != 'syncing' Update),
	//     防止同一 Service 被并发 schedule 触发两次 ApplyRelease。
	//   - 接管入口(pkg/takeover/* + pkg/discovery/importer.go):直写 'synced'
	//     表达"已对齐"初值,不参与 reconciler 的状态机。
	AutoSync     bool   `gorm:"default:false" json:"auto_sync"`
	SyncInterval int    `gorm:"default:60" json:"sync_interval"`
	SyncStatus   string `gorm:"default:''" json:"sync_status"`

	// Status: 历史 LastStatus/LastRunAt 在 P-G 起从 deploy_runs 派生 ——
	// 上层只看"最近一条 DeployRun.Status/StartedAt"。Service 不再持有摘要列。

	// Discovery source. SourceFingerprint is computed by discovery.Fingerprint
	// from kind-specific stable inputs and used to dedup candidates against
	// already-managed services.
	SourceKind        string `gorm:"default:'';index:idx_svc_source,priority:2" json:"source_kind"`
	SourceID          string `gorm:"default:'';index:idx_svc_source,priority:3" json:"source_id"`
	SourceFingerprint string `gorm:"default:'';size:64;index" json:"source_fingerprint"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Service) TableName() string { return "services" }

// BeforeUpdate 钩子：校验 SyncStatus 状态迁移合法性（INV-1）。
// GORM BeforeUpdate 不提供旧值，此处查 DB 取旧状态后调用 domain.CanTransitionTo。
func (s *Service) BeforeUpdate(tx *gorm.DB) error {
	if !tx.Statement.Changed("SyncStatus") {
		return nil
	}
	var old Service
	if err := tx.Session(&gorm.Session{}).Where("id = ?", s.ID).First(&old).Error; err != nil {
		return err
	}
	return domain.CanTransitionTo(old.SyncStatus, s.SyncStatus)
}

// ValidateSyncTransition 供 repo 层在更新 SyncStatus 前显式调用（避免依赖 GORM hook）。
func (s *Service) ValidateSyncTransition(newStatus string) error {
	return domain.CanTransitionTo(s.SyncStatus, newStatus)
}


// ServiceType* 是 Service.Type 的合法取值集合。任何写 Service 行的代码路径
// (4 个 takeover、importer、4 个 discovery suggester)以及 release_apply 里
// buildStartPart 的 switch 分支都应使用这些常量,而不是裸字符串。
//
// 历史 migration(migration/m2_deploy_to_release.go::buildStartSpec)故意保留
// 字面量 —— 那里的字符串是 schema 契约的一部分,不能跟着代码常量演进。
const (
	ServiceTypeDocker        = "docker"
	ServiceTypeDockerCompose = "docker-compose"
	ServiceTypeNative        = "native"
	ServiceTypeStatic        = "static"
)
