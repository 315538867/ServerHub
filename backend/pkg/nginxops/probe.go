package nginxops

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/serverhub/serverhub/pkg/runner"
)

// ProbeResult 是 `nginx -V 2>&1` 的解析结果。所有字段都是从输出文本里提取的
// 静态信息，调用方决定是否回写到 model.NginxProfile。
//
// 字段缺失（远端 nginx 编译时未带相关参数）一律用空串/空 slice 表示，由 UI
// 决定如何展示（"未配置"或回退默认）。
type ProbeResult struct {
	BinaryPath  string   // which 解析出的 nginx 可执行路径
	Raw         string   // 原始 nginx -V 输出，便于排查
	Version     string   // 1.24.0 / 1.25.3 等
	BuildPrefix string   // --prefix=
	BuildConf   string   // --conf-path=
	Modules     []string // --with-XXX_module 列表（已去 with- 前缀，按字典序）
}

var (
	versionRE = regexp.MustCompile(`(?m)^nginx version: (?:nginx/)?([\w.\-]+)`)
	prefixRE  = regexp.MustCompile(`--prefix=([^\s]+)`)
	confRE    = regexp.MustCompile(`--conf-path=([^\s]+)`)
	moduleRE  = regexp.MustCompile(`--with-([A-Za-z0-9_]+_module)`)
)

// ProbeNginxV 在远端跑 `which nginx` + `nginx -V`，把结果解析成 ProbeResult。
// 返回错误时通常说明 nginx 不在 PATH 里——这种 edge 不能 Apply，调用方应给出
// 明确的诊断而不是继续尝试 reconcile。
func ProbeNginxV(rn runner.Runner) (*ProbeResult, error) {
	whichOut, err := rn.Run("command -v nginx 2>/dev/null || which nginx 2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("which nginx 失败: %w", err)
	}
	binary := strings.TrimSpace(whichOut)
	if binary == "" {
		return nil, fmt.Errorf("远端找不到 nginx 可执行文件")
	}
	// nginx -V 把版本信息写到 stderr，必须 2>&1 合并；TimeoutCmd 的开销在
	// runner 层已处理，这里不再额外包装。
	raw, err := rn.Run("nginx -V 2>&1")
	if err != nil {
		return nil, fmt.Errorf("nginx -V 失败: %w（输出: %s）", err, strings.TrimSpace(raw))
	}
	return ParseNginxV(binary, raw), nil
}

// ParseNginxV 是纯函数，方便单测。binary 由调用方传入，不参与解析。
func ParseNginxV(binary, raw string) *ProbeResult {
	res := &ProbeResult{BinaryPath: binary, Raw: raw}
	if m := versionRE.FindStringSubmatch(raw); len(m) == 2 {
		res.Version = m[1]
	}
	if m := prefixRE.FindStringSubmatch(raw); len(m) == 2 {
		res.BuildPrefix = m[1]
	}
	if m := confRE.FindStringSubmatch(raw); len(m) == 2 {
		res.BuildConf = m[1]
	}
	seen := map[string]struct{}{}
	for _, m := range moduleRE.FindAllStringSubmatch(raw, -1) {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		res.Modules = append(res.Modules, m[1])
	}
	sort.Strings(res.Modules)
	return res
}
