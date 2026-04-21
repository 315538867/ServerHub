package auditq

import (
	"encoding/json"

	"github.com/serverhub/serverhub/model"
)

// Security records a security-relevant event (failed login, account lockout,
// host-key mismatch, etc.) into the audit log. It is a thin wrapper around
// Submit that synthesises the AuditLog row so call sites don't need to know
// the schema.
//
// path must be a stable identifier like "security:login_failed" — callers
// rely on it for filtering in the dashboard.
//
// detail is marshalled as JSON; pass nil if there is nothing to record.
func Security(username, ip, path string, status int, detail map[string]any) {
	if Default == nil {
		return
	}
	body := ""
	if len(detail) > 0 {
		if b, err := json.Marshal(detail); err == nil {
			body = string(b)
		}
	}
	if username == "" {
		username = "anonymous"
	}
	Default.Submit(model.AuditLog{
		Username: username,
		IP:       ip,
		Method:   "SEC",
		Path:     path,
		Body:     body,
		Status:   status,
	})
}
