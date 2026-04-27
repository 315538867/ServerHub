package docker

import (
	"sort"
	"strings"
)

// dockerEnvSkip — env vars docker / OCI runtimes inject by default. Filtering
// 它们让导入的 deploy 干净:用户只看到 image 或 `docker run -e` 实际注入的。
var dockerEnvSkip = map[string]bool{
	"PATH": true, "HOSTNAME": true, "HOME": true, "TERM": true,
	"PWD": true, "SHLVL": true, "LANG": true,
}

// secretKeyHints — substring 大小写不敏感命中,UI 默认掩码。
// 与 v1 pkg/discovery/env.go 保持完全一致以维持行为兼容。
var secretKeyHints = []string{
	"password", "passwd", "secret", "token", "apikey", "api_key",
	"private", "credential", "jdbc", "dsn", "auth", "session",
	"jwt", "encrypt", "salt", "cookie",
}

// keyLooksSecret 在 env key 暗示敏感时返回 true。保守策略:
// 漏判可接受,误判(过度掩码)也 OK——用户能反向取消。
func keyLooksSecret(k string) bool {
	lower := strings.ToLower(k)
	if strings.HasSuffix(lower, "_key") || (strings.HasSuffix(lower, "key") && len(lower) > 3) {
		return true
	}
	for _, h := range secretKeyHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// parseLabels 拆 docker ps 的 "k=v,k=v" 标签串。
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if eq := strings.IndexByte(p, '='); eq > 0 {
			m[p[:eq]] = p[eq+1:]
		}
	}
	return m
}

// parseKVPairs 拆 KEY=VAL 列表为 (k, v, secret) 三元组。
type envKV struct {
	Key    string
	Value  string
	Secret bool
}

func parseKVPairs(pairs []string) []envKV {
	out := make([]envKV, 0, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		v = unquote(v)
		if k == "" {
			continue
		}
		out = append(out, envKV{Key: k, Value: v, Secret: keyLooksSecret(k)})
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// normalizeList 把形如 "a,b,c" 的串拆分排序去空再拼,稳定 fingerprint 输入,
// 避免 docker inspect 返回顺序不稳定导致指纹漂移。与 v1 算法字节一致。
func normalizeList(s string) string {
	if s == "" {
		return ""
	}
	sep := ","
	if strings.Contains(s, ";") && !strings.Contains(s, ",") {
		sep = ";"
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
