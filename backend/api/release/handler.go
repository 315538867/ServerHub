// Package release 实现 M1 Release 三维正交模型的 HTTP 层。
// 路由挂在 /panel/api/v1/services/:id 下:
//
//	/services/:id/releases      ── Release CRUD + Apply + Rollback
//	/services/:id/artifacts     ── Artifact 上传/声明/Probe
//	/services/:id/env-sets      ── EnvVarSet CRUD
//	/services/:id/config-sets   ── ConfigFileSet CRUD
//	/services/:id/deploy-runs   ── 部署执行历史 + 日志
//	/services/:id/settings/auto-rollback ── 自动回滚开关
//
// webhook.go 还在本包里挂 /panel/api/v1/webhooks/:token —— Git provider 推送
// 复用同一个 Release Apply 路径。
//
// 实际 Apply 执行逻辑在 usecase/deploy.go。
package release

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
	"github.com/serverhub/serverhub/usecase"
)

// RegisterRoutes 挂载 Release 模型相关的子路由。
// 调用方需把 r 传成 protected.Group("/services") 。
func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
	// Service 列表 + 单条只读。M2 之前由 apideploy 包提供;现在 service 写路径
	// 已经全部归 Release 链路,这两个只读端点也跟着收到这里——避免再起一个
	// "服务基础信息"的小包,保持 /services/:id 子树语义内聚。
	r.GET("", listServices(db))
	r.GET("/:id", getService(db))

	g := r.Group("/:id")

	// Release
	g.GET("/releases", listReleases(db))
	g.POST("/releases", createRelease(db))
	g.POST("/releases/:rid/apply", applyRelease(db, cfg))

	// Artifact
	g.GET("/artifacts", listArtifacts(db))
	g.POST("/artifacts", createArtifact(db, cfg))
	g.POST("/artifacts/:aid/probe", probeArtifact())

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

// listServices 返回所有 Service。
func listServices(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		services, err := repo.ListAllServices(c.Request.Context(), db)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, services)
	}
}

// getService 返回单条 Service。
func getService(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		s, err := repo.GetServiceByID(c.Request.Context(), db, sid)
		if err != nil {
			resp.NotFound(c, "Service 不存在")
			return
		}
		resp.OK(c, s)
	}
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

func listReleases(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rows, err := repo.ListReleasesByServiceID(c.Request.Context(), db, sid, 0)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

func createRelease(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		var req releaseReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		startSpecJSON, _ := json.Marshal(req.StartSpec)
		rel, err := usecase.CreateRelease(c.Request.Context(), db, usecase.CreateReleaseParams{
			ServiceID:   sid,
			Label:       req.Label,
			ArtifactID:  req.ArtifactID,
			EnvSetID:    req.EnvSetID,
			ConfigSetID: req.ConfigSetID,
			StartSpec:   string(startSpecJSON),
			Note:        req.Note,
		})
		if err != nil {
			if repo.IsNotFound(err) {
				resp.NotFound(c, err.Error())
			} else {
				resp.BadRequest(c, err.Error())
			}
			return
		}
		resp.OK(c, rel)
	}
}

type applyReq struct {
	TriggerSource string `json:"trigger_source"` // 默认 manual
}

func applyRelease(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
			req.TriggerSource = domain.TriggerSourceManual
		}
		run, err := usecase.ApplyRelease(db, cfg, sid, uint(rid), req.TriggerSource, nil)
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

func listArtifacts(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rows, err := repo.ListArtifactsByServiceID(c.Request.Context(), db, sid, 0)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

// createArtifact 支持两种请求格式：
//   - Content-Type: multipart/form-data （含文件）=> provider=upload
//   - Content-Type: application/json （声明 docker/script/http/git）
func createArtifact(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		ctx := c.Request.Context()
		if _, err := repo.GetServiceByID(ctx, db, sid); err != nil {
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
		if req.Provider == domain.ArtifactProviderUpload {
			resp.BadRequest(c, "upload provider requires multipart body")
			return
		}
		art := domain.Artifact{
			ServiceID:  sid,
			Provider:   req.Provider,
			Ref:        req.Ref,
			PullScript: req.PullScript,
		}
		if err := repo.CreateArtifact(ctx, db, &art); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, art)
	}
}

func validProvider(p string) bool {
	switch p {
	case domain.ArtifactProviderUpload,
		domain.ArtifactProviderScript,
		domain.ArtifactProviderGit,
		domain.ArtifactProviderHTTP,
		domain.ArtifactProviderDocker:
		return true
	}
	return false
}

// saveUploadArtifact 把上传文件保存到 data_dir/artifacts/${sid}/${sha256}.${ext}
func saveUploadArtifact(c *gin.Context, db repo.DB, cfg *config.Config, sid uint) (*domain.Artifact, error) {
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
		os.Remove(tmpPath)
	}
	rel := filepath.Join("artifacts", strconv.FormatUint(uint64(sid), 10), hash+ext)
	art := domain.Artifact{
		ServiceID: sid,
		Provider:  domain.ArtifactProviderUpload,
		Ref:       rel,
		Checksum:  hash,
		SizeBytes: size,
	}
	if err := repo.CreateArtifact(c.Request.Context(), db, &art); err != nil {
		return nil, err
	}
	return &art, nil
}

func copyAndHash(r io.Reader, w io.Writer) (string, int64, error) {
	return crypto.CopyAndSHA256(r, w)
}

func probeArtifact() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp.OK(c, gin.H{"probed": false, "msg": "probe not implemented yet"})
	}
}

// ── EnvVarSet handlers ────────────────────────────────────────────────────

type envSetReq struct {
	Label string `json:"label"`
	Vars  []envVar `json:"vars" binding:"required"`
}
type envVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func listEnvSets(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rows, err := repo.ListEnvSetsByServiceID(c.Request.Context(), db, sid)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

func createEnvSet(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
		row := domain.EnvVarSet{ServiceID: sid, Label: req.Label, Content: enc}
		if err := repo.CreateEnvSet(c.Request.Context(), db, &row); err != nil {
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
	Name       string `json:"name"`
	ContentB64 string `json:"content_b64"`
	Mode       int    `json:"mode"`
}

func listConfigSets(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rows, err := repo.ListConfigSetsByServiceID(c.Request.Context(), db, sid)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

func createConfigSet(db repo.DB) gin.HandlerFunc {
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
		row := domain.ConfigFileSet{ServiceID: sid, Label: req.Label, Files: string(raw)}
		if err := repo.CreateConfigSet(c.Request.Context(), db, &row); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, row)
	}
}

// ── DeployRun handlers ────────────────────────────────────────────────────

func listDeployRuns(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := parseServiceID(c)
		if !ok {
			return
		}
		rows, err := repo.ListDeployRunsByServiceID(c.Request.Context(), db, sid, 200)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

func getDeployRun(db repo.DB) gin.HandlerFunc {
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
		row, err := repo.GetDeployRunByIDAndServiceID(c.Request.Context(), db, uint(rid), sid)
		if err != nil {
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

func patchAutoRollback(db repo.DB) gin.HandlerFunc {
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
		if err := repo.UpdateServiceFields(c.Request.Context(), db, sid, map[string]any{
			"auto_rollback_on_fail": req.Enabled,
		}); err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}
