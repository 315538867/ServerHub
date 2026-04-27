package ingress

// IngressCandidate 是 Backend.Discover 返回的候选反代 vhost。
//
// 1:N 候选→路由:一个 server{} 块映射成一个 candidate;Routes 通常 ≥1 条。
// AlreadyManaged 由 usecase 层比对 db 后填入,Discover 实现本身置 false。
type IngressCandidate struct {
	ConfigFile     string  `json:"config_file"`
	ServerName     string  `json:"server_name"`
	Listen         string  `json:"listen"`
	Routes         []Route `json:"routes"`
	Fingerprint    string  `json:"fingerprint"`
	AlreadyManaged bool    `json:"already_managed"`
}

// Route 是候选 vhost 内一条 location(或 server 顶层 proxy_pass)。
//
// Extra 是 location 块内除 proxy_pass 之外的 body 行原样保留;接管入库后
// 由渲染器照贴回 location,确保第一次 apply 不丢失原配置语义。
//
// CrossServerID/CrossServerName 仅当 ProxyPass 主机命中**另一台**已注册
// Server 时由 usecase 层填入,纯展示用。同 edge 自身 / unix sock / 域名
// 解析失败均置零。
type Route struct {
	Path            string `json:"path"`
	ProxyPass       string `json:"proxy_pass"`
	WebSocket       bool   `json:"websocket"`
	Extra           string `json:"extra"`
	CrossServerID   uint   `json:"cross_server_id,omitempty"`
	CrossServerName string `json:"cross_server_name,omitempty"`
}

// Fingerprint 算法约定:sha1(path + "|" + serverName) 取前 8 字节 hex。
// 等价 v1 pkg/discovery.ingressProxyFingerprint;实现见 adapters 侧,
// 端口包不持有 crypto 依赖以保持 core 纯净。
