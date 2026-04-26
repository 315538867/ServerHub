package model

import (
	"time"

	"gorm.io/gorm"
)

// Application 是用户视角的"业务单元"，它聚合若干 Service（运行实体），
// 并持有应用级元数据（域名、容器主名等）。一个 Application 可以挂多个
// Service（1:N），通过 Service.ApplicationID 反向关联。
//
// 兼容历史字段（BaseDir/ContainerName/SiteName/ExposeMode/DeployID）暂保留，
// 因 Apps 详情页 dirs/metrics 等接口仍依赖；后续清理拆到 Service 维度。
//
// Nginx-P0 重构：
//   - 新增 RunServerID 用于明确表达「应用跑在哪台机」，与 ServerID 双写过渡。
//     P3 删除 ServerID。BeforeSave 钩子保证两字段永远一致。
//   - Domain 字段在 P1 起由 Ingress.Domain 接管，P3 删除。
//   - ExposeMode 当前仍是 none/path/site，迁移到 Ingress 模型后语义变为
//     managed/none，P1 切换。
type Application struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"not null;uniqueIndex" json:"name"`
	Description string `gorm:"default:''" json:"description"`
	ServerID    uint   `gorm:"not null;index" json:"server_id"`
	// RunServerID 表示应用实际运行的服务器；与 ServerID 在 P0~P2 期间双写，
	// P3 删除 ServerID 后成为唯一字段。新代码请优先读 RunServerID。
	RunServerID uint `gorm:"index" json:"run_server_id"`

	// 应用级聚合
	PrimaryServiceID *uint  `gorm:"index" json:"primary_service_id"`
	SiteName         string `gorm:"default:''" json:"site_name"`
	Domain           string `gorm:"default:''" json:"domain"`
	ContainerName    string `gorm:"default:''" json:"container_name"`
	BaseDir          string `gorm:"default:''" json:"base_dir"`
	ExposeMode       string `gorm:"default:none" json:"expose_mode"`

	// 历史关联，迁移期保留；新代码用 PrimaryServiceID + Service.ApplicationID
	DeployID *uint `gorm:"index" json:"deploy_id"`
	DBConnID *uint `gorm:"index" json:"db_conn_id"`

	// R3 起 Status 列已下线:application 状态由 derive.AppStatus 从下属 Service 的
	// 最近 DeployRun.Status 聚合派生。响应 DTO 由 api 层包一层 appResp 注入 Status。
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeSave 钩子：双向同步 ServerID ↔ RunServerID，保证两字段在过渡期永远一致。
//
// 调用方可以只填一个字段，钩子自动补另一个：
//   - 老代码只填 ServerID → RunServerID 自动跟上
//   - 新代码只填 RunServerID → ServerID 自动跟上
//
// 两个都填且不一致：以 RunServerID 为权威（新字段优先），ServerID 被覆盖。
func (a *Application) BeforeSave(_ *gorm.DB) error {
	switch {
	case a.RunServerID == 0 && a.ServerID != 0:
		a.RunServerID = a.ServerID
	case a.ServerID == 0 && a.RunServerID != 0:
		a.ServerID = a.RunServerID
	case a.RunServerID != 0 && a.ServerID != 0 && a.RunServerID != a.ServerID:
		// 不一致：新字段优先
		a.ServerID = a.RunServerID
	}
	return nil
}
