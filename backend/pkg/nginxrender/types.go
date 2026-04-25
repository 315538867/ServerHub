// Package nginxrender 是纯函数 Renderer：把 desired state（一组 IngressCtx）
// 渲染成 []ConfigFile，由上层 Reconciler 写入远端文件系统。
//
// 设计原则：
//   - 纯函数：不依赖 DB / runner / 网络 IO，便于单测
//   - 输入 RouteCtx.UpstreamURL 已由 netresolve 算好（Renderer 不解析跨机网络）
//   - 输出按 Path 升序，便于 Differ 稳定比较
//
// 与旧 applyPath/applySite 的字节对齐：本包刻意保留旧版的缩进、换行、
// proxy_set_header 顺序，确保升级到 Reconciler 后首次 apply 不会出现无意义
// 的全量重写（最多空白/注释级差异）。
package nginxrender

import "os"

// ConfigFile 是 Renderer 的输出单元，对应远端 /etc/nginx/ 下一个具体文件。
// Reconciler 据此与远端实际状态做 Diff，并最终落盘。
type ConfigFile struct {
	Path    string      // 远端绝对路径，如 /etc/nginx/sites-available/foo-sh.conf
	Content string      // 完整文件内容
	Mode    os.FileMode // 文件权限位（一般 0644）
}

// RouteCtx 是渲染单条 nginx location 块所需的全部上下文。
// UpstreamURL 必须是已解析好的最终 URL（http://host:port），Renderer 不再
// 二次处理；这是为了让 Renderer 保持纯函数。
type RouteCtx struct {
	Sort        int    // 排序键（asc）
	Path        string // location 路径，如 /api 或 /
	Protocol    string // P1 仅 http；ws/grpc/tcp/udp 留 P2/P3
	UpstreamURL string // 已由 Resolver 算出，例如 http://10.0.0.5:8080
	WebSocket   bool   // 勾选后注入 Upgrade/Connection 头
	Extra       string // 用户自定义 nginx 指令（已过 safeshell.NginxValue 校验）
}

// IngressCtx 是渲染一个入口（一组 location 或一个独占 server block）的上下文。
//
// FileStem 是文件名干（不含扩展名）。一台 edge 上同 MatchKind 的 ingress 必须
// 各自取唯一 FileStem（建议：domain 模式用 sanitize(domain)，path 模式用
// sanitize(domain)+"-"+ingressID 等策略，由调用方保证）。
type IngressCtx struct {
	EdgeServerID uint
	FileStem     string     // 文件名干（不含 .conf）
	MatchKind    string     // domain | path
	Domain       string     // server_name 用；path 模式下也用于聚合到同一个 hub
	Routes       []RouteCtx // 至少一条；空 routes 不会渲染出文件
}
