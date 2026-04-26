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
}
