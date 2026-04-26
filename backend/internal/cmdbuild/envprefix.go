package cmdbuild

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/gorm"
)

// BuildEnvPrefix 把 EnvVarSet 解密成 "export K=V; ..." 前缀。
// envSetID 为 nil 或对应记录不存在时返回空串。
func BuildEnvPrefix(db *gorm.DB, envSetID *uint, aesKey string) (string, error) {
	if envSetID == nil {
		return "", nil
	}
	var es model.EnvVarSet
	if err := db.First(&es, *envSetID).Error; err != nil {
		return "", nil
	}
	if es.Content == "" {
		return "", nil
	}
	dec, err := crypto.Decrypt(es.Content, aesKey)
	if err != nil {
		return "", fmt.Errorf("env set decrypt: %w", err)
	}
	var vars []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(dec), &vars); err != nil {
		return "", nil
	}
	var out []string
	for _, v := range vars {
		if v.Key != "" {
			out = append(out, fmt.Sprintf("export %s=%s", ShellQuote(v.Key), ShellQuote(v.Value)))
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	return strings.Join(out, "; ") + "; ", nil
}
