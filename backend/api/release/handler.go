// Package release 实现 M1 Release 三维正交模型的 HTTP 层。
// 路由挂在 /panel/api/v1/services/:id 下，与 api/deploy 并存（子路径不冲突）：
//
//	/services/:id/releases      ── Release CRUD + Apply + Rollback
//	/services/:id/artifacts     ── Artifact 上传/声明/Probe
//	/services/:id/env-sets      ── EnvVarSet CRUD
//	/services/:id/config-sets   ── ConfigFileSet CRUD
//	/services/:id/deploy-runs   ── 部署执行历史 + 日志
//	/services/:id/settings/auto-rollback ── 自动回滚开关
//
// 实际 Apply 执行逻辑在 pkg/deployer/release_apply.go。
package release

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/deployer"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

// RegisterRoutes 挂载 Release 模型相关的子路由。
// 调用方需把 r 传成 protected.Group("/services") 。
func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	g := r.Group("/:id")

	// Release
	g.GET("/releases", listReleases(db))
	g.POST("/releases", createRelease(db))
	g.POST("/releases/:rid/apply", applyRelease(db, cfg))

	// Artifact
	g.GET("/artifacts", listArtifacts(db))
	g.POST("/artifacts", createArtifact(db, cfg))
	g.POST("/artifacts/:aid/probe", probeArtifact(db, cfg))

	// EnvVarSet
	g.GET("/env-sets", listEnvSets(db))
	g.POST("/env-sets", createEnvSet(db, cfg))

	// ConfigFileSet
	g.GET("/config-sets", listConfigSets(db))
	g.POST("/config-sets", createConfigSet(db))

	// DeployRun
	g.GET("/deploy-runs", listDeployRuns(db))
	g.GET("/deploy-runs/:runid", getDeployRun(db))

	// Settings
	g.PATCH("/settings/auto-rollback", patchAutoRollback(db))
}

// ── helpers ───────────────────────────────────────────────────────────────

func parseServiceID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		resp.BadRequest(c, "invalid service id")
		return 0, false
	}
	return uint(v), true
}

func getService(db *gorm.DB, id uint) (*model.Service, error) {
	var s model.Service
	if err := db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ── Release handlers ──────────────────────────────────────────────────────

type releaseReq struct {
	Label       string `json:"label"`
	ArtifactID  uint   `json:"artifact_id" binding:"required"`
	EnvSetID    *uint  `json:"env_set_id"`
	ConfigSetID *uint  `json:"config_set_id"`
	StartSpec   any    `json:"start_spec"`
	Note        string `json:"note"`
}

func listReleases(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var rows []model.Release
		db.Where("service_id = ?", sid).Order("id desc").Find(&rows)
		resp.OK(c, rows)
	}
}

func createRelease(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		if _, err := getService(db, sid); err != nil {
			resp.NotFound(c, "service not found")
			return
		}
		var req releaseReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		var art model.Artifact
		if err := db.Where("id = ? AND service_id = ?", req.ArtifactID, sid).First(&art).Error; err != nil {
			resp.BadRequest(c, "artifact not found")
			return
		}
		if art.Provider == model.ArtifactProviderImported {
			resp.BadRequest(c, "imported artifact cannot be used for new release; pick a real provider")
			return
		}
		startSpecJSON, _ := json.Marshal(req.StartSpec)
		label := req.Label
		if label == "" {
			label = autoLabel(db, sid)
		}
		rel := model.Release{
			ServiceID:   sid,
			Label:       label,
			ArtifactID:  req.ArtifactID,
			EnvSetID:    req.EnvSetID,
			ConfigSetID: req.ConfigSetID,
			StartSpec:   string(startSpecJSON),
			Note:        req.Note,
			Status:      model.ReleaseStatusDraft,
		}
		if err := db.Create(&rel).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rel)
	}
}

// autoLabel 生成 YYYY-MM-DD-N 兜底标签
func autoLabel(db *gorm.DB, sid uint) string {
	today := time.Now().Format("2006-01-02")
	var n int64
	db.Model(&model.Release{}).Where("service_id = ? AND label LIKE ?", sid, today+"-%").Count(&n)
	return today + "-" + strconv.FormatInt(n+1, 10)
}

type applyReq struct {
	TriggerSource string `json:"trigger_source"` // 默认 manual
}

func applyRelease(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rid, err := strconv.ParseUint(c.Param("rid"), 10, 64)
		if err != nil || rid == 0 {
			resp.BadRequest(c, "invalid release id")
			return
		}
		var req applyReq
		_ = c.ShouldBindJSON(&req)
		if req.TriggerSource == "" {
			req.TriggerSource = model.TriggerSourceManual
		}
		run, err := deployer.ApplyRelease(db, cfg, sid, uint(rid), req.TriggerSource, nil)
		// 部署失败但 run 已建（含 failed/rolled_back 状态 + output）时，仍按业务成功回吐，
		// 便于前端统一读 data.status / data.output，无需区分 HTTP error 与业务 error。
		if err != nil && run == nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, run)
	}
}

// ── Artifact handlers ─────────────────────────────────────────────────────

type artifactReq struct {
	Provider   string `json:"provider" binding:"required"`
	Ref        string `json:"ref"`
	PullScript string `json:"pull_script"`
}

func listArtifacts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var rows []model.Artifact
		db.Where("service_id = ?", sid).Order("id desc").Find(&rows)
		resp.OK(c, rows)
	}
}

// createArtifact 支持两种请求格式：
//   - Content-Type: multipart/form-data （含文件）=> provider=upload
//   - Content-Type: application/json （声明 docker/script/http/git）
func createArtifact(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		if _, err := getService(db, sid); err != nil {
			resp.NotFound(c, "service not found")
			return
		}

		ct := c.ContentType()
		if ct == "multipart/form-data" || ct == "application/octet-stream" ||
			(len(ct) >= 19 && ct[:19] == "multipart/form-data") {
			art, err := saveUploadArtifact(c, db, cfg, sid)
			if err != nil {
				resp.BadRequest(c, err.Error())
				return
			}
			resp.OK(c, art)
			return
		}

		var req artifactReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if !validProvider(req.Provider) {
			resp.BadRequest(c, "unsupported provider: "+req.Provider)
			return
		}
		if req.Provider == model.ArtifactProviderUpload {
			resp.BadRequest(c, "upload provider requires multipart body")
			return
		}
		art := model.Artifact{
			ServiceID:  sid,
			Provider:   req.Provider,
			Ref:        req.Ref,
			PullScript: req.PullScript,
		}
		if err := db.Create(&art).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, art)
	}
}

func validProvider(p string) bool {
	switch p {
	case model.ArtifactProviderUpload,
		model.ArtifactProviderScript,
		model.ArtifactProviderGit,
		model.ArtifactProviderHTTP,
		model.ArtifactProviderDocker:
		return true
	}
	return false
}

// saveUploadArtifact 把上传文件保存到 data_dir/artifacts/${sid}/${sha256}.${ext}
func saveUploadArtifact(c *gin.Context, db *gorm.DB, cfg *config.Config, sid uint) (*model.Artifact, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, errors.New("file field required")
	}
	dir := filepath.Join(cfg.Server.DataDir, "artifacts", strconv.FormatUint(uint64(sid), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// 临时落盘 -> sha256 -> 重命名
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	hash, size, err := copyAndHash(src, tmp)
	tmp.Close()
	if err != nil {
		os.Remove(tmpPath)
		return nil, err
	}
	ext := filepath.Ext(fh.Filename)
	final := filepath.Join(dir, hash+ext)
	if _, err := os.Stat(final); os.IsNotExist(err) {
		if err := os.Rename(tmpPath, final); err != nil {
			return nil, err
		}
	} else {
		// 相同 sha 已存在，去重
		os.Remove(tmpPath)
	}
	rel := filepath.Join("artifacts", strconv.FormatUint(uint64(sid), 10), hash+ext)
	art := model.Artifact{
		ServiceID: sid,
		Provider:  model.ArtifactProviderUpload,
		Ref:       rel,
		Checksum:  hash,
		SizeBytes: size,
	}
	if err := db.Create(&art).Error; err != nil {
		return nil, err
	}
	return &art, nil
}

func copyAndHash(r io.Reader, w io.Writer) (string, int64, error) {
	return crypto.CopyAndSHA256(r, w)
}

func probeArtifact(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// M1: 占位实现；M2 实现 git/http/script 的试拉取
		resp.OK(c, gin.H{"probed": false, "msg": "probe not implemented yet"})
	}
}

// ── EnvVarSet handlers ────────────────────────────────────────────────────

type envSetReq struct {
	Label string            `json:"label"`
	Vars  []envVar          `json:"vars" binding:"required"`
}
type envVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func listEnvSets(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		// 不带 Content（加密体，不出 JSON）
		var rows []model.EnvVarSet
		db.Select("id, service_id, label, created_at").
			Where("service_id = ?", sid).Order("id desc").Find(&rows)
		resp.OK(c, rows)
	}
}

func createEnvSet(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var req envSetReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		raw, _ := json.Marshal(req.Vars)
		enc, err := crypto.Encrypt(string(raw), cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "encrypt: "+err.Error())
			return
		}
		row := model.EnvVarSet{ServiceID: sid, Label: req.Label, Content: enc}
		if err := db.Create(&row).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, row)
	}
}

// ── ConfigFileSet handlers ────────────────────────────────────────────────

type configSetReq struct {
	Label string       `json:"label"`
	Files []configFile `json:"files" binding:"required"`
}
type configFile struct {
	Name        string `json:"name"`
	ContentB64  string `json:"content_b64"`
	Mode        int    `json:"mode"`
}

func listConfigSets(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var rows []model.ConfigFileSet
		db.Where("service_id = ?", sid).Order("id desc").Find(&rows)
		resp.OK(c, rows)
	}
}

func createConfigSet(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var req configSetReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		raw, _ := json.Marshal(req.Files)
		row := model.ConfigFileSet{ServiceID: sid, Label: req.Label, Files: string(raw)}
		if err := db.Create(&row).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, row)
	}
}

// ── DeployRun handlers ────────────────────────────────────────────────────

func listDeployRuns(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var rows []model.DeployRun
		db.Where("service_id = ?", sid).Order("id desc").Limit(200).Find(&rows)
		resp.OK(c, rows)
	}
}

func getDeployRun(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rid, err := strconv.ParseUint(c.Param("runid"), 10, 64)
		if err != nil {
			resp.BadRequest(c, "invalid run id")
			return
		}
		var row model.DeployRun
		if err := db.Where("id = ? AND service_id = ?", rid, sid).First(&row).Error; err != nil {
			resp.NotFound(c, "deploy run not found")
			return
		}
		resp.OK(c, row)
	}
}

// ── Settings ──────────────────────────────────────────────────────────────

type autoRollbackReq struct {
	Enabled bool `json:"enabled"`
}

func patchAutoRollback(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var req autoRollbackReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := db.Model(&model.Service{}).Where("id = ?", sid).
			Update("auto_rollback_on_fail", req.Enabled).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}
