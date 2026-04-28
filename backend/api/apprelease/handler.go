// Package apprelease handler 提供 AppReleaseSet 的 HTTP 入口：
//
//	GET    /apps/:id/release-sets            列表
//	POST   /apps/:id/release-sets            从当前 Service.CurrentReleaseID 快照
//	GET    /apps/:id/release-sets/:rsid      详情
//	POST   /apps/:id/release-sets/:rsid/apply    SSE 流式应用
//	POST   /apps/:id/release-sets/:rsid/rollback SSE 流式回滚
//
// Apply/Rollback 接口返回 text/event-stream，按 service_started → service_line →
// service_done → set_done 顺序推送。客户端断开不中断后端：Apply 在 goroutine 里
// 同步跑完落库，前端可通过 GET 详情读终态。
package apprelease

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sse"
	"github.com/serverhub/serverhub/repo"
	"github.com/serverhub/serverhub/usecase"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/release-sets", listHandler(db))
	r.POST("/:id/release-sets", createHandler(db))
	r.GET("/:id/release-sets/:rsid", getHandler(db))
	r.POST("/:id/release-sets/:rsid/apply", applyHandler(db, cfg))
	r.POST("/:id/release-sets/:rsid/rollback", rollbackHandler(db, cfg))
}

// ─────────────────────────────── list / get / create ─────────────────────────

func listHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, err := parseUint(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 App ID")
			return
		}
		rows, err := repo.ListAppReleaseSetsByAppID(c.Request.Context(), db, appID)
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, rows)
	}
}

func getHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, err := parseUint(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 App ID")
			return
		}
		rsID, err := parseUint(c.Param("rsid"))
		if err != nil {
			resp.BadRequest(c, "无效 Release Set ID")
			return
		}
		set, err := repo.GetAppReleaseSetByIDAndAppID(c.Request.Context(), db, rsID, appID)
		if err != nil {
			resp.NotFound(c, "Release Set 不存在")
			return
		}
		resp.OK(c, set)
	}
}

type createReq struct {
	Label string `json:"label"`
	Note  string `json:"note"`
}

func createHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, err := parseUint(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "无效 App ID")
			return
		}
		var req createReq
		_ = c.ShouldBindJSON(&req) // 全字段可选
		createdBy, _ := c.Get("username")
		createdByStr, _ := createdBy.(string)
		set, err := usecase.CreateAppReleaseSetFromCurrent(c.Request.Context(), db, appID, req.Label, req.Note, createdByStr)
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		resp.OK(c, set)
	}
}

// ─────────────────────────────── Apply / Rollback (SSE) ──────────────────────

func applyHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		runSSE(c, db, func(w *sse.Writer, setID uint, trigger string) error {
			return usecase.AppReleaseApply(c.Request.Context(), db, cfg, setID, trigger,
				func(name string, data any) {
					if w != nil && !w.Closed() {
						_ = w.Event(name, data)
					}
				})
		})
	}
}

func rollbackHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		runSSE(c, db, func(w *sse.Writer, setID uint, trigger string) error {
			return usecase.AppReleaseRollback(c.Request.Context(), db, cfg, setID, trigger,
				func(name string, data any) {
					if w != nil && !w.Closed() {
						_ = w.Event(name, data)
					}
				})
		})
	}
}

// runSSE 抽取 Apply/Rollback 共用的 SSE 启动流程：解析 ID → 校验归属 → 起 SSE 头
// → 跑业务函数 → 发 done。
func runSSE(c *gin.Context, db *gorm.DB,
	fn func(*sse.Writer, uint, string) error,
) {
	appID, err := parseUint(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "无效 App ID")
		return
	}
	rsID, err := parseUint(c.Param("rsid"))
	if err != nil {
		resp.BadRequest(c, "无效 Release Set ID")
		return
	}
	if _, err := repo.GetAppReleaseSetByIDAndAppID(c.Request.Context(), db, rsID, appID); err != nil {
		resp.NotFound(c, "Release Set 不存在")
		return
	}

	w := sse.New(c)
	if w == nil {
		resp.InternalError(c, "响应不支持流式输出")
		return
	}

	trigger := "manual"
	if src := c.Query("trigger"); src != "" {
		trigger = src
	}

	if err := fn(w, rsID, trigger); err != nil {
		data := map[string]any{"error": err.Error()}
		if errors.Is(err, usecase.ErrAppReleaseAlreadyApplying) {
			data["code"] = "already_applying"
		}
		_ = w.Event("error", data)
	}
	w.Done()
}

// ─────────────────────────────── helpers ─────────────────────────────────────

func parseUint(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
