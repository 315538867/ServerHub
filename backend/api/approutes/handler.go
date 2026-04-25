package approutes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/nginx", getNginxHandler(db))
	r.PUT("/:id/nginx/mode", setModeHandler(db, cfg))
	r.POST("/:id/nginx/routes", addRouteHandler(db))
	r.PUT("/:id/nginx/routes/:rid", updateRouteHandler(db))
	r.DELETE("/:id/nginx/routes/:rid", deleteRouteHandler(db))
	r.POST("/:id/nginx/apply", applyHandler(db, cfg))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sq(s string) string { return safeshell.Quote(s) }

// validateRoute rejects route fields that would let a caller break out of the
// nginx directive or inject a shell terminator when written via base64+tee.
func validateRoute(r *routeReq) error {
	if err := safeshell.NginxValue(r.Path); err != nil {
		return fmt.Errorf("path 非法: %w", err)
	}
	if err := safeshell.NginxValue(r.Upstream); err != nil {
		return fmt.Errorf("upstream 非法: %w", err)
	}
	if r.Extra != "" {
		// Extra is spliced as a raw directive line — disallow newlines and
		// braces so callers cannot open/close nested contexts.
		if strings.ContainsAny(r.Extra, "\n\r{}") {
			return fmt.Errorf("extra 包含非法字符")
		}
	}
	return nil
}

// validateAppName ensures an Application.Name is safe to use as the filename
// for its generated nginx location/site include. Called in applyHandler.
func validateAppName(name string) error {
	return safeshell.ValidName(name, 64)
}

func getApp(c *gin.Context, db *gorm.DB) (*model.Application, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "应用 ID 无效")
		return nil, false
	}
	var app model.Application
	if err := db.First(&app, id).Error; err != nil {
		resp.NotFound(c, "应用不存在")
		return nil, false
	}
	return &app, true
}

func getRunnerForApp(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, *model.Application, bool) {
	app, ok := getApp(c, db)
	if !ok {
		return nil, nil, false
	}
	var s model.Server
	if err := db.First(&s, app.ServerID).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, nil, false
	}
	r, err := runner.For(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "连接失败: "+err.Error())
		return nil, nil, false
	}
	return r, app, true
}

// ── GET /:id/nginx ────────────────────────────────────────────────────────────

type nginxConfig struct {
	ExposeMode string               `json:"expose_mode"`
	Routes     []model.AppNginxRoute `json:"routes"`
}

func getNginxHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var routes []model.AppNginxRoute
		db.Where("app_id = ?", app.ID).Order("sort asc, id asc").Find(&routes)
		if routes == nil {
			routes = []model.AppNginxRoute{}
		}
		resp.OK(c, nginxConfig{ExposeMode: app.ExposeMode, Routes: routes})
	}
}

// ── PUT /:id/nginx/mode ───────────────────────────────────────────────────────

func setModeHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var body struct {
			Mode string `json:"mode" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "mode 字段不能为空")
			return
		}
		if body.Mode != "none" && body.Mode != "path" && body.Mode != "site" {
			resp.BadRequest(c, "mode 取值为 none / path / site")
			return
		}
		if err := db.Model(app).Update("expose_mode", body.Mode).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── POST /:id/nginx/routes ────────────────────────────────────────────────────

type routeReq struct {
	Path     string `json:"path" binding:"required"`
	Upstream string `json:"upstream" binding:"required"`
	Extra    string `json:"extra"`
	Sort     int    `json:"sort"`
}

func addRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateRoute(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		route := model.AppNginxRoute{
			AppID:    app.ID,
			Path:     req.Path,
			Upstream: req.Upstream,
			Extra:    req.Extra,
			Sort:     req.Sort,
		}
		if err := db.Create(&route).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, route)
	}
}

// ── PUT /:id/nginx/routes/:rid ────────────────────────────────────────────────

func updateRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		rid, err := strconv.Atoi(c.Param("rid"))
		if err != nil {
			resp.BadRequest(c, "路由 ID 无效")
			return
		}
		var route model.AppNginxRoute
		if err := db.Where("id = ? AND app_id = ?", rid, app.ID).First(&route).Error; err != nil {
			resp.NotFound(c, "路由不存在")
			return
		}
		var req routeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if err := validateRoute(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		route.Path = req.Path
		route.Upstream = req.Upstream
		route.Extra = req.Extra
		route.Sort = req.Sort
		if err := db.Save(&route).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, route)
	}
}

// ── DELETE /:id/nginx/routes/:rid ─────────────────────────────────────────────

func deleteRouteHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, ok := getApp(c, db)
		if !ok {
			return
		}
		rid, err := strconv.Atoi(c.Param("rid"))
		if err != nil {
			resp.BadRequest(c, "路由 ID 无效")
			return
		}
		if err := db.Where("id = ? AND app_id = ?", rid, app.ID).Delete(&model.AppNginxRoute{}).Error; err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// ── POST /:id/nginx/apply ─────────────────────────────────────────────────────

const appHubSiteName = "serverhub-app-hub"
const appLocationsDir = "/etc/nginx/app-locations"

func applyHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, app, ok := getRunnerForApp(c, db, cfg)
		if !ok {
			return
		}
		if err := validateAppName(app.Name); err != nil {
			resp.BadRequest(c, "应用名包含不能用作文件名的字符: "+err.Error())
			return
		}
		var routes []model.AppNginxRoute
		db.Where("app_id = ?", app.ID).Order("sort asc, id asc").Find(&routes)
		// Re-validate persisted routes — DB rows from before this validation
		// landed could still contain dangerous values.
		for i := range routes {
			rq := routeReq{Path: routes[i].Path, Upstream: routes[i].Upstream, Extra: routes[i].Extra}
			if err := validateRoute(&rq); err != nil {
				resp.BadRequest(c, fmt.Sprintf("路由 #%d 非法: %s", routes[i].ID, err.Error()))
				return
			}
		}

		var output string
		var err error

		switch app.ExposeMode {
		case "none":
			output, err = applyNone(r, app.Name)
		case "path":
			output, err = applyPath(r, app.Name, routes)
		case "site":
			output, err = applySite(r, app.Name, app.Domain, routes)
		default:
			resp.BadRequest(c, "请先设置暴露模式")
			return
		}

		if err != nil {
			resp.InternalError(c, sshpool.HumanizeErr(output)+": "+err.Error())
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(output)})
	}
}

func applyNone(r runner.Runner, name string) (string, error) {
	cmds := []string{
		fmt.Sprintf("sudo -n rm -f %s/%s.conf", appLocationsDir, name),
		fmt.Sprintf("sudo -n rm -f /etc/nginx/sites-enabled/%s-sh", name),
		fmt.Sprintf("sudo -n rm -f /etc/nginx/sites-available/%s-sh.conf", name),
		"sudo -n nginx -s reload 2>&1",
	}
	return r.Run(strings.Join(cmds, " && "))
}

func applyPath(r runner.Runner, name string, routes []model.AppNginxRoute) (string, error) {
	// ensure app-locations dir exists
	if _, err := r.Run("sudo -n mkdir -p " + appLocationsDir); err != nil {
		return "", fmt.Errorf("创建目录失败")
	}

	// generate location blocks
	var sb strings.Builder
	for _, rt := range routes {
		sb.WriteString(fmt.Sprintf("location %s {\n", rt.Path))
		sb.WriteString(fmt.Sprintf("    proxy_pass %s;\n", rt.Upstream))
		sb.WriteString("    proxy_set_header Host $host;\n")
		sb.WriteString("    proxy_set_header X-Real-IP $remote_addr;\n")
		sb.WriteString("    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		sb.WriteString("    proxy_set_header X-Forwarded-Proto $scheme;\n")
		if rt.Extra != "" {
			sb.WriteString("    " + rt.Extra + "\n")
		}
		sb.WriteString("}\n\n")
	}

	locPath := fmt.Sprintf("%s/%s.conf", appLocationsDir, name)
	writeCmd := safeshell.WriteRemoteFile(locPath, sb.String(), true)
	if _, err := r.Run(writeCmd); err != nil {
		return "", fmt.Errorf("写入 location 配置失败")
	}

	// ensure serverhub-app-hub site exists and is enabled
	hubAvail := "/etc/nginx/sites-available/" + appHubSiteName
	hubEnabled := "/etc/nginx/sites-enabled/" + appHubSiteName
	hubConf := fmt.Sprintf(`server {
    listen 80;
    server_name _;

    include %s/*.conf;
}`, appLocationsDir)

	checkCmd := fmt.Sprintf("test -f %s", sq(hubAvail))
	if _, err := r.Run(checkCmd); err != nil {
		// hub doesn't exist, create it
		createCmd := safeshell.WriteRemoteFile(hubAvail, hubConf, true)
		if _, err := r.Run(createCmd); err != nil {
			return "", fmt.Errorf("创建 app-hub 站点失败")
		}
	}
	r.Run(fmt.Sprintf("sudo -n ln -sf %s %s", sq(hubAvail), sq(hubEnabled))) //nolint:errcheck

	return r.Run("sudo -n nginx -t 2>&1 && sudo -n nginx -s reload 2>&1")
}

func applySite(r runner.Runner, name, domain string, routes []model.AppNginxRoute) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("site 模式需要配置域名")
	}
	if err := safeshell.NginxValue(domain); err != nil {
		return "", fmt.Errorf("domain 非法: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n", domain))
	for _, rt := range routes {
		sb.WriteString(fmt.Sprintf("    location %s {\n", rt.Path))
		sb.WriteString(fmt.Sprintf("        proxy_pass %s;\n", rt.Upstream))
		sb.WriteString("        proxy_set_header Host $host;\n")
		sb.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
		if rt.Extra != "" {
			sb.WriteString("        " + rt.Extra + "\n")
		}
		sb.WriteString("    }\n\n")
	}
	sb.WriteString("}\n")

	sitePath := fmt.Sprintf("/etc/nginx/sites-available/%s-sh.conf", name)
	symlinkPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s-sh", name)

	writeCmd := safeshell.WriteRemoteFile(sitePath, sb.String(), true)
	if _, err := r.Run(writeCmd); err != nil {
		return "", fmt.Errorf("写入站点配置失败")
	}
	r.Run(fmt.Sprintf("sudo -n ln -sf %s %s", sq(sitePath), sq(symlinkPath))) //nolint:errcheck

	return r.Run("sudo -n nginx -t 2>&1 && sudo -n nginx -s reload 2>&1")
}
