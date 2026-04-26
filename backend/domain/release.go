package domain

// Release 是 R1 的最小占位,R7/R8 充实。
//
// StartSpec 在 R8 改为 typed interface;R1 阶段保持 string,
// 仅满足 core/runtime.Adapter.PlanStart 的签名编译需求。
type Release struct {
	ID        uint
	ServiceID uint
	Version   string
	StartSpec string // R8: 改为 StartSpec interface

	// R2 扩(为 adapter docker 路径的 image 兜底所需):
	// ArtifactProvider / ArtifactRef 来自 model.Artifact,在 model→domain 转换时填入。
	ArtifactProvider string
	ArtifactRef      string
}
