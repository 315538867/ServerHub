// Package logsearch provides one-shot remote log search across supported sources
// (docker containers, systemd journals, nginx access/error logs). It uses the
// standard sshpool to run a single grep-based pipeline and returns matched
// lines. Server-side concurrency is bounded by a semaphore; over-limit requests
// receive HTTP 429.
package logsearch

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

var searchSem = semaphore.NewWeighted(8)

// allowed "since" window values (docker/journalctl only).
var sinceAllowed = map[string]string{
	"30m": "30 minutes ago",
	"1h":  "1 hour ago",
	"2h":  "2 hours ago",
	"6h":  "6 hours ago",
	"1d":  "1 day ago",
	"2d":  "2 days ago",
	"7d":  "7 days ago",
}

var (
	reContainer = regexp.MustCompile(`^[a-zA-Z0-9_.\-/]{1,128}$`)
	reService   = regexp.MustCompile(`^[a-zA-Z0-9@_.\-]{1,128}$`)
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.POST("/:id/logs/search", searchHandler(db, cfg))
}

type searchReq struct {
	Source        string `json:"source"         binding:"required"`
	Target        string `json:"target"`
	Query         string `json:"query"          binding:"required"`
	Regex         bool   `json:"regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	Since         string `json:"since"`
	Context       int    `json:"context"`
	Limit         int    `json:"limit"`
}

type lineItem struct {
	Raw string `json:"raw"`
}

func sq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func getRunner(c *gin.Context, db *gorm.DB, cfg *config.Config) (runner.Runner, bool) {
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
	rn, err := runner.For(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, false
	}
	return rn, true
}

func searchHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req searchReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if req.Limit <= 0 || req.Limit > 2000 {
			req.Limit = 500
		}
		if req.Context < 0 {
			req.Context = 0
		}
		if req.Context > 10 {
			req.Context = 10
		}

		// tryAcquire — fail fast with 429 when busy
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		if !searchSem.TryAcquire(1) {
			resp.Fail(c, http.StatusTooManyRequests, 4290, "搜索并发已满，请稍后重试")
			return
		}
		defer searchSem.Release(1)
		_ = ctx

		cmd, err := buildCmd(&req)
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		client, ok := getRunner(c, db, cfg)
		if !ok {
			return
		}
		out, runErr := client.Run(cmd)
		// grep 无命中返回非零，这里不把它视为错误
		lines := splitLines(out, req.Limit*(req.Context*2+1)+req.Limit)
		truncated := false
		if len(lines) > req.Limit*(req.Context*2+1) {
			lines = lines[:req.Limit*(req.Context*2+1)]
			truncated = true
		}
		items := make([]lineItem, len(lines))
		for i, l := range lines {
			items[i] = lineItem{Raw: l}
		}
		if runErr != nil && len(items) == 0 {
			// 区分"无匹配"与"真错误"：仅在输出为空时回传 stderr
			resp.OK(c, gin.H{"lines": items, "truncated": truncated, "error": strings.TrimSpace(out)})
			return
		}
		resp.OK(c, gin.H{"lines": items, "truncated": truncated})
	}
}

func buildCmd(r *searchReq) (string, error) {
	// grep options
	grepFlags := []string{"-h", fmt.Sprintf("-m %d", r.Limit)}
	if r.Regex {
		grepFlags = append(grepFlags, "-E")
	} else {
		grepFlags = append(grepFlags, "-F")
	}
	if !r.CaseSensitive {
		grepFlags = append(grepFlags, "-i")
	}
	if r.Context > 0 {
		grepFlags = append(grepFlags, fmt.Sprintf("-B %d -A %d", r.Context, r.Context))
	}
	grep := "grep " + strings.Join(grepFlags, " ") + " -- " + sq(r.Query)

	switch r.Source {
	case "docker":
		if !reContainer.MatchString(r.Target) {
			return "", fmt.Errorf("容器 ID 格式无效")
		}
		since := ""
		if r.Since != "" {
			v, ok := sinceAllowed[r.Since]
			if !ok {
				return "", fmt.Errorf("since 取值无效")
			}
			// docker logs --since 接受 "1h" "30m"
			_ = v
			since = "--since=" + sq(r.Since)
		}
		return fmt.Sprintf("docker logs %s %s 2>&1 | %s", since, sq(r.Target), grep), nil

	case "journalctl":
		if !reService.MatchString(r.Target) {
			return "", fmt.Errorf("服务名格式无效")
		}
		since := ""
		if r.Since != "" {
			v, ok := sinceAllowed[r.Since]
			if !ok {
				return "", fmt.Errorf("since 取值无效")
			}
			since = "--since=" + sq(v)
		}
		return fmt.Sprintf("journalctl -u %s %s --no-pager 2>&1 | %s",
			sq(r.Target), since, grep), nil

	case "nginx-access":
		return fmt.Sprintf("sudo -n %s /var/log/nginx/access.log* 2>&1 || %s /var/log/nginx/access.log 2>&1",
			grep, grep), nil

	case "nginx-error":
		return fmt.Sprintf("sudo -n %s /var/log/nginx/error.log* 2>&1 || %s /var/log/nginx/error.log 2>&1",
			grep, grep), nil

	default:
		return "", fmt.Errorf("不支持的日志源: %s", r.Source)
	}
}

func splitLines(s string, max int) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	// trim trailing empty line from final \n
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > max {
		parts = parts[:max]
	}
	return parts
}
