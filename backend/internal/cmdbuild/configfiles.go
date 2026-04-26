package cmdbuild

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// BuildConfigFilesPart 渲染 ConfigFileSet 为 shell 命令片段(每个文件 1-2 行命令)。
// configSetID 为 nil / 记录不存在 / Files 字段为空时返回 nil。
func BuildConfigFilesPart(db *gorm.DB, configSetID *uint) ([]string, error) {
	if configSetID == nil {
		return nil, nil
	}
	var cs model.ConfigFileSet
	if err := db.First(&cs, *configSetID).Error; err != nil || cs.Files == "" {
		return nil, nil
	}
	var files []struct {
		Name       string `json:"name"`
		ContentB64 string `json:"content_b64"`
		Mode       int    `json:"mode"`
	}
	if err := json.Unmarshal([]byte(cs.Files), &files); err != nil {
		return nil, nil
	}
	var parts []string
	for _, f := range files {
		content := f.ContentB64
		if content == "" {
			content = base64.StdEncoding.EncodeToString(nil)
		}
		parts = append(parts, fmt.Sprintf(
			"mkdir -p %s && echo %s | base64 -d > %s",
			ShellQuote(DirOf(f.Name)),
			ShellQuote(content),
			ShellQuote(f.Name),
		))
		if f.Mode > 0 {
			parts = append(parts, fmt.Sprintf("chmod %o %s", f.Mode, ShellQuote(f.Name)))
		}
	}
	return parts, nil
}
