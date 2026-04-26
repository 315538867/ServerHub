package scheduler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/runner"
)

// fakeRunner 走查表式的 Run：按 cmd 子串匹配，命中第一条就返回。
// scheduler 的 cert_renew 走 acme 的几个固定命令，子串匹配足够稳。
type fakeRunner struct {
	rules []rule
	calls []string
}

type rule struct {
	contains string
	out      string
	err      error
}

func (f *fakeRunner) Run(cmd string) (string, error) {
	f.calls = append(f.calls, cmd)
	for _, r := range f.rules {
		if strings.Contains(cmd, r.contains) {
			return r.out, r.err
		}
	}
	return "", errors.New("unexpected cmd: " + cmd)
}
func (f *fakeRunner) NewSession() (runner.Session, error) { return nil, errors.New("nope") }
func (f *fakeRunner) IsLocal() bool                       { return false }
func (f *fakeRunner) Capability() string                  { return "full" }
func (f *fakeRunner) Close() error                        { return nil }

func newCertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 用 t.Name() 当 DSN 名拿到独立的内存 DB，避免多个子测试共享 cache=shared 导致互相污染。
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Server{}, &model.SSLCert{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// 32 字节 AES key，单测共用。
const testAES = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func installFakeCertRunner(t *testing.T, fr *fakeRunner) {
	t.Helper()
	old := SetCertRunnerFactory(func(*model.Server, *config.Config) (runner.Runner, error) {
		return fr, nil
	})
	t.Cleanup(func() { SetCertRunnerFactory(old) })
}

// 到期 < 30 天的应被续签；> 30 天的不动；auto_renew=false 也不动。
func TestRenewExpiringCerts_FiltersAndRenews(t *testing.T) {
	db := newCertTestDB(t)
	cfg := &config.Config{}
	cfg.Security.AESKey = testAES

	srv := model.Server{Name: "edge", Type: "ssh"}
	if err := db.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	expiring := model.SSLCert{
		ServerID: srv.ID, Domain: "soon.example.com",
		ExpiresAt: now.Add(10 * 24 * time.Hour), AutoRenew: true,
	}
	farFuture := model.SSLCert{
		ServerID: srv.ID, Domain: "far.example.com",
		ExpiresAt: now.Add(90 * 24 * time.Hour), AutoRenew: true,
	}
	manual := model.SSLCert{
		ServerID: srv.ID, Domain: "manual.example.com",
		ExpiresAt: now.Add(5 * 24 * time.Hour),
	}
	for _, c := range []*model.SSLCert{&expiring, &farFuture, &manual} {
		if err := db.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	// AutoRenew 字段带 gorm:"default:true"，Create 时零值（false）会被 default 替换为 true，
	// 因此手动 Update 强制设为 false，模拟"用户关掉自动续签"的真实状态。
	if err := db.Model(&manual).Update("auto_renew", false).Error; err != nil {
		t.Fatal(err)
	}

	newCertPEM := "-----BEGIN CERTIFICATE-----\nFRESH\n-----END CERTIFICATE-----"
	newKeyPEM := "-----BEGIN PRIVATE KEY-----\nFRESH-KEY\n-----END PRIVATE KEY-----"
	newExpiry := now.Add(89 * 24 * time.Hour).UTC().Truncate(time.Second)
	expiryFormatted := newExpiry.Format("Jan  2 15:04:05 2006") + " GMT"

	// 注意：openssl 命令的参数里也带 fullchain.pem，所以 openssl 规则必须排在 cat 规则之前，
	// 否则会被前缀匹配命中。
	fr := &fakeRunner{rules: []rule{
		{contains: "certbot renew", out: "Congratulations, all renewals succeeded"},
		{contains: "openssl x509", out: "notAfter=" + expiryFormatted + "\n"},
		{contains: "fullchain.pem", out: newCertPEM},
		{contains: "privkey.pem", out: newKeyPEM},
	}}
	installFakeCertRunner(t, fr)

	renewed, failed := RenewExpiringCerts(db, cfg)
	if renewed != 1 || failed != 0 {
		t.Errorf("renewed=%d failed=%d, want 1/0", renewed, failed)
	}

	// 数据库里 expiring 应被回写
	var got model.SSLCert
	if err := db.First(&got, expiring.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.CertPEM == "" || got.KeyPEM == "" {
		t.Errorf("PEM 未回写")
	}
	plain, err := crypto.Decrypt(got.CertPEM, testAES)
	if err != nil {
		t.Fatalf("解密 cert 失败: %v", err)
	}
	if plain != newCertPEM {
		t.Errorf("cert 内容不符: got %q", plain)
	}
	if got.LastRenewedAt == nil {
		t.Errorf("LastRenewedAt 未刷新")
	}
	if !got.ExpiresAt.Equal(newExpiry) {
		t.Errorf("ExpiresAt 未刷新: got %v want %v", got.ExpiresAt, newExpiry)
	}

	// far 和 manual 均不应被触发——保留原始 ExpiresAt 且 PEM 仍空
	var farGot, manualGot model.SSLCert
	db.First(&farGot, farFuture.ID)
	db.First(&manualGot, manual.ID)
	if farGot.CertPEM != "" {
		t.Errorf("远期证书不应被续签")
	}
	if manualGot.CertPEM != "" {
		t.Errorf("auto_renew=false 不应被续签")
	}
}

// certbot 失败时 failed 计数应递增；DB 不应被改写。
func TestRenewExpiringCerts_CertbotFailure(t *testing.T) {
	db := newCertTestDB(t)
	cfg := &config.Config{}
	cfg.Security.AESKey = testAES

	srv := model.Server{Name: "edge", Type: "ssh"}
	db.Create(&srv)

	cert := model.SSLCert{
		ServerID: srv.ID, Domain: "fail.example.com",
		ExpiresAt: time.Now().Add(5 * 24 * time.Hour), AutoRenew: true,
	}
	db.Create(&cert)

	fr := &fakeRunner{rules: []rule{
		{contains: "certbot renew", out: "boom", err: errors.New("exit 1")},
	}}
	installFakeCertRunner(t, fr)

	renewed, failed := RenewExpiringCerts(db, cfg)
	if renewed != 0 || failed != 1 {
		t.Errorf("renewed=%d failed=%d, want 0/1", renewed, failed)
	}

	var got model.SSLCert
	db.First(&got, cert.ID)
	if got.CertPEM != "" || got.LastRenewedAt != nil {
		t.Errorf("失败时不应回写: %+v", got)
	}
}

// expires_at 零值（未知）应被排除——避免 "= 0001-01-01" 也命中 ≤ now+30d。
func TestRenewExpiringCerts_SkipsZeroExpiry(t *testing.T) {
	db := newCertTestDB(t)
	cfg := &config.Config{}
	cfg.Security.AESKey = testAES

	srv := model.Server{Name: "edge"}
	db.Create(&srv)

	zeroCert := model.SSLCert{
		ServerID: srv.ID, Domain: "unknown.example.com",
		AutoRenew: true, // ExpiresAt 留零值
	}
	db.Create(&zeroCert)

	fr := &fakeRunner{} // 没注册任何 rule——若被错误地触发 renew 会立刻报错
	installFakeCertRunner(t, fr)

	renewed, failed := RenewExpiringCerts(db, cfg)
	if renewed != 0 || failed != 0 {
		t.Errorf("零值 expires_at 应被排除：renewed=%d failed=%d", renewed, failed)
	}
	if len(fr.calls) != 0 {
		t.Errorf("不应有任何 runner 调用，实际: %v", fr.calls)
	}
}
