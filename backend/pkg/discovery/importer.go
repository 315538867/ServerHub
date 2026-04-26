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

// Import materializes candidates into Service rows under the given server.
// Existing (server_id, source_kind, source_id) rows are left untouched.
//
// 发现到的环境变量在 M3 起不再写 Service.EnvVars(P-F 起字段已下线),而是落到
// 同 ServiceID 下一条新建的 EnvVarSet,Label="imported"。语义对齐 release.go
// 里的 createEnvSet:JSON 序列化 EnvKV → AES-GCM 加密 → 写 env_var_sets。
// 用户随后在 Releases Tab 创建首个 Release 时即可直接选用这条 set。
//
// EnvVarSet 落库失败不回滚 Service 行,仅记 errors —— Service 已经接管成功,
// 让用户重试 env 集比连服务一起回滚更友好。
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
		var existing model.Service
		q := db.Where("server_id = ? AND source_kind = ? AND source_id = ?",
			serverID, c.Kind, c.SourceID).First(&existing)
		if q.Error == nil {
			res.Skipped++
			continue
		}
		d := model.Service{
			Name:       fallback(c.Name, c.Kind+"-"+c.SourceID),
			ServerID:   serverID,
			Type:       fallback(c.Suggested.Type, model.ServiceTypeNative),
			WorkDir:    c.Suggested.WorkDir,
			ImageName:  c.Suggested.ImageName,
			SourceKind: c.Kind,
			SourceID:   c.SourceID,
			SyncStatus: "synced",
		}
		if err := db.Create(&d).Error; err != nil {
			res.Errors = append(res.Errors, c.Name+": "+err.Error())
			continue
		}
		if len(c.Suggested.Env) > 0 && aesKey != "" {
			if err := createImportedEnvSet(db, d.ID, c.Suggested.Env, aesKey); err != nil {
				res.Errors = append(res.Errors, c.Name+": env-set: "+err.Error())
			}
		}
		res.Imported++
	}
	return res
}

// createImportedEnvSet 把候选 env 列表序列化为 release.go::createEnvSet 同款
// JSON([{key,value,secret}])、AES-GCM 加密后写入 env_var_sets。EnvKV 与
// release.go::envVar 字段名 + tag 完全一致,直接 Marshal 即可二进制等价。
func createImportedEnvSet(db *gorm.DB, serviceID uint, env []EnvKV, aesKey string) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	enc, err := crypto.Encrypt(string(b), aesKey)
	if err != nil {
		return err
	}
	return db.Create(&model.EnvVarSet{
		ServiceID: serviceID,
		Label:     "imported",
		Content:   enc,
	}).Error
}

func fallback(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
