// Package nginxpresets 把"常见 nginx location 内合法指令组合"参数化,
// 让前端不必让用户手抄 add_header / limit_rate / proxy_cache 这些指令。
//
// 设计边界:
//   - 输出文本仅供 IngressRoute.Extra 字段使用——也就是会被原样插入
//     `location { ... }` 内部。因此本包只生成"location 上下文合法"的指令,
//     依赖 http{} 顶层 zone 的预设(limit_req_zone、proxy_cache_path)在
//     注释里提示用户由运维侧自行准备,不在这里输出。
//   - 所有参数都先经 normalize/validate,再格式化输出;非法参数返回 error
//     而不是吞掉,避免渲染出"半成品"的 nginx 片段。
//   - 输出本身需要能通过 safeshell.NginxBlock,所以不会包含 `{}#\` 等字符。
package nginxpresets

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Kind 标识预设类型,前端通过 kind 字段路由到具体 builder。
type Kind string

const (
	KindRateLimit Kind = "ratelimit"
	KindCache     Kind = "cache"
	KindSecurity  Kind = "security"
)

// ── ratelimit ────────────────────────────────────────────────────────────────

// RateLimitOpts 对应 location 内可独立使用的限流/限速指令组合。
//
// 故意不暴露 limit_req_zone(它必须挂在 http{} 顶层),只输出
// 客户端体大小 + 单连接限速,这两个不需要 zone 配合就能生效。
type RateLimitOpts struct {
	// MaxBodyKB: client_max_body_size 的 KiB 数。0 表示不下发该指令。
	MaxBodyKB int `json:"max_body_kb"`
	// RateKBs: limit_rate 的 KiB/s 限速。0 表示不下发。
	RateKBs int `json:"rate_kbs"`
	// RateAfterKB: limit_rate_after 的 KiB 阈值(超过后才开始限速)。
	// 仅在 RateKBs > 0 时有意义。
	RateAfterKB int `json:"rate_after_kb"`
}

func BuildRateLimit(opts RateLimitOpts) (string, error) {
	if opts.MaxBodyKB < 0 || opts.MaxBodyKB > 1024*1024 {
		return "", errors.New("max_body_kb 必须在 [0, 1048576] 区间")
	}
	if opts.RateKBs < 0 || opts.RateKBs > 1024*1024 {
		return "", errors.New("rate_kbs 必须在 [0, 1048576] 区间")
	}
	if opts.RateAfterKB < 0 || opts.RateAfterKB > 1024*1024 {
		return "", errors.New("rate_after_kb 必须在 [0, 1048576] 区间")
	}
	if opts.MaxBodyKB == 0 && opts.RateKBs == 0 {
		return "", errors.New("max_body_kb 与 rate_kbs 至少传一项 (>0)")
	}
	var lines []string
	if opts.MaxBodyKB > 0 {
		lines = append(lines, fmt.Sprintf("client_max_body_size %dk;", opts.MaxBodyKB))
	}
	if opts.RateKBs > 0 {
		lines = append(lines, fmt.Sprintf("limit_rate %dk;", opts.RateKBs))
		if opts.RateAfterKB > 0 {
			lines = append(lines, fmt.Sprintf("limit_rate_after %dk;", opts.RateAfterKB))
		}
	}
	return strings.Join(lines, "\n"), nil
}

// ── cache ────────────────────────────────────────────────────────────────────

// CacheOpts 输出 proxy_cache 系列指令。
//
// ZoneName 必须由用户在 http{} 顶层提前 proxy_cache_path 声明(预设里
// 会用注释提示这一点);本 builder 仅生成 location 内的引用语句。
type CacheOpts struct {
	// ZoneName: 已存在的 proxy_cache_path 名。仅允许 [A-Za-z0-9_]+。
	ZoneName string `json:"zone_name"`
	// Valid200Mins: 200 响应的缓存时长(分钟)。0 表示不下发。
	Valid200Mins int `json:"valid_200_mins"`
	// Valid404Mins: 404 响应的负缓存时长(分钟)。0 表示不下发。
	Valid404Mins int `json:"valid_404_mins"`
	// UseStale: 后端不可用时是否仍返回过期缓存。
	UseStale bool `json:"use_stale"`
}

var zoneNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func BuildCache(opts CacheOpts) (string, error) {
	if !zoneNameRe.MatchString(opts.ZoneName) {
		return "", errors.New("zone_name 必须是字母/数字/下划线")
	}
	if len(opts.ZoneName) > 64 {
		return "", errors.New("zone_name 过长")
	}
	if opts.Valid200Mins < 0 || opts.Valid200Mins > 60*24*30 {
		return "", errors.New("valid_200_mins 必须在 [0, 43200] 区间")
	}
	if opts.Valid404Mins < 0 || opts.Valid404Mins > 60*24*30 {
		return "", errors.New("valid_404_mins 必须在 [0, 43200] 区间")
	}
	if opts.Valid200Mins == 0 && opts.Valid404Mins == 0 && !opts.UseStale {
		return "", errors.New("至少传一项缓存策略 (valid_200_mins / valid_404_mins / use_stale)")
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("proxy_cache %s;", opts.ZoneName))
	if opts.Valid200Mins > 0 {
		lines = append(lines, fmt.Sprintf("proxy_cache_valid 200 %dm;", opts.Valid200Mins))
	}
	if opts.Valid404Mins > 0 {
		lines = append(lines, fmt.Sprintf("proxy_cache_valid 404 %dm;", opts.Valid404Mins))
	}
	if opts.UseStale {
		lines = append(lines, "proxy_cache_use_stale error timeout updating http_500 http_502 http_503 http_504;")
	}
	return strings.Join(lines, "\n"), nil
}

// ── security ─────────────────────────────────────────────────────────────────

// SecurityOpts 用一组开关控制 add_header 输出。
//
// 之所以不让用户手填值,是因为这些 header 的安全语义只在固定策略下成立——
// X-Frame-Options 不该让用户改成 "ALLOWALL",HSTS max-age 也只允许预定义档位。
type SecurityOpts struct {
	FrameDeny       bool `json:"frame_deny"`
	NoSniff         bool `json:"no_sniff"`
	ReferrerStrict  bool `json:"referrer_strict"`
	HSTSMaxAgeDays  int  `json:"hsts_max_age_days"`  // 0=不下发,允许 30/90/180/365
	HSTSIncludeSub  bool `json:"hsts_include_sub"`   // 仅 HSTS 启用时生效
	XSSReflected    bool `json:"xss_reflected"`      // X-XSS-Protection (历史兼容)
}

var allowedHSTSDays = map[int]bool{30: true, 90: true, 180: true, 365: true}

func BuildSecurity(opts SecurityOpts) (string, error) {
	if opts.HSTSMaxAgeDays != 0 && !allowedHSTSDays[opts.HSTSMaxAgeDays] {
		return "", errors.New("hsts_max_age_days 仅允许 0/30/90/180/365")
	}
	var lines []string
	if opts.FrameDeny {
		lines = append(lines, `add_header X-Frame-Options DENY always;`)
	}
	if opts.NoSniff {
		lines = append(lines, `add_header X-Content-Type-Options nosniff always;`)
	}
	if opts.ReferrerStrict {
		lines = append(lines, `add_header Referrer-Policy strict-origin-when-cross-origin always;`)
	}
	if opts.HSTSMaxAgeDays > 0 {
		val := fmt.Sprintf("max-age=%d", opts.HSTSMaxAgeDays*86400)
		if opts.HSTSIncludeSub {
			val += "; includeSubDomains"
		}
		// 注:HSTS 只在 HTTPS 链路上才有意义,但 nginx 会让 HTTP 响应也带,
		// 浏览器会忽略——前端表单上需要显式提示用户"仅 HTTPS 入口启用"。
		lines = append(lines, fmt.Sprintf(`add_header Strict-Transport-Security "%s" always;`, val))
	}
	if opts.XSSReflected {
		lines = append(lines, `add_header X-XSS-Protection "1; mode=block" always;`)
	}
	if len(lines) == 0 {
		return "", errors.New("至少启用一项 security 头")
	}
	return strings.Join(lines, "\n"), nil
}
