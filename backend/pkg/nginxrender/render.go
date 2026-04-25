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
	// StreamsConf 聚合所有 tcp/udp 路由的 nginx stream 配置。stream 块只能在
	// nginx.conf 顶层 include，因此独立成文件，由 Reconciler 在 nginx.conf 写入
	// 幂等的 include。
	StreamsConf = "/etc/nginx/streams.conf"
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
//   - protocol=tcp|udp → 全部聚合到 StreamsConf 单文件（顶层 stream 块）
//
// 注意：sites-enabled 下的 symlink、nginx.conf 中的 stream include 都不在本函数
// 输出中，由 Reconciler 单独维护（Differ 把 symlink 当成独立 ChangeKind；nginx.conf
// 的 marker 块由 Reconciler.ensureStreamInclude 幂等处理）。
func Render(ingresses []IngressCtx) ([]ConfigFile, error) {
	var files []ConfigFile
	hasPath := false
	var streamRoutes []RouteCtx

	for _, ig := range ingresses {
		if len(ig.Routes) == 0 {
			continue
		}
		// 先按协议拆分：stream 路由全局聚合，其余按 MatchKind 走原渲染路径
		httpRoutes, sRoutes := partitionStream(ig.Routes)
		streamRoutes = append(streamRoutes, sRoutes...)
		if len(httpRoutes) == 0 {
			continue
		}
		igHTTP := ig
		igHTTP.Routes = httpRoutes

		switch igHTTP.MatchKind {
		case MatchKindDomain:
			f, err := renderDomainSite(igHTTP)
			if err != nil {
				return nil, fmt.Errorf("render ingress edge=%d domain=%q: %w", ig.EdgeServerID, ig.Domain, err)
			}
			files = append(files, f)
		case MatchKindPath:
			f, err := renderPathLocations(igHTTP)
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

	// 任一 stream 路由 → 聚合成单个 streams.conf
	if len(streamRoutes) > 0 {
		sf, err := renderStreams(streamRoutes)
		if err != nil {
			return nil, err
		}
		files = append(files, sf)
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
//
// TLS（P2-D4）：
//   - TLSCertPath 非空即视为启用 HTTPS：渲染额外的 listen 443 ssl 块（grpc 时升级
//     成 ssl http2）；同时把 ssl_certificate / ssl_certificate_key 写在 server_name
//     之后、空行之前
//   - ForceHTTPS=true：把 80 块替换成 return 301 https://$host$request_uri 的跳转
//     server；HTTPS 块照常渲染
//   - ForceHTTPS=false 且 TLS 启用：80 与 443 同时挂同一组路由（明/密双轨）
func renderDomainSite(ig IngressCtx) (ConfigFile, error) {
	if ig.FileStem == "" {
		return ConfigFile{}, fmt.Errorf("FileStem 不可为空")
	}
	if ig.Domain == "" {
		return ConfigFile{}, fmt.Errorf("domain 不可为空（domain 模式）")
	}
	tlsOn := ig.TLSCertPath != ""
	if tlsOn && ig.TLSKeyPath == "" {
		return ConfigFile{}, fmt.Errorf("启用 TLS 时 TLSKeyPath 不可为空")
	}

	routes := sortedRoutes(ig.Routes)
	grpc := anyGRPC(routes)

	var sb strings.Builder
	// 80 段：要么走重定向（force_https + tls），要么挂同样的路由
	if tlsOn && ig.ForceHTTPS {
		fmt.Fprintf(&sb, "server {\n    listen 80;\n    server_name %s;\n    return 301 https://$host$request_uri;\n}\n", ig.Domain)
	} else {
		listen := "listen 80;"
		if grpc {
			listen = "listen 80 http2;"
		}
		writeServerWithRoutes(&sb, listen, ig.Domain, "", "", routes)
	}
	// 443 段：仅 TLS 启用时输出
	if tlsOn {
		listen := "listen 443 ssl;"
		if grpc {
			listen = "listen 443 ssl http2;"
		}
		writeServerWithRoutes(&sb, listen, ig.Domain, ig.TLSCertPath, ig.TLSKeyPath, routes)
	}

	return ConfigFile{
		Path:    fmt.Sprintf("%s/%s-sh.conf", SitesAvailableDir, ig.FileStem),
		Content: sb.String(),
		Mode:    0o644,
	}, nil
}

// writeServerWithRoutes 输出一个 server { listen ...; server_name ...; [ssl_*]; <routes> } 块。
// certPath 非空时在 server_name 之后注入 ssl_certificate / ssl_certificate_key 两行。
// 字节级与旧 applySite 对齐：所有路由前固定一个空行，块尾为 "}\n"。
func writeServerWithRoutes(sb *strings.Builder, listen, name, certPath, keyPath string, routes []RouteCtx) {
	fmt.Fprintf(sb, "server {\n    %s\n    server_name %s;\n", listen, name)
	if certPath != "" {
		fmt.Fprintf(sb, "    ssl_certificate %s;\n", certPath)
		fmt.Fprintf(sb, "    ssl_certificate_key %s;\n", keyPath)
	}
	sb.WriteString("\n")
	for _, rt := range routes {
		writeLocation(sb, rt, "    ")
	}
	sb.WriteString("}\n")
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

// partitionStream 拆 routes 成（http/grpc/ws 走 sites/locations，stream 走 streams.conf）。
// stream 协议：tcp / udp。
func partitionStream(in []RouteCtx) (http, stream []RouteCtx) {
	for _, r := range in {
		if isStreamProto(r.Protocol) {
			stream = append(stream, r)
		} else {
			http = append(http, r)
		}
	}
	return
}

func isStreamProto(p string) bool {
	return p == "tcp" || p == "udp"
}

// renderStreams 把所有 tcp/udp 路由聚合成单个 streams.conf。
//
// nginx stream 块语义：
//   - 必须挂在 nginx.conf 顶层（不能在 http{} 里）
//   - 一台 nginx 实例只能有一个 stream{} 块，因此本函数收集 edge 上所有 stream
//     路由放进同一个 server 列表
//   - listen 端口默认 TCP；udp 路由 listen 后追加 " udp"
//   - proxy_pass 接受 host:port，scheme 必须剥掉
//
// 路由按 (ListenPort asc, Path asc) 稳定排序，使输出可重现。
func renderStreams(routes []RouteCtx) (ConfigFile, error) {
	srv := make([]RouteCtx, 0, len(routes))
	for _, r := range routes {
		if r.ListenPort <= 0 {
			return ConfigFile{}, fmt.Errorf("stream 路由 path=%q protocol=%s 缺 listen_port", r.Path, r.Protocol)
		}
		if r.UpstreamURL == "" {
			return ConfigFile{}, fmt.Errorf("stream 路由 path=%q listen=%d 缺 upstream", r.Path, r.ListenPort)
		}
		srv = append(srv, r)
	}
	sort.SliceStable(srv, func(i, j int) bool {
		if srv[i].ListenPort != srv[j].ListenPort {
			return srv[i].ListenPort < srv[j].ListenPort
		}
		return srv[i].Path < srv[j].Path
	})

	var sb strings.Builder
	sb.WriteString("stream {\n")
	for _, r := range srv {
		listen := fmt.Sprintf("%d", r.ListenPort)
		if r.Protocol == "udp" {
			listen += " udp"
		}
		fmt.Fprintf(&sb, "    server {\n")
		fmt.Fprintf(&sb, "        listen %s;\n", listen)
		fmt.Fprintf(&sb, "        proxy_pass %s;\n", streamUpstream(r.UpstreamURL))
		if r.Extra != "" {
			fmt.Fprintf(&sb, "        %s\n", r.Extra)
		}
		fmt.Fprintf(&sb, "    }\n")
	}
	sb.WriteString("}\n")
	return ConfigFile{Path: StreamsConf, Content: sb.String(), Mode: 0o644}, nil
}

// streamUpstream 把上游 URL 转成 stream proxy_pass 期望的 host:port。
//   - http://h:p  → h:p
//   - https://h:p → h:p（stream 不区分 scheme，走原始 TLS/明文留 nginx 自己判断；
//     P3 接 ssl_preread 时再扩展）
//   - 已是 h:p     → 原样
func streamUpstream(u string) string {
	switch {
	case strings.HasPrefix(u, "http://"):
		return strings.TrimPrefix(u, "http://")
	case strings.HasPrefix(u, "https://"):
		return strings.TrimPrefix(u, "https://")
	default:
		return u
	}
}
