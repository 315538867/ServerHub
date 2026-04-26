package domain

// IngressRoute 占位,R5/R7 充实。
//
// 当前字段仅满足 core/ingress.Backend.Render 的签名;
// 真实字段集(host/path/upstream/tls/...)在 R5 ingress 适配器迁出时锁定。
type IngressRoute struct {
	ID         uint
	Host       string
	Path       string
	UpstreamID uint
}
