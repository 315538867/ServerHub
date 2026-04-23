package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
)

// Fingerprint 基于 Candidate 的稳定属性算 SHA1 指纹，供发现去重。
// 规则因 Kind 而异：
//
//	docker / container: image + WorkDir + 排序后的 ExtraLabels[binds/ports]
//	compose:            compose 文件绝对路径
//	systemd:            unit 名 + ExecStart
//	nginx/nginx-static: server_name + location_prefix + root (WorkDir)
//
// 返回值固定 40 字符 hex；同服务器内同 fingerprint 视为同一 Service 源。
func Fingerprint(c Candidate) string {
	var key string
	switch c.Kind {
	case KindDocker, "container":
		binds := c.ExtraLabels["binds"]
		ports := c.ExtraLabels["ports"]
		key = strings.Join([]string{
			"docker",
			c.Suggested.ImageName,
			c.Suggested.WorkDir,
			normalizeList(binds),
			normalizeList(ports),
		}, "|")
	case KindCompose:
		key = "compose|" + c.Suggested.ComposeFile
	case KindSystemd:
		key = "systemd|" + c.SourceID + "|" + c.ExtraLabels["exec_start"]
	case KindNginx, "nginx-static":
		key = strings.Join([]string{
			"nginx",
			c.ExtraLabels["server_name"],
			c.ExtraLabels["location_prefix"],
			c.Suggested.WorkDir,
		}, "|")
	default:
		key = c.Kind + "|" + c.SourceID
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// normalizeList 把形如 "a,b,c" 或 "a;b;c" 的字符串拆分、排序、去空后重新拼接，
// 用于构造指纹输入，避免 docker inspect 返回顺序不稳定导致指纹漂移。
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
