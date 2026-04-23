package model

import "time"

// Application 是用户视角的"业务单元"，它聚合若干 Service（运行实体），
// 并持有应用级元数据（域名、容器主名等）。一个 Application 可以挂多个
// Service（1:N），通过 Service.ApplicationID 反向关联。
//
// 兼容历史字段（BaseDir/ContainerName/SiteName/ExposeMode/DeployID）暂保留，
// 因 Apps 详情页 dirs/metrics 等接口仍依赖；后续清理拆到 Service 维度。
type Application struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"not null;uniqueIndex" json:"name"`
	Description string `gorm:"default:''" json:"description"`
	ServerID    uint   `gorm:"not null;index" json:"server_id"`

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

	Status    string    `gorm:"default:unknown" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
