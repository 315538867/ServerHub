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
func renderDomainSite(ig IngressCtx) (ConfigFile, error) {
	if ig.FileStem == "" {
		return ConfigFile{}, fmt.Errorf("FileStem 不可为空")
	}
	if ig.Domain == "" {
		return ConfigFile{}, fmt.Errorf("domain 不可为空（domain 模式）")
	}

	routes := sortedRoutes(ig.Routes)
	var sb strings.Builder
	fmt.Fprintf(&sb, "server {\n    listen 80;\n    server_name %s;\n\n", ig.Domain)
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
func writeLocation(sb *strings.Builder, rt RouteCtx, indent string) {
	body := indent + "    "
	fmt.Fprintf(sb, "%slocation %s {\n", indent, rt.Path)
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
