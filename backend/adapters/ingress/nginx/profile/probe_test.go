package profile

import (
	"strings"
	"testing"
)

const sampleNginxV = `nginx version: nginx/1.24.0
built by gcc 12.2.0 (Debian 12.2.0-14)
built with OpenSSL 3.0.11 19 Sep 2023
TLS SNI support enabled
configure arguments: --prefix=/etc/nginx --sbin-path=/usr/sbin/nginx --conf-path=/etc/nginx/nginx.conf --error-log-path=/var/log/nginx/error.log --http-log-path=/var/log/nginx/access.log --pid-path=/run/nginx.pid --modules-path=/usr/lib/nginx/modules --user=www-data --group=www-data --with-http_ssl_module --with-http_v2_module --with-http_realip_module --with-http_stub_status_module --with-stream --with-stream_ssl_module --with-http_gzip_static_module --with-http_ssl_module
`

func TestParseNginxV_Standard(t *testing.T) {
	r := ParseNginxV("/usr/sbin/nginx", sampleNginxV)
	if r.BinaryPath != "/usr/sbin/nginx" {
		t.Errorf("binary path: %s", r.BinaryPath)
	}
	if r.Version != "1.24.0" {
		t.Errorf("version: %s", r.Version)
	}
	if r.BuildPrefix != "/etc/nginx" {
		t.Errorf("prefix: %s", r.BuildPrefix)
	}
	if r.BuildConf != "/etc/nginx/nginx.conf" {
		t.Errorf("conf: %s", r.BuildConf)
	}
	// 期望 ssl/v2/realip/stub_status/stream_ssl/gzip_static 这几个常见 module 出现，且去重
	want := []string{
		"http_gzip_static_module", "http_realip_module", "http_ssl_module",
		"http_stub_status_module", "http_v2_module", "stream_ssl_module",
	}
	for _, w := range want {
		if !contains(r.Modules, w) {
			t.Errorf("module %q 应被识别，实得 %v", w, r.Modules)
		}
	}
	// 字典序
	for i := 1; i < len(r.Modules); i++ {
		if r.Modules[i-1] > r.Modules[i] {
			t.Errorf("modules 未按字典序排序：%v", r.Modules)
			break
		}
	}
	// http_ssl_module 在配置里出现两次，应只算一次
	count := 0
	for _, m := range r.Modules {
		if m == "http_ssl_module" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("应去重，http_ssl_module 出现 %d 次", count)
	}
}

func TestParseNginxV_MinimalOutput(t *testing.T) {
	raw := "nginx version: nginx/1.18.0\nconfigure arguments: --with-http_ssl_module\n"
	r := ParseNginxV("/usr/local/bin/nginx", raw)
	if r.Version != "1.18.0" {
		t.Errorf("version: %s", r.Version)
	}
	if r.BuildPrefix != "" || r.BuildConf != "" {
		t.Errorf("缺少 prefix/conf 时应为空，实得 prefix=%q conf=%q", r.BuildPrefix, r.BuildConf)
	}
	if !contains(r.Modules, "http_ssl_module") {
		t.Errorf("应识别 http_ssl_module")
	}
}

func TestParseNginxV_GarbageOutput(t *testing.T) {
	r := ParseNginxV("/x", "completely unrelated output")
	if r.Version != "" || len(r.Modules) != 0 {
		t.Errorf("乱码不应解析出字段：%+v", r)
	}
	if !strings.Contains(r.Raw, "completely unrelated") {
		t.Errorf("Raw 应保留原文")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
