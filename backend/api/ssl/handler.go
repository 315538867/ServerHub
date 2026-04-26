package ssl

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/acme"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/nginxrender"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/sftppool"
	"github.com/serverhub/serverhub/pkg/wsstream"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.GET("/:id/ssl/certs", listCertsHandler(db, cfg))
	r.GET("/:id/ssl/certs/request", requestCertHandler(db, cfg))
	r.POST("/:id/ssl/certs/upload", uploadCertHandler(db, cfg))
	r.GET("/:id/ssl/certs/:cid/renew", renewCertHandler(db, cfg))
	r.DELETE("/:id/ssl/certs/:cid", deleteCertHandler(db))
	r.POST("/:id/ssl/certs/scan", scanCertsHandler(db, cfg))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func loadServer(c *gin.Context, db *gorm.DB) (*model.Server, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, false
	}
	return &s, true
}

func getRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, *model.Server, bool) {
	s, ok := loadServer(c, db)
	if !ok {
		return nil, nil, false
	}
	rn, err := runner.For(s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, nil, false
	}
	return rn, s, true
}

func getDedicatedRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, *model.Server, bool) {
	s, ok := loadServer(c, db)
	if !ok {
		return nil, nil, false
	}
	rn, err := runner.ForDedicated(s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, nil, false
	}
	return rn, s, true
}

// writeRemoteFile writes content to path on the target (local = os; ssh = sftp).
func writeRemoteFile(rn runner.Runner, serverID uint, path, content string, mode os.FileMode) error {
	if rn.IsLocal() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), mode)
	}
	cli := runner.SSHClient(rn)
	if cli == nil {
		return fmt.Errorf("no ssh client")
	}
	sc, err := sftppool.Get(serverID, cli)
	if err != nil {
		return err
	}
	// sftppool 创建后需要单独 Chmod；先 Create + Write 再 Chmod，最大化兼容。
	f, err := sc.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return sc.Chmod(path, mode)
}

// persistCert 把 PEM 内容加密落库并写到 canonical 远端路径。Issuer / AutoRenew /
// LastRenewedAt 由调用方决定，剩下的 schema 字段在这里统一计算。upload / request
// / renew / scan 共享。
func persistCert(
	db *gorm.DB, rn runner.Runner, cfg *config.Config,
	serverID uint, domain, certPEM, keyPEM, issuer string,
	autoRenew bool, markRenew bool,
) error {
	encCert, err := crypto.Encrypt(certPEM, cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("加密 cert: %w", err)
	}
	encKey, err := crypto.Encrypt(keyPEM, cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("加密 key: %w", err)
	}
	certPath, keyPath := nginxrender.CertCanonicalPaths(domain)

	// 远端先落盘——上传后立即可用，不必等下一次 apply。Reconciler 后续 apply
	// 看到内容相同会 noop（base64 tee 幂等）。
	if err := writeRemoteFile(rn, serverID, certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("写入远端 cert: %w", err)
	}
	if err := writeRemoteFile(rn, serverID, keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("写入远端 key: %w", err)
	}

	expiry, _ := parseExpiryFromPEM(certPEM)

	now := time.Now()
	updates := map[string]any{
		"cert_path":  certPath,
		"key_path":   keyPath,
		"cert_pem":   encCert,
		"key_pem":    encKey,
		"issuer":     issuer,
		"expires_at": expiry,
		"auto_renew": autoRenew,
	}
	if markRenew {
		updates["last_renewed_at"] = &now
	}

	var existing model.SSLCert
	err = db.Where("server_id = ? AND domain = ?", serverID, domain).First(&existing).Error
	switch {
	case err == nil:
		return db.Model(&existing).Updates(updates).Error
	case err == gorm.ErrRecordNotFound:
		cert := model.SSLCert{
			ServerID:  serverID,
			Domain:    domain,
			CertPath:  certPath,
			KeyPath:   keyPath,
			CertPEM:   encCert,
			KeyPEM:    encKey,
			Issuer:    issuer,
			ExpiresAt: expiry,
			AutoRenew: autoRenew,
		}
		if markRenew {
			cert.LastRenewedAt = &now
		}
		return db.Create(&cert).Error
	default:
		return err
	}
}

// parseExpiryFromPEM 用本地 openssl 解析（不打远端，handler 拿到 PEM 文本时就算）。
// 失败返回零值；调用方接受零值——expires_at 不是强必需字段，scheduler 续签兜底。
func parseExpiryFromPEM(certPEM string) (time.Time, error) {
	tmp, err := os.CreateTemp("", "sh-cert-*.pem")
	if err != nil {
		return time.Time{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(certPEM); err != nil {
		_ = tmp.Close()
		return time.Time{}, err
	}
	_ = tmp.Close()
	out, err := runLocal("openssl x509 -enddate -noout -in " + shellQuote(tmp.Name()))
	if err != nil {
		return time.Time{}, err
	}
	out = strings.TrimSpace(out)
	after, found := strings.CutPrefix(out, "notAfter=")
	if !found {
		return time.Time{}, fmt.Errorf("openssl 输出异常: %s", out)
	}
	after = strings.TrimSpace(after)
	t, err := time.Parse("Jan  2 15:04:05 2006 GMT", after)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05 2006 GMT", after)
	}
	return t, err
}

// ── cert list ─────────────────────────────────────────────────────────────────

type certResp struct {
	ID            uint   `json:"id"`
	Domain        string `json:"domain"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	Issuer        string `json:"issuer"`
	ExpiresAt     string `json:"expires_at"`
	DaysLeft      int    `json:"days_left"`
	AutoRenew     bool   `json:"auto_renew"`
	HasPEM        bool   `json:"has_pem"` // 是否已加密入库（前端用来给图标标记）
	LastRenewedAt string `json:"last_renewed_at,omitempty"`
}

func listCertsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		var certs []model.SSLCert
		q := db.Where("server_id = ?", id)
		if appID := c.Query("application_id"); appID != "" {
			q = q.Where("application_id = ?", appID)
		}
		q.Find(&certs)
		result := make([]certResp, len(certs))
		for i, cert := range certs {
			days := int(time.Until(cert.ExpiresAt).Hours() / 24)
			r := certResp{
				ID: cert.ID, Domain: cert.Domain,
				CertPath: cert.CertPath, KeyPath: cert.KeyPath,
				Issuer:    cert.Issuer,
				ExpiresAt: cert.ExpiresAt.Format("2006-01-02"),
				DaysLeft:  days, AutoRenew: cert.AutoRenew,
				HasPEM: cert.CertPEM != "" && cert.KeyPEM != "",
			}
			if cert.LastRenewedAt != nil {
				r.LastRenewedAt = cert.LastRenewedAt.Format("2006-01-02 15:04:05")
			}
			result[i] = r
		}
		resp.OK(c, result)
	}
}

// ── request cert (Let's Encrypt) ──────────────────────────────────────────────

func requestCertHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, s, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer rn.Close()
		domain := c.Query("domain")
		if domain == "" {
			resp.BadRequest(c, "域名不能为空")
			return
		}
		webroot := c.Query("webroot")
		email := c.Query("email")

		issueCmd, err := acme.IssueCmd(acme.IssueOpts{
			Domain: domain, Webroot: webroot, Email: email,
		})
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()

		go func() {
			wsstream.Stream(ws, rn, issueCmd, wsstream.Opts{})
			pem, err := acme.ReadPEM(rn, domain)
			if err != nil {
				return // certbot 失败时 letsencrypt 没生成文件，跳过入库
			}
			_ = persistCert(db, rn, cfg, s.ID, domain, pem.Cert, pem.Key, "Let's Encrypt", true, true)
		}()
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

// ── upload cert ───────────────────────────────────────────────────────────────

func uploadCertHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, s, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Domain string `json:"domain" binding:"required"`
			Cert   string `json:"cert"   binding:"required"`
			Key    string `json:"key"    binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "域名、证书和密钥不能为空")
			return
		}
		if !strings.Contains(body.Cert, "BEGIN CERTIFICATE") {
			resp.BadRequest(c, "cert 不是合法 PEM")
			return
		}
		if !strings.Contains(body.Key, "PRIVATE KEY") {
			resp.BadRequest(c, "key 不是合法 PEM")
			return
		}
		if err := persistCert(db, rn, cfg, s.ID, body.Domain, body.Cert, body.Key, "manual", false, false); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── renew cert ────────────────────────────────────────────────────────────────

func renewCertHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, s, ok := getDedicatedRunner(c, db, cfg)
		if !ok {
			return
		}
		defer rn.Close()
		cid, _ := strconv.Atoi(c.Param("cid"))
		var cert model.SSLCert
		if err := db.Where("server_id = ? AND id = ?", s.ID, cid).First(&cert).Error; err != nil {
			resp.NotFound(c, "证书不存在")
			return
		}
		ws, err := middleware.WSUpgrade(upgrader, c)
		if err != nil {
			return
		}
		defer ws.Close()

		renewCmd, err := acme.RenewCmd(cert.Domain)
		if err != nil {
			return
		}
		go func() {
			wsstream.Stream(ws, rn, renewCmd, wsstream.Opts{})
			pem, err := acme.ReadPEM(rn, cert.Domain)
			if err != nil {
				return
			}
			_ = persistCert(db, rn, cfg, s.ID, cert.Domain, pem.Cert, pem.Key, cert.Issuer, cert.AutoRenew, true)
		}()
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

// ── delete cert ───────────────────────────────────────────────────────────────

func deleteCertHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, _ := strconv.Atoi(c.Param("cid"))
		if err := db.Delete(&model.SSLCert{}, cid).Error; err != nil {
			resp.InternalError(c, "删除失败")
			return
		}
		resp.OK(c, nil)
	}
}

// ── scan certs ────────────────────────────────────────────────────────────────

func scanCertsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		rn, s, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, _ := rn.Run("ls /etc/letsencrypt/live/ 2>/dev/null")
		imported := 0
		for _, domain := range strings.Fields(out) {
			domain = strings.TrimSpace(domain)
			if domain == "" || domain == "README" {
				continue
			}
			pem, err := acme.ReadPEM(rn, domain)
			if err != nil {
				continue
			}
			if err := persistCert(db, rn, cfg, s.ID, domain, pem.Cert, pem.Key, "Let's Encrypt", true, false); err != nil {
				continue
			}
			imported++
		}
		resp.OK(c, gin.H{"imported": imported})
	}
}

// ── utils ─────────────────────────────────────────────────────────────────────

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// runLocal 给 parseExpiryFromPEM 用，本地 fork openssl 不走 runner。
func runLocal(cmd string) (string, error) {
	return execShell(cmd)
}
