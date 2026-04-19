package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
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

		// cap body to 1 KB to prevent audit log bloat
		body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1024))
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
		db.Create(&log)
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
	return string(out)
}
