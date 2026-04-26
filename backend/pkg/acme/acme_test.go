package acme

import (
	"errors"
	"strings"
	"testing"

	"github.com/serverhub/serverhub/pkg/runner"
)

// fakeRunner 让测试能注入 cat / openssl 的输出，不需要真 SSH。
type fakeRunner struct {
	out map[string]string
	err map[string]error
}

func (f *fakeRunner) Run(cmd string) (string, error) {
	if e, ok := f.err[cmd]; ok {
		return "", e
	}
	if v, ok := f.out[cmd]; ok {
		return v, nil
	}
	return "", errors.New("unexpected: " + cmd)
}
func (f *fakeRunner) NewSession() (runner.Session, error) { return nil, errors.New("nope") }
func (f *fakeRunner) IsLocal() bool                       { return false }
func (f *fakeRunner) Capability() string                  { return "full" }
func (f *fakeRunner) Close() error                        { return nil }

func TestIssueCmd(t *testing.T) {
	c, err := IssueCmd(IssueOpts{Domain: "a.example.com", Email: "x@y.z"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"certbot certonly --webroot",
		"-w '/var/www/html'",
		"-d 'a.example.com'",
		"--email 'x@y.z'",
		"--agree-tos --non-interactive",
		"2>&1",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("missing %q in %q", want, c)
		}
	}

	// staging 应该出现 --staging
	c2, _ := IssueCmd(IssueOpts{Domain: "x.test", Webroot: "/srv/www", Email: "a@b.c", Staging: true})
	if !strings.Contains(c2, "-w '/srv/www'") || !strings.Contains(c2, "--staging") {
		t.Errorf("staging cmd 不正确: %q", c2)
	}
}

func TestIssueCmd_RejectsInjection(t *testing.T) {
	bad := []string{"a;rm -rf /", "a$(whoami).com", "a com", "", strings.Repeat("a", 254)}
	for _, d := range bad {
		if _, err := IssueCmd(IssueOpts{Domain: d}); err == nil {
			t.Errorf("expected reject for %q", d)
		}
	}

	good := []string{"a.example.com", "*.example.com", "x-y.test", "1.2.3.4.example"}
	for _, d := range good {
		if _, err := IssueCmd(IssueOpts{Domain: d}); err != nil {
			t.Errorf("expected accept for %q, got %v", d, err)
		}
	}
}

func TestRenewCmd(t *testing.T) {
	c, err := RenewCmd("a.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c, "certbot renew --cert-name 'a.example.com'") {
		t.Errorf("renew cmd 不对: %q", c)
	}
	if _, err := RenewCmd("bad;rm"); err == nil {
		t.Error("应拒绝注入")
	}
}

func TestReadPEM_Success(t *testing.T) {
	domain := "a.example.com"
	cert := "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"
	key := "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----"
	rn := &fakeRunner{out: map[string]string{
		"cat '/etc/letsencrypt/live/a.example.com/fullchain.pem'": cert,
		"cat '/etc/letsencrypt/live/a.example.com/privkey.pem'":   key,
	}}
	pem, err := ReadPEM(rn, domain)
	if err != nil {
		t.Fatal(err)
	}
	if pem.Cert != cert || pem.Key != key {
		t.Errorf("PEM 内容回读不匹配")
	}
}

func TestReadPEM_RejectsBogusContent(t *testing.T) {
	rn := &fakeRunner{out: map[string]string{
		"cat '/etc/letsencrypt/live/a.example.com/fullchain.pem'": "not a cert",
		"cat '/etc/letsencrypt/live/a.example.com/privkey.pem'":   "----BEGIN PRIVATE KEY----",
	}}
	if _, err := ReadPEM(rn, "a.example.com"); err == nil {
		t.Error("应识别非证书内容")
	}
}

func TestReadPEM_RejectsBadDomain(t *testing.T) {
	rn := &fakeRunner{out: map[string]string{}}
	if _, err := ReadPEM(rn, "a;b"); err == nil {
		t.Error("非法 domain 应被拒")
	}
}

func TestParseExpiry(t *testing.T) {
	rn := &fakeRunner{out: map[string]string{
		"openssl x509 -enddate -noout -in '/x/y' 2>/dev/null": "notAfter=Jun  1 12:00:00 2026 GMT\n",
	}}
	t1, err := ParseExpiry(rn, "/x/y")
	if err != nil {
		t.Fatal(err)
	}
	if t1.Year() != 2026 || t1.Month() != 6 || t1.Day() != 1 {
		t.Errorf("解析时间错误: %v", t1)
	}

	// 单空格变体
	rn.out["openssl x509 -enddate -noout -in '/x/y' 2>/dev/null"] = "notAfter=Jun 12 12:00:00 2026 GMT"
	t2, err := ParseExpiry(rn, "/x/y")
	if err != nil {
		t.Fatal(err)
	}
	if t2.Day() != 12 {
		t.Errorf("单空格解析错: %v", t2)
	}

	rn.out["openssl x509 -enddate -noout -in '/x/y' 2>/dev/null"] = "garbage"
	if _, err := ParseExpiry(rn, "/x/y"); err == nil {
		t.Error("乱码应报错")
	}
}

func TestLivePaths(t *testing.T) {
	if got := LiveCertPath("a.test"); got != "/etc/letsencrypt/live/a.test/fullchain.pem" {
		t.Errorf("cert path: %s", got)
	}
	if got := LiveKeyPath("a.test"); got != "/etc/letsencrypt/live/a.test/privkey.pem" {
		t.Errorf("key path: %s", got)
	}
}
