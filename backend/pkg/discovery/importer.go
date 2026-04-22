package discovery

import (
	"encoding/json"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/gorm"
)

// ImportResult summarises a batch import.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"` // already existed (idempotent)
	Errors   []string `json:"errors,omitempty"`
}

// Import materializes candidates into Deploy rows under the given server.
// Existing (server_id, source_kind, source_id) rows are left untouched.
// aesKey is used to encrypt any discovered env vars into Deploy.EnvVars so
// the imported service starts with the same configuration its source had.
func Import(db *gorm.DB, serverID uint, cands []Candidate, aesKey string) ImportResult {
	var res ImportResult
	if len(cands) == 0 {
		return res
	}
	for _, c := range cands {
		if c.Kind == "" || c.SourceID == "" {
			res.Errors = append(res.Errors, "candidate missing kind/source_id: "+c.Name)
			continue
		}
		var existing model.Deploy
		q := db.Where("server_id = ? AND source_kind = ? AND source_id = ?",
			serverID, c.Kind, c.SourceID).First(&existing)
		if q.Error == nil {
			res.Skipped++
			continue
		}
		d := model.Deploy{
			Name:        fallback(c.Name, c.Kind+"-"+c.SourceID),
			ServerID:    serverID,
			Type:        fallback(c.Suggested.Type, "native"),
			WorkDir:     c.Suggested.WorkDir,
			ComposeFile: c.Suggested.ComposeFile,
			StartCmd:    c.Suggested.StartCmd,
			ImageName:   c.Suggested.ImageName,
			Runtime:     c.Suggested.Runtime,
			SourceKind:  c.Kind,
			SourceID:    c.SourceID,
			SyncStatus:  "synced",
			LastStatus:  "success",
		}
		if enc, err := encryptEnv(c.Suggested.Env, aesKey); err != nil {
			res.Errors = append(res.Errors, c.Name+": env encrypt: "+err.Error())
			continue
		} else {
			d.EnvVars = enc
		}
		if err := db.Create(&d).Error; err != nil {
			res.Errors = append(res.Errors, c.Name+": "+err.Error())
			continue
		}
		res.Imported++
	}
	return res
}

// encryptEnv serialises the discovered env list to the same JSON shape
// `getEnvHandler` expects ([{key,value,secret}]) and AES-encrypts it. Empty
// list → empty string (no decryption attempt needed at read time).
func encryptEnv(env []EnvKV, aesKey string) (string, error) {
	if len(env) == 0 || aesKey == "" {
		return "", nil
	}
	b, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return crypto.Encrypt(string(b), aesKey)
}

func fallback(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
