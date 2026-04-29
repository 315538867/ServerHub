package ssl

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/acme"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/wsstream"
	"github.com/serverhub/serverhub/usecase"
	"github.com/serverhub/serverhub/repo"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096}

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.GET("/:id/ssl/certs", listCertsHandler(db, cfg))
	r.GET("/:id/ssl/certs/request", requestCertHandler(db, cfg))
	r.POST("/:id/ssl/certs/upload", uploadCertHandler(db, cfg))
	r.GET("/:id/ssl/certs/:cid/renew", renewCertHandler(db, cfg))
	r.DELETE("/:id/ssl/certs/:cid", deleteCertHandler(db))
	r.POST("/:id/ssl/certs/scan", scanCertsHandler(db, cfg))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseServerID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return 0, false
	}
	return uint(id), true
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
	HasPEM        bool   `json:"has_pem"`
	LastRenewedAt string `json:"last_renewed_at,omitempty"`
}

func listCertsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseServerID(c)
		if !ok {
			return
		}
		var appID *uint
		if s := c.Query("application_id"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				u := uint(v)
				appID = &u
			}
		}
		certs, err := usecase.ListSSLCerts(c.Request.Context(), db, serverID, appID)
		if err != nil {
			resp.InternalError(c, "查询证书失败")
			return
		}
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

func requestCertHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseServerID(c)
		if !ok {
			return
		}
		rn, s, err := usecase.GetServerDedicatedRunner(c.Request.Context(), db, cfg, serverID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
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
				return
			}
			_ = usecase.PersistCert(c.Request.Context(), db, rn, cfg, s.ID, domain, pem.Cert, pem.Key, "Let's Encrypt", true, true)
		}()
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

// ── upload cert ───────────────────────────────────────────────────────────────

func uploadCertHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseServerID(c)
		if !ok {
			return
		}
		rn, s, err := usecase.GetServerRunner(c.Request.Context(), db, cfg, serverID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
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
		if err := usecase.PersistCert(c.Request.Context(), db, rn, cfg, s.ID, body.Domain, body.Cert, body.Key, "manual", false, false); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── renew cert ────────────────────────────────────────────────────────────────

func renewCertHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseServerID(c)
		if !ok {
			return
		}
		rn, s, err := usecase.GetServerDedicatedRunner(c.Request.Context(), db, cfg, serverID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
			return
		}
		defer rn.Close()

		cid, _ := strconv.Atoi(c.Param("cid"))
		cert, err := usecase.GetCertByServerAndCertID(c.Request.Context(), db, s.ID, uint(cid))
		if err != nil {
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
			_ = usecase.PersistCert(c.Request.Context(), db, rn, cfg, s.ID, cert.Domain, pem.Cert, pem.Key, cert.Issuer, cert.AutoRenew, true)
		}()
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}

// ── delete cert ───────────────────────────────────────────────────────────────

func deleteCertHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, _ := strconv.Atoi(c.Param("cid"))
		if err := usecase.DeleteSSLCert(c.Request.Context(), db, uint(cid)); err != nil {
			resp.InternalError(c, "删除失败")
			return
		}
		resp.OK(c, nil)
	}
}

// ── scan certs ────────────────────────────────────────────────────────────────

func scanCertsHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, ok := parseServerID(c)
		if !ok {
			return
		}
		rn, s, err := usecase.GetServerRunner(c.Request.Context(), db, cfg, serverID)
		if err != nil {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
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
			if err := usecase.PersistCert(c.Request.Context(), db, rn, cfg, s.ID, domain, pem.Cert, pem.Key, "Let's Encrypt", true, false); err != nil {
				continue
			}
			imported++
		}
		resp.OK(c, gin.H{"imported": imported})
	}
}
