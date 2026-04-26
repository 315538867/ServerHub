package domain

// ServiceType 标识服务的运行时种类,与 RuntimeAdapter.Kind() 一一对应。
type ServiceType string

const (
	ServiceTypeDocker  ServiceType = "docker"
	ServiceTypeCompose ServiceType = "compose"
	ServiceTypeNative  ServiceType = "native"
	ServiceTypeStatic  ServiceType = "static"
)

// Service 是 R1 的最小占位,R7 充实。
//
// 字段仅满足 core/runtime 与 core/source 端口签名;
// 业务代码请勿直接使用本结构,改走 model→domain 转换层(R7)。
type Service struct {
	ID       uint
	Name     string
	Type     ServiceType
	ServerID uint

	// R2 扩(为 adapter BuildStartCmd / PlanStart 纯函数化所需):
	WorkDir            string // 远端工作目录(空则退化 /tmp/serverhub-svc-<id>)
	AutoRollbackOnFail bool   // 部署失败是否自动回滚到上一条 active Release
	CurrentReleaseID   *uint  // 当前 active Release(回滚选目标用)
}
