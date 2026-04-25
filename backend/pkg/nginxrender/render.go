package nginxrender

import (
	"fmt"
	"sort"
	"strings"
)

// 远端路径常量。P1 单 nginx 实例硬编码；P3 抽到 NginxProfile。
const (
	SitesAvailableDir = "/etc/nginx/sites-available"
	SitesEnabledDir   = "/etc/nginx/sites-enabled"
	AppLocationsDir   = "/etc/nginx/app-locations"
	HubSiteName       = "serverhub-app-hub"
)

// MatchKind 常量。
const (
	MatchKindDomain = "domain"
	MatchKindPath   = "path"
)

// Render 把一组 IngressCtx 渲染成所有该 edge 上应有的 nginx 配置文件。
// 返回结果按 Path 升序，便于 Differ 稳定比较；空 routes 的 ingress 不出文件。
//
// 路径策略：
//   - MatchKind=domain → SitesAvailableDir/<FileStem>-sh.conf（独占 server block）
//   - MatchKind=path   → AppLocationsDir/<FileStem>.conf（被 hub 站点 include）
//
// 注意：sites-enabled 下的 symlink 不在本函数输出中，由 Reconciler 单独维护
// （Differ 把 symlink 当成独立 ChangeKind）。
func Render(ingresses []IngressCtx) ([]ConfigFile, error) {
	var files []ConfigFile
	hasPath := false

	for _, ig := range ingresses {
		if len(ig.Routes) == 0 {
			continue
		}
		switch ig.MatchKind {
		case MatchKindDomain:
			f, err := renderDomainSite(ig)
			if err != nil {
				return nil, fmt.Errorf("render ingress edge=%d domain=%q: %w", ig.EdgeServerID, ig.Domain, err)
			}
			files = append(files, f)
		case MatchKindPath:
			f, err := renderPathLocations(ig)
			if err != nil {
				return nil, fmt.Errorf("render ingress edge=%d domain=%q: %w", ig.EdgeServerID, ig.Domain, err)
			}
			files = append(files, f)
			hasPath = true
		default:
			return nil, fmt.Errorf("ingress edge=%d domain=%q: 未知 match_kind=%q", ig.EdgeServerID, ig.Domain, ig.MatchKind)
		}
	}

	// 任一 path 模式 ingress 存在 → 共享 hub 站点也要渲染出来
	if hasPath {
		hub, err := RenderHubSite()
		if err != nil {
			return nil, err
		}
		files = append(files, hub)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// RenderHubSite 渲染 path 模式共用的 hub 站点（include 全部 app-locations）。
// 与旧 applyPath 中硬编码的 hubConf 字节对齐。
func RenderHubSite() (ConfigFile, error) {
	content := fmt.Sprintf(`server {
    listen 80;
    server_name _;

    include %s/*.conf;
}`, AppLocationsDir)
	return ConfigFile{
		Path:    SitesAvailableDir + "/" + HubSiteName,
		Content: content,
		Mode:    0o644,
	}, nil
}

// renderDomainSite 输出独占域名的 server block。
// 字节对齐旧 applySite 的格式：8 空格缩进 location 内部，4 空格缩进 location 关键字。
//
// 任一 route.Protocol=="grpc" 时给 listen 加 http2 标志（nginx grpc_pass 强依赖
// HTTP/2 over cleartext）。后续 TLS 接入后再扩展为 listen 443 ssl http2。
func renderDomainSite(ig IngressCtx) (ConfigFile, error) {
	if ig.FileStem == "" {
		return ConfigFile{}, fmt.Errorf("FileStem 不可为空")
	}
	if ig.Domain == "" {
		return ConfigFile{}, fmt.Errorf("domain 不可为空（domain 模式）")
	}

	routes := sortedRoutes(ig.Routes)
	listen := "listen 80;"
	if anyGRPC(routes) {
		listen = "listen 80 http2;"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "server {\n    %s\n    server_name %s;\n\n", listen, ig.Domain)
	for _, rt := range routes {
		writeLocation(&sb, rt, "    ")
	}
	sb.WriteString("}\n")

	return ConfigFile{
		Path:    fmt.Sprintf("%s/%s-sh.conf", SitesAvailableDir, ig.FileStem),
		Content: sb.String(),
		Mode:    0o644,
	}, nil
}

// renderPathLocations 输出 path 模式下的纯 location 列表（无外层 server）。
// 字节对齐旧 applyPath：每个 location 以 "}\n\n" 结尾，文件末尾保留尾随空行。
func renderPathLocations(ig IngressCtx) (ConfigFile, error) {
	if ig.FileStem == "" {
		return ConfigFile{}, fmt.Errorf("FileStem 不可为空")
	}
	routes := sortedRoutes(ig.Routes)

	var sb strings.Builder
	for _, rt := range routes {
		writeLocation(&sb, rt, "")
	}

	return ConfigFile{
		Path:    fmt.Sprintf("%s/%s.conf", AppLocationsDir, ig.FileStem),
		Content: sb.String(),
		Mode:    0o644,
	}, nil
}

// writeLocation 渲染单个 location 块，以 indent 作为外层缩进前缀。
// indent="" 适配 path 模式（独立 location 文件），indent="    " 适配 domain 模式
// （内嵌于 server block）。块尾固定 "}\n\n"，与旧实现一致。
//
// Protocol 分支：
//   - "grpc": 用 grpc_pass + grpc_set_header；UpstreamURL 的 http:// 前缀
//     替换为 grpc://，scheme 缺失时直接补 grpc://
//   - 其它（含空、"http"、"ws"）: 走 proxy_pass HTTP 链路，WebSocket=true 时
//     额外注入 Upgrade/Connection 头
func writeLocation(sb *strings.Builder, rt RouteCtx, indent string) {
	body := indent + "    "
	fmt.Fprintf(sb, "%slocation %s {\n", indent, rt.Path)

	if rt.Protocol == "grpc" {
		fmt.Fprintf(sb, "%sgrpc_pass %s;\n", body, grpcURL(rt.UpstreamURL))
		fmt.Fprintf(sb, "%sgrpc_set_header Host $host;\n", body)
		fmt.Fprintf(sb, "%sgrpc_set_header X-Real-IP $remote_addr;\n", body)
		if rt.Extra != "" {
			fmt.Fprintf(sb, "%s%s\n", body, rt.Extra)
		}
		fmt.Fprintf(sb, "%s}\n\n", indent)
		return
	}

	fmt.Fprintf(sb, "%sproxy_pass %s;\n", body, rt.UpstreamURL)
	fmt.Fprintf(sb, "%sproxy_set_header Host $host;\n", body)
	fmt.Fprintf(sb, "%sproxy_set_header X-Real-IP $remote_addr;\n", body)
	fmt.Fprintf(sb, "%sproxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n", body)
	fmt.Fprintf(sb, "%sproxy_set_header X-Forwarded-Proto $scheme;\n", body)
	if rt.WebSocket {
		fmt.Fprintf(sb, "%sproxy_http_version 1.1;\n", body)
		fmt.Fprintf(sb, "%sproxy_set_header Upgrade $http_upgrade;\n", body)
		fmt.Fprintf(sb, "%sproxy_set_header Connection \"upgrade\";\n", body)
	}
	if rt.Extra != "" {
		fmt.Fprintf(sb, "%s%s\n", body, rt.Extra)
	}
	fmt.Fprintf(sb, "%s}\n\n", indent)
}

// anyGRPC 判断 routes 里是否至少有一条 protocol=grpc。
// 用于决定 server 块的 listen 是否需要 http2 标志。
func anyGRPC(routes []RouteCtx) bool {
	for _, r := range routes {
		if r.Protocol == "grpc" {
			return true
		}
	}
	return false
}

// grpcURL 把上游 URL 转成 nginx grpc_pass 期望的形式：
//   - "http://host:port"  → "grpc://host:port"
//   - "https://host:port" → "grpcs://host:port"
//   - 没有 scheme 的裸 host[:port] → "grpc://host[:port]"
//
// netresolve 永远输出 http://，但留这层保险以兼容用户在 raw 上游里手填的串。
func grpcURL(u string) string {
	switch {
	case strings.HasPrefix(u, "http://"):
		return "grpc://" + strings.TrimPrefix(u, "http://")
	case strings.HasPrefix(u, "https://"):
		return "grpcs://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "grpc://"), strings.HasPrefix(u, "grpcs://"):
		return u
	default:
		return "grpc://" + u
	}
}

// sortedRoutes 复制 routes 并按 (Sort asc, Path asc) 稳定排序，避免外部传入顺序影响输出。
func sortedRoutes(in []RouteCtx) []RouteCtx {
	out := make([]RouteCtx, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].Path < out[j].Path
	})
	return out
}
