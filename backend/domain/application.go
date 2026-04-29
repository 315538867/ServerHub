package domain

import "time"

// Application 是用户视角的业务单元,聚合若干 Service。
// Status 已在 R3 下线,由 derive.AppStatus 从下属 Service 派生。
type Application struct {
	ID               uint      `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	ServerID         uint      `json:"server_id"`
	RunServerID      uint      `json:"run_server_id"`
	PrimaryServiceID *uint     `json:"primary_service_id"`
	SiteName         string    `json:"site_name"`
	Domain           string    `json:"domain"`
	ContainerName    string    `json:"container_name"`
	BaseDir          string    `json:"base_dir"`
	ExposeMode       string    `json:"expose_mode"`
	DeployID         *uint     `json:"deploy_id"`
	DBConnID         *uint     `json:"db_conn_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
