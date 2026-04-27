// Package usecase: discovery.go 替代 v1 pkg/discovery.Scan + Import + Fingerprint。
// 与 R4 source.Default 注册表配合,scanner 实现下沉到 adapters/source/<kind>。
package usecase

import (
	"context"
	"encoding/json"

	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/gorm"
)

// DiscoveryResult 是发现接口的最终响应。按 kind 分桶给前端展示用。
// Errors 收集每个 adapter 自己抛出的非致命错误(例如 docker 未安装),不阻断其他 adapter。
type DiscoveryResult struct {
	Docker  []source.Candidate `json:"docker"`
	Compose []source.Candidate `json:"compose"`
	Systemd []source.Candidate `json:"systemd"`
	Nginx   []source.Candidate `json:"nginx"`
	Errors  []string           `json:"errors,omitempty"`
}

// DiscoverServer 在指定 server 上跑所有(或 kinds 过滤后的)source.Scanner。
// 每个候选回填 Fingerprint 与 AlreadyManaged(同 server 已存在 source_fingerprint 命中)。
//
// kinds 为空 → 跑全集; 否则只跑 kinds ∩ source.Default.Kinds()。
func DiscoverServer(ctx context.Context, db *gorm.DB, r infra.Runner,
	serverID uint, kinds []string) DiscoveryResult {

	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	all := len(want) == 0

	var existing []string
	db.Model(&model.Service{}).
		Where("server_id = ? AND source_fingerprint != ''", serverID).
		Pluck("source_fingerprint", &existing)
	known := make(map[string]struct{}, len(existing))
	for _, fp := range existing {
		known[fp] = struct{}{}
	}

	out := DiscoveryResult{}
	for _, sc := range source.Default.All() {
		kind := sc.Kind()
		if !all && !want[kind] {
			continue
		}
		cands, err := sc.Discover(ctx, r)
		if err != nil {
			out.Errors = append(out.Errors, kind+": "+err.Error())
			continue
		}
		for i := range cands {
			fp := sc.Fingerprint(cands[i])
			cands[i].Kind = kind
			if _, hit := known[fp]; hit {
				cands[i].AlreadyManaged = true
			}
			// 把 fingerprint 暂存到 Raw,前端需要用来灰化按钮。
			if cands[i].Raw == nil {
				cands[i].Raw = map[string]string{}
			}
			cands[i].Raw["fingerprint"] = fp
		}
		switch kind {
		case "docker":
			out.Docker = cands
		case "compose":
			out.Compose = cands
		case "systemd":
			out.Systemd = cands
		case "nginx":
			out.Nginx = cands
		}
	}
	return out
}

// ImportResult 是被动导入(不接管,只登记)的统计。
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportCandidates 在不动远端的前提下登记候选为 Service 行(SyncStatus=synced)。
// 已存在 (server_id, source_kind, source_id) 视为幂等命中,Skipped++。
//
// EnvVars+EnvSecrets 折叠成 JSON([{key,value,secret}]) → AES-GCM → env_var_sets
// 一行,Label="imported"。env-set 写失败不回滚 Service,仅入 Errors —— 与 v1
// pkg/discovery.Import 行为一致(运维可在 UI 重试 env)。
func ImportCandidates(db *gorm.DB, serverID uint, cands []source.Candidate, aesKey string) ImportResult {
	var res ImportResult
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
		svcType := c.Suggested.Type
		if svcType == "" {
			svcType = model.ServiceTypeNative
		}
		name := c.Name
		if name == "" {
			name = c.Kind + "-" + c.SourceID
		}
		d := model.Service{
			Name:       name,
			ServerID:   serverID,
			Type:       svcType,
			WorkDir:    c.Suggested.Workdir,
			SourceKind: c.Kind,
			SourceID:   c.SourceID,
			SyncStatus: "synced",
		}
		if err := db.Create(&d).Error; err != nil {
			res.Errors = append(res.Errors, c.Name+": "+err.Error())
			continue
		}
		if envJSON, ok := encodeImportedEnv(c); ok && aesKey != "" {
			enc, err := crypto.Encrypt(envJSON, aesKey)
			if err != nil {
				res.Errors = append(res.Errors, c.Name+": env-set encrypt: "+err.Error())
			} else if err := db.Create(&model.EnvVarSet{
				ServiceID: d.ID,
				Label:     "imported",
				Content:   enc,
			}).Error; err != nil {
				res.Errors = append(res.Errors, c.Name+": env-set: "+err.Error())
			}
		}
		res.Imported++
	}
	return res
}

// encodeImportedEnv 把 SuggestedFields.EnvVars + EnvSecrets 折叠为
// JSON 数组 [{"key":..,"value":..,"secret":..}],与 release.go::createEnvSet
// 字节兼容。无 env 返回 (_,false)。
func encodeImportedEnv(c source.Candidate) (string, bool) {
	if len(c.Suggested.EnvVars) == 0 {
		return "", false
	}
	type envKV struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Secret bool   `json:"secret,omitempty"`
	}
	out := make([]envKV, 0, len(c.Suggested.EnvVars))
	// 保持稳定顺序:按 key 字典序。
	keys := make([]string, 0, len(c.Suggested.EnvVars))
	for k := range c.Suggested.EnvVars {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		out = append(out, envKV{
			Key:    k,
			Value:  c.Suggested.EnvVars[k],
			Secret: c.Suggested.EnvSecrets[k],
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// sortStrings 内置 sort.Strings 的最小副本,避免在导入路径再加 sort 依赖。
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
