// Package acme 封装 ACME 证书的申请 / 续签 / 读取，对调用方隐藏底层是
// certbot 还是 acme.sh。当前实现走 certbot HTTP-01 + webroot —— 项目历史上
// 已经依赖它（旧 ssl handler），换成 acme.sh 留作后续。
//
// 设计原则：
//   - 所有命令通过 runner.Runner 执行，不裸调 exec/sshpool；
//   - 输入域名/邮箱/webroot 严格 shell-quote，禁止注入；
//   - PEM 内容由调用方自己加密入库，pkg/acme 只负责拿到原文。
package acme

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/pkg/runner"
)

// LiveDir certbot 默认的证书目录——renew 后 fullchain.pem / privkey.pem 都在这。
const LiveDir = "/etc/letsencrypt/live"

// IssueOpts 一次 HTTP-01 申请的输入。
type IssueOpts struct {
	Domain  string
	Email   string
	Webroot string // 默认 /var/www/html
	Staging bool   // 走 LE staging，避免单测/预发把真服务的速率打满
}

// IssueCmd 拼接 certbot certonly 命令；不执行，方便 wsstream.Stream 流式跑。
func IssueCmd(o IssueOpts) (string, error) {
	if err := validateDomain(o.Domain); err != nil {
		return "", err
	}
	webroot := o.Webroot
	if webroot == "" {
		webroot = "/var/www/html"
	}
	email := o.Email
	if email == "" {
		email = "admin@" + o.Domain
	}
	parts := []string{
		"certbot certonly --webroot",
		"-w " + shellQuote(webroot),
		"-d " + shellQuote(o.Domain),
		"--email " + shellQuote(email),
		"--agree-tos --non-interactive",
	}
	if o.Staging {
		parts = append(parts, "--staging")
	}
	parts = append(parts, "2>&1")
	return strings.Join(parts, " "), nil
}

// RenewCmd 续签——走 --cert-name 精确指定，不会顺带把别的也 renew。
func RenewCmd(domain string) (string, error) {
	if err := validateDomain(domain); err != nil {
		return "", err
	}
	return fmt.Sprintf("certbot renew --cert-name %s --non-interactive 2>&1", shellQuote(domain)), nil
}

// PEM letsencrypt live 目录下读到的两段证书。
type PEM struct {
	Cert string
	Key  string
}

// ReadPEM 把 fullchain.pem 和 privkey.pem 从远端拉回内存。
// 用 cat 而非 sftp，是因为 runner 抽象里的 SSH 路径只暴露了 Run。
func ReadPEM(rn runner.Runner, domain string) (PEM, error) {
	if err := validateDomain(domain); err != nil {
		return PEM{}, err
	}
	dir := LiveDir + "/" + domain
	cert, err := rn.Run("cat " + shellQuote(dir+"/fullchain.pem"))
	if err != nil {
		return PEM{}, fmt.Errorf("读 fullchain: %w", err)
	}
	key, err := rn.Run("cat " + shellQuote(dir+"/privkey.pem"))
	if err != nil {
		return PEM{}, fmt.Errorf("读 privkey: %w", err)
	}
	cert = strings.TrimSpace(cert)
	key = strings.TrimSpace(key)
	if !strings.Contains(cert, "BEGIN CERTIFICATE") {
		return PEM{}, fmt.Errorf("fullchain 内容异常: %s", truncate(cert))
	}
	if !strings.Contains(key, "PRIVATE KEY") {
		return PEM{}, fmt.Errorf("privkey 内容异常: %s", truncate(key))
	}
	return PEM{Cert: cert, Key: key}, nil
}

// ParseExpiry 用 openssl 读证书 notAfter；上层用来判断要不要续签。
func ParseExpiry(rn runner.Runner, certPath string) (time.Time, error) {
	out, err := rn.Run(fmt.Sprintf("openssl x509 -enddate -noout -in %s 2>/dev/null", shellQuote(certPath)))
	if err != nil {
		return time.Time{}, err
	}
	out = strings.TrimSpace(out)
	after, found := strings.CutPrefix(out, "notAfter=")
	if !found {
		return time.Time{}, fmt.Errorf("openssl 输出异常: %s", truncate(out))
	}
	after = strings.TrimSpace(after)
	t, err := time.Parse("Jan  2 15:04:05 2006 GMT", after)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05 2006 GMT", after)
	}
	return t, err
}

// LiveCertPath / LiveKeyPath certbot 默认存放路径，renew 完写回 SSLCert 用。
func LiveCertPath(domain string) string { return LiveDir + "/" + domain + "/fullchain.pem" }
func LiveKeyPath(domain string) string  { return LiveDir + "/" + domain + "/privkey.pem" }

// validateDomain 限制域名只含字母数字 . - *，挡住命令注入。
func validateDomain(d string) error {
	d = strings.TrimSpace(d)
	if d == "" {
		return errors.New("domain 不能为空")
	}
	if len(d) > 253 {
		return errors.New("domain 过长")
	}
	for _, r := range d {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '*':
			// ok
		default:
			return fmt.Errorf("domain 含非法字符: %q", r)
		}
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
