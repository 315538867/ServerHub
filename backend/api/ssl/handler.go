package ssl

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sftppool"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/wsstream"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/ssl/certs", listCertsHandler(db, cfg))
	r.GET("/:id/ssl/certs/request", requestCertHandler(db, cfg))
	r.POST("/:id/ssl/certs/upload", uploadCertHandler(db, cfg))
	r.GET("/:id/ssl/certs/:cid/renew", renewCertHandler(db, cfg))
	r.DELETE("/:id/ssl/certs/:cid", deleteCertHandler(db))
	r.POST("/:id/ssl/certs/scan", scanCertsHandler(db, cfg))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func getSSH(c *gin.Context, db *gorm.DB, cfg *config.Config) (*gossh.Client, *model.Server, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, nil, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, nil, false
	}
	var cred string
	switch s.AuthType {
	case "key":
		if s.PrivateKey != "" {
			cred, err = crypto.Decrypt(s.PrivateKey, cfg.Security.AESKey)
		}
	default:
		if s.Password != "" {
			cred, err = crypto.Decrypt(s.Password, cfg.Security.AESKey)
		}
	}
	if err != nil {
		resp.InternalError(c, "解密失败")
		return nil, nil, false
	}
	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, nil, false
	}
	return client, &s, true
}

func streamSSH(ws *websocket.Conn, client *gossh.Client, cmd string) {
	wsstream.Stream(ws, client, cmd, wsstream.Opts{})
}

// ── cert list ─────────────────────────────────────────────────────────────────

type certResp struct {
	ID        uint   `json:"id"`
	Domain    string `json:"domain"`
	CertPath  string `json:"cert_path"`
	KeyPath   string `json:"key_path"`
	Issuer    string `json:"issuer"`
	ExpiresAt string `json:"expires_at"`
	DaysLeft  int    `json:"days_left"`
	AutoRenew bool   `json:"auto_renew"`
}

func listCertsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		var certs []model.SSLCert
		db.Where("server_id = ?", id).Find(&certs)
		result := make([]certResp, len(certs))
		for i, cert := range certs {
			days := int(time.Until(cert.ExpiresAt).Hours() / 24)
			result[i] = certResp{
				ID: cert.ID, Domain: cert.Domain,
				CertPath: cert.CertPath, KeyPath: cert.KeyPath,
				Issuer:    cert.Issuer,
				ExpiresAt: cert.ExpiresAt.Format("2006-01-02"),
				DaysLeft:  days, AutoRenew: cert.AutoRenew,
			}
		}
		resp.OK(c, result)
	}
}

// ── request cert (Let's Encrypt) ──────────────────────────────────────────────

func requestCertHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, s, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		domain := c.Query("domain")
		if domain == "" {
			resp.BadRequest(c, "域名不能为空")
			return
		}
		webroot := c.Query("webroot")
		if webroot == "" {
			webroot = "/var/www/html"
		}
		email := c.Query("email")
		if email == "" {
			email = "admin@" + domain
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		cmd := fmt.Sprintf(
			"certbot certonly --webroot -w %s -d %s --email %s --agree-tos --non-interactive 2>&1",
			shellQuote(webroot), shellQuote(domain), shellQuote(email),
		)

		go func() {
			streamSSH(ws, client, cmd)
			certPath := "/etc/letsencrypt/live/" + domain + "/fullchain.pem"
			keyPath := "/etc/letsencrypt/live/" + domain + "/privkey.pem"
			expiry, _ := parseCertExpiry(client, certPath)
			cert := model.SSLCert{
				ServerID:  s.ID,
				Domain:    domain,
				CertPath:  certPath,
				KeyPath:   keyPath,
				Issuer:    "Let's Encrypt",
				ExpiresAt: expiry,
				AutoRenew: true,
			}
			db.Where(model.SSLCert{ServerID: s.ID, Domain: domain}).Assign(cert).FirstOrCreate(&cert)
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
		client, s, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Domain   string `json:"domain"    binding:"required"`
			Cert     string `json:"cert"      binding:"required"`
			Key      string `json:"key"       binding:"required"`
			CertPath string `json:"cert_path"`
			KeyPath  string `json:"key_path"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "域名、证书和密钥不能为空")
			return
		}
		if body.CertPath == "" {
			body.CertPath = "/etc/ssl/certs/" + body.Domain + ".pem"
		}
		if body.KeyPath == "" {
			body.KeyPath = "/etc/ssl/private/" + body.Domain + ".key"
		}

		id, _ := strconv.Atoi(c.Param("id"))
		sc, err := sftppool.Get(uint(id), client)
		if err != nil {
			resp.InternalError(c, "SFTP 连接失败: "+err.Error())
			return
		}

		writeFile := func(path, content string) error {
			f, err := sc.Create(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.Write([]byte(content))
			return err
		}
		if err := writeFile(body.CertPath, body.Cert); err != nil {
			resp.InternalError(c, "写入证书失败: "+err.Error())
			return
		}
		if err := writeFile(body.KeyPath, body.Key); err != nil {
			resp.InternalError(c, "写入密钥失败: "+err.Error())
			return
		}

		expiry, _ := parseCertExpiry(client, body.CertPath)
		cert := model.SSLCert{
			ServerID:  s.ID,
			Domain:    body.Domain,
			CertPath:  body.CertPath,
			KeyPath:   body.KeyPath,
			Issuer:    "manual",
			ExpiresAt: expiry,
			AutoRenew: false,
		}
		db.Where(model.SSLCert{ServerID: s.ID, Domain: body.Domain}).Assign(cert).FirstOrCreate(&cert)
		resp.OK(c, nil)
	}
}

// ── renew cert ────────────────────────────────────────────────────────────────

func renewCertHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, s, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		cid, _ := strconv.Atoi(c.Param("cid"))
		var cert model.SSLCert
		if err := db.Where("server_id = ? AND id = ?", s.ID, cid).First(&cert).Error; err != nil {
			resp.NotFound(c, "证书不存在")
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		cmd := fmt.Sprintf("certbot renew --cert-name %s --non-interactive 2>&1", shellQuote(cert.Domain))
		go func() {
			streamSSH(ws, client, cmd)
			expiry, _ := parseCertExpiry(client, cert.CertPath)
			if !expiry.IsZero() {
				db.Model(&cert).Update("expires_at", expiry)
			}
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
		client, s, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		out, _ := sshpool.Run(client, "ls /etc/letsencrypt/live/ 2>/dev/null")
		imported := 0
		for _, domain := range strings.Fields(out) {
			domain = strings.TrimSpace(domain)
			if domain == "" || domain == "README" {
				continue
			}
			certPath := "/etc/letsencrypt/live/" + domain + "/fullchain.pem"
			keyPath := "/etc/letsencrypt/live/" + domain + "/privkey.pem"
			expiry, err := parseCertExpiry(client, certPath)
			if err != nil {
				continue
			}
			cert := model.SSLCert{
				ServerID:  s.ID,
				Domain:    domain,
				CertPath:  certPath,
				KeyPath:   keyPath,
				Issuer:    "Let's Encrypt",
				ExpiresAt: expiry,
				AutoRenew: true,
			}
			db.Where(model.SSLCert{ServerID: s.ID, Domain: domain}).Assign(cert).FirstOrCreate(&cert)
			imported++
		}
		resp.OK(c, gin.H{"imported": imported})
	}
}

// ── utils ─────────────────────────────────────────────────────────────────────

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func parseCertExpiry(client *gossh.Client, certPath string) (time.Time, error) {
	out, err := sshpool.Run(client, fmt.Sprintf(
		"openssl x509 -enddate -noout -in %s 2>/dev/null", shellQuote(certPath),
	))
	if err != nil {
		return time.Time{}, err
	}
	// output: "notAfter=Jan  1 00:00:00 2026 GMT"
	out = strings.TrimSpace(out)
	after, found := strings.CutPrefix(out, "notAfter=")
	if !found {
		return time.Time{}, fmt.Errorf("unexpected output: %s", out)
	}
	t, err := time.Parse("Jan  2 15:04:05 2006 GMT", strings.TrimSpace(after))
	if err != nil {
		// try single-digit day
		t, err = time.Parse("Jan 2 15:04:05 2006 GMT", strings.TrimSpace(after))
	}
	return t, err
}
