package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/deployer"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("", listHandler(db))
	r.POST("", createHandler(db))
	r.GET("/:id", getHandler(db))
	r.PUT("/:id", updateHandler(db))
	r.DELETE("/:id", deleteHandler(db))
	r.POST("/:id/run", runHandler(db, cfg))
	r.GET("/:id/logs", logsHandler(db))
	r.GET("/:id/env", getEnvHandler(db, cfg))
	r.PUT("/:id/env", putEnvHandler(db, cfg))
	r.POST("/:id/rollback", rollbackHandler(db, cfg))
	r.GET("/:id/webhook", webhookInfoHandler(db))
	r.POST("/:id/upload", uploadHandler(db, cfg))
}

// ── CRUD ──────────────────────────────────────────────────────────────────

type deployReq struct {
	Name        string `json:"name" binding:"required"`
	ServerID    uint   `json:"server_id" binding:"required"`
	Type        string `json:"type"`
	WorkDir     string `json:"work_dir"`
	ComposeFile string `json:"compose_file"`
	StartCmd    string `json:"start_cmd"`
	ImageName   string `json:"image_name"`
	// Version management
	DesiredVersion string `json:"desired_version"`
	AutoSync       bool   `json:"auto_sync"`
	SyncInterval   int    `json:"sync_interval"`
}

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var deploys []model.Deploy
		db.Order("id asc").Find(&deploys)
		resp.OK(c, deploys)
	}
}

func createHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req deployReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		applyDefaults(&req)
		d := model.Deploy{
			Name: req.Name, ServerID: req.ServerID, Type: req.Type,
			WorkDir: req.WorkDir, ComposeFile: req.ComposeFile,
			StartCmd: req.StartCmd, ImageName: req.ImageName,
			DesiredVersion: req.DesiredVersion,
			AutoSync:       req.AutoSync, SyncInterval: req.SyncInterval,
			WebhookSecret: generateSecret(),
		}
		if req.DesiredVersion != "" {
			d.SyncStatus = "drifted"
		}
		if err := db.Create(&d).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		c.JSON(http.StatusCreated, gin.H{"code": 0, "msg": "ok", "data": d})
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		resp.OK(c, d)
	}
}

func updateHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		var req deployReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		applyDefaults(&req)

		updates := map[string]any{
			"name": req.Name, "server_id": req.ServerID, "type": req.Type,
			"work_dir": req.WorkDir, "compose_file": req.ComposeFile,
			"start_cmd": req.StartCmd, "image_name": req.ImageName,
			"desired_version": req.DesiredVersion,
			"auto_sync":       req.AutoSync, "sync_interval": req.SyncInterval,
		}
		// Mark drifted when desired_version changes and doesn't match actual
		if req.DesiredVersion != "" && req.DesiredVersion != d.ActualVersion {
			updates["sync_status"] = "drifted"
		}
		db.Model(&d).Updates(updates)
		resp.OK(c, d)
	}
}

func deleteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		db.Delete(&d)
		db.Where("deploy_id = ?", d.ID).Delete(&model.DeployLog{})
		resp.OK(c, nil)
	}
}

func logsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		limit := 20
		if l := c.Query("limit"); l != "" {
			if v, _ := strconv.Atoi(l); v > 0 && v <= 100 {
				limit = v
			}
		}
		var logs []model.DeployLog
		db.Where("deploy_id = ?", d.ID).Order("created_at desc").Limit(limit).Find(&logs)
		resp.OK(c, logs)
	}
}

// ── Run (SSE) ──────────────────────────────────────────────────────────────

func runHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Header("Access-Control-Allow-Origin", "*")

		sendEvent := func(eventType, data string) {
			payload, _ := json.Marshal(map[string]string{"type": eventType, "line": data})
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			c.Writer.Flush()
		}

		result := deployer.Run(db, cfg, d, "manual", func(line string) {
			sendEvent("output", line)
		})
		if result.Success {
			sendEvent("done", "success")
		} else {
			sendEvent("done", "failed")
		}
	}
}

// ── Rollback ───────────────────────────────────────────────────────────────

func rollbackHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		if d.PreviousVersion == "" {
			resp.BadRequest(c, "无历史版本记录")
			return
		}

		db.Model(&d).Updates(map[string]any{
			"desired_version": d.PreviousVersion,
			"sync_status":     "drifted",
		})
		d.DesiredVersion = d.PreviousVersion

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Header("Access-Control-Allow-Origin", "*")

		sendEvent := func(eventType, data string) {
			payload, _ := json.Marshal(map[string]string{"type": eventType, "line": data})
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			c.Writer.Flush()
		}

		result := deployer.Run(db, cfg, d, "manual", func(line string) {
			sendEvent("output", line)
		})
		if result.Success {
			sendEvent("done", "success")
		} else {
			sendEvent("done", "failed")
		}
	}
}

// ── Env vars ───────────────────────────────────────────────────────────────

type envVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func getEnvHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		if d.EnvVars == "" {
			resp.OK(c, []envVar{})
			return
		}
		decrypted, err := crypto.Decrypt(d.EnvVars, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密失败")
			return
		}
		var vars []envVar
		if err := json.Unmarshal([]byte(decrypted), &vars); err != nil {
			resp.InternalError(c, "环境变量数据损坏")
			return
		}
		for i := range vars {
			if vars[i].Secret {
				vars[i].Value = "***"
			}
		}
		resp.OK(c, vars)
	}
}

func putEnvHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		var vars []envVar
		if err := c.ShouldBindJSON(&vars); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}

		// Load original vars to preserve unchanged secret values (value == "***")
		var original []envVar
		if d.EnvVars != "" {
			if dec, err := crypto.Decrypt(d.EnvVars, cfg.Security.AESKey); err == nil {
				_ = json.Unmarshal([]byte(dec), &original)
			}
		}
		origMap := make(map[string]string, len(original))
		for _, v := range original {
			origMap[v.Key] = v.Value
		}
		for i := range vars {
			if vars[i].Secret && vars[i].Value == "***" {
				if orig, ok := origMap[vars[i].Key]; ok {
					vars[i].Value = orig
				}
			}
		}

		b, _ := json.Marshal(vars)
		encrypted, err := crypto.Encrypt(string(b), cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密失败")
			return
		}
		db.Model(&d).Update("env_vars", encrypted)
		resp.OK(c, nil)
	}
}

// ── Webhook info ───────────────────────────────────────────────────────────

func webhookInfoHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		url := fmt.Sprintf("%s://%s/panel/webhooks/%s", scheme, c.Request.Host, d.WebhookSecret)
		resp.OK(c, gin.H{"url": url, "secret": d.WebhookSecret})
	}
}

// ── Upload (SSE) ───────────────────────────────────────────────────────────

func uploadHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Header("Access-Control-Allow-Origin", "*")

		sendUpEvent := func(eventType string, extra map[string]any) {
			m := map[string]any{"type": eventType}
			for k, v := range extra {
				m[k] = v
			}
			payload, _ := json.Marshal(m)
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			c.Writer.Flush()
		}

		fh, err := c.FormFile("file")
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "获取上传文件失败: " + err.Error()})
			return
		}
		f, err := fh.Open()
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "打开文件失败: " + err.Error()})
			return
		}
		defer f.Close()

		var s model.Server
		if err := db.First(&s, d.ServerID).Error; err != nil {
			sendUpEvent("error", map[string]any{"msg": "服务器不存在"})
			return
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
			sendUpEvent("error", map[string]any{"msg": "解密 SSH 凭证失败"})
			return
		}

		sshClient, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "SSH 连接失败: " + err.Error()})
			return
		}

		sftpClient, err := sftp.NewClient(sshClient)
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "SFTP 会话失败: " + err.Error()})
			return
		}
		defer sftpClient.Close()

		workDir := d.WorkDir
		if workDir == "" {
			workDir = "/tmp"
		}
		if err := sftpClient.MkdirAll(workDir); err != nil {
			sendUpEvent("error", map[string]any{"msg": "创建远程目录失败: " + err.Error()})
			return
		}

		filename := filepath.Base(fh.Filename)
		remotePath := workDir + "/" + filename
		total := fh.Size

		sendUpEvent("start", map[string]any{"filename": filename, "total": total})

		dst, err := sftpClient.Create(remotePath)
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "创建远程文件失败: " + err.Error()})
			return
		}
		defer dst.Close()

		buf := make([]byte, 128*1024)
		var transferred int64
		for {
			n, readErr := f.Read(buf)
			if n > 0 {
				if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
					sendUpEvent("error", map[string]any{"msg": "写入远程文件失败: " + writeErr.Error()})
					return
				}
				transferred += int64(n)
				sendUpEvent("progress", map[string]any{"bytes": transferred, "total": total})
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				sendUpEvent("error", map[string]any{"msg": "读取文件失败: " + readErr.Error()})
				return
			}
		}

		sendUpEvent("done", map[string]any{"filename": filename, "path": remotePath})
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func findDeploy(c *gin.Context, db *gorm.DB) (model.Deploy, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return model.Deploy{}, false
	}
	var d model.Deploy
	if err := db.First(&d, id).Error; err != nil {
		resp.NotFound(c, "应用不存在")
		return model.Deploy{}, false
	}
	return d, true
}

func applyDefaults(req *deployReq) {
	if req.Type == "" {
		req.Type = "docker-compose"
	}
	if req.ComposeFile == "" {
		req.ComposeFile = "docker-compose.yml"
	}
}

func generateSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))[:32]
	}
	return hex.EncodeToString(b)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
