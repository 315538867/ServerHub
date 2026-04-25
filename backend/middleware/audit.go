package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/auditq"
	"gorm.io/gorm"
)

var sensitiveKeys = regexp.MustCompile(`(?i)(password|secret|key|token|pass|private)`)

func Audit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			c.Next()
			return
		}

		start := time.Now()

		// Read the full body so downstream handlers still see it. The earlier
		// implementation used io.LimitReader and wrote only the first 1 KB
		// back, which truncated payloads larger than 1 KB (e.g. the
		// discover/takeover candidate) and surfaced as "unexpected EOF"
		// during ShouldBindJSON. We now sanitize the full body and truncate
		// only the audit log entry.
		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		c.Next()

		claims := GetClaims(c)
		var userID *uint
		username := "anonymous"
		if claims != nil {
			userID = &claims.UserID
			username = claims.Username
		}

		log := model.AuditLog{
			UserID:     userID,
			Username:   username,
			IP:         c.ClientIP(),
			Method:     method,
			Path:       c.Request.URL.Path,
			Body:       sanitizeBody(body),
			Status:     c.Writer.Status(),
			DurationMS: int(time.Since(start).Milliseconds()),
		}
		if auditq.Default != nil {
			auditq.Default.Submit(log)
		} else {
			db.Create(&log)
		}
	}
}

func sanitizeBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return "" // non-JSON body: skip logging
	}
	for k := range m {
		if sensitiveKeys.MatchString(k) {
			m[k] = "***"
		}
	}
	out, _ := json.Marshal(m)
	// Cap the stored audit string to 1 KB to prevent table bloat. Sanitizing
	// before truncating ensures we don't slice through a multibyte rune or a
	// JSON token in a way that hides the redaction.
	if len(out) > 1024 {
		return string(out[:1024])
	}
	return string(out)
}
