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
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/deployer"
	"github.com/serverhub/serverhub/pkg/fsclient"
	"github.com/serverhub/serverhub/pkg/resp"
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
	r.POST("/:id/rollback", rollbackLatestHandler(db, cfg))
	r.GET("/:id/versions", listVersionsHandler(db))
	r.GET("/:id/versions/:vid", getVersionHandler(db))
	r.POST("/:id/versions/:vid/rollback", rollbackToVersionHandler(db, cfg))
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

// ── Rollback & Versions ────────────────────────────────────────────────────

// rollbackLatestHandler rolls back to the most recent historical version
// whose `version` differs from the current actual_version. Streams the
// redeploy output as SSE.
func rollbackLatestHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		target, err := pickPreviousVersion(db, d)
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		streamRollback(c, db, cfg, d, target)
	}
}

func rollbackToVersionHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		vid, err := strconv.Atoi(c.Param("vid"))
		if err != nil {
			resp.BadRequest(c, "版本 ID 格式错误")
			return
		}
		var v model.DeployVersion
		if err := db.Where("id = ? AND deploy_id = ?", vid, d.ID).First(&v).Error; err != nil {
			resp.NotFound(c, "版本不存在")
			return
		}
		streamRollback(c, db, cfg, d, v)
	}
}

func listVersionsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		var versions []model.DeployVersion
		db.Where("deploy_id = ?", d.ID).
			Order("created_at DESC").
			Limit(deployer.MaxVersionsPerDeploy).
			Find(&versions)
		resp.OK(c, versions)
	}
}

func getVersionHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := findDeploy(c, db)
		if !ok {
			return
		}
		vid, err := strconv.Atoi(c.Param("vid"))
		if err != nil {
			resp.BadRequest(c, "版本 ID 格式错误")
			return
		}
		var v model.DeployVersion
		if err := db.Where("id = ? AND deploy_id = ?", vid, d.ID).First(&v).Error; err != nil {
			resp.NotFound(c, "版本不存在")
			return
		}
		resp.OK(c, v)
	}
}

// pickPreviousVersion returns the most recent DeployVersion whose version
// label differs from the deploy's current actual_version.
func pickPreviousVersion(db *gorm.DB, d model.Deploy) (model.DeployVersion, error) {
	var versions []model.DeployVersion
	db.Where("deploy_id = ?", d.ID).Order("created_at DESC").
		Limit(deployer.MaxVersionsPerDeploy).Find(&versions)
	for _, v := range versions {
		if v.Version != "" && v.Version != d.ActualVersion {
			return v, nil
		}
	}
	return model.DeployVersion{}, fmt.Errorf("无可用历史版本")
}

// streamRollback applies a version snapshot to the deploy and triggers a
// redeploy, streaming output as SSE.
func streamRollback(c *gin.Context, db *gorm.DB, cfg *config.Config, d model.Deploy, v model.DeployVersion) {
	updates := map[string]any{
		"type":         v.Type,
		"work_dir":     v.WorkDir,
		"compose_file": v.ComposeFile,
		"start_cmd":    v.StartCmd,
		"image_name":   v.ImageName,
		"runtime":      v.Runtime,
		"config_files": v.ConfigFiles,
		"env_vars":     v.EnvVars,
		"sync_status":  "drifted",
	}
	if v.Version != "" {
		updates["desired_version"] = v.Version
	}
	db.Model(&d).Updates(updates)
	d.Type = v.Type
	d.WorkDir = v.WorkDir
	d.ComposeFile = v.ComposeFile
	d.StartCmd = v.StartCmd
	d.ImageName = v.ImageName
	d.Runtime = v.Runtime
	d.ConfigFiles = v.ConfigFiles
	d.EnvVars = v.EnvVars
	if v.Version != "" {
		d.DesiredVersion = v.Version
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

	result := deployer.Run(db, cfg, d, "rollback", func(line string) {
		sendEvent("output", line)
	})
	if result.Success {
		sendEvent("done", "success")
	} else {
		sendEvent("done", "failed")
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

		fc, err := fsclient.For(&s, cfg)
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "文件客户端获取失败: " + err.Error()})
			return
		}
		defer fc.Close()

		workDir := d.WorkDir
		if workDir == "" {
			workDir = "/tmp"
		}
		if err := fc.MkdirAll(workDir); err != nil {
			sendUpEvent("error", map[string]any{"msg": "创建目录失败: " + err.Error()})
			return
		}

		filename := filepath.Base(fh.Filename)
		remotePath := workDir + "/" + filename
		total := fh.Size

		sendUpEvent("start", map[string]any{"filename": filename, "total": total})

		dst, err := fc.Create(remotePath)
		if err != nil {
			sendUpEvent("error", map[string]any{"msg": "创建文件失败: " + err.Error()})
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
