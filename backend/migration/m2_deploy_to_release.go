// Package migration 承载跨版本数据迁移脚本。
//
// M2 数据迁移：把老 deploy_versions 表按时间顺序拆成
// Artifact(provider=imported) + EnvVarSet + ConfigFileSet + Release + DeployRun。
//
// 调用方式：
//
//	serverhub -migrate=m2-dryrun -config ...   只打印报告不写库
//	serverhub -migrate=m2        -config ...   正式执行（带幂等标记）
//
// 幂等保证：执行成功后在 settings 表写入 key=migration.m2.done，再次运行立即返回 skipped。
package migration

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/gorm"
)

const migrationM2DoneKey = "migration.m2.done"

// M2Report 聚合一次迁移的写入计数，dry-run/real 共用。
type M2Report struct {
	DryRun             bool     `json:"dry_run"`
	AlreadyDone        bool     `json:"already_done"`
	DeployVersionsSeen int      `json:"deploy_versions_seen"`
	ServicesTouched    int      `json:"services_touched"`
	ArtifactsCreated   int      `json:"artifacts_created"`
	EnvSetsCreated     int      `json:"env_sets_created"`
	ConfigSetsCreated  int      `json:"config_sets_created"`
	ReleasesCreated    int      `json:"releases_created"`
	DeployRunsCreated  int      `json:"deploy_runs_created"`
	CurrentReleaseSet  int      `json:"current_release_set"`
	Skipped            []string `json:"skipped"`
}

// RunM2 执行 M2 迁移。dryRun=true 只统计，不写库。
// aesKey 用来判定 env_vars 旧密文是否仍能解密（不能就跳过并登记 Skipped）。
func RunM2(db *gorm.DB, aesKey string, dryRun bool) (*M2Report, error) {
	rep := &M2Report{DryRun: dryRun, Skipped: []string{}}

	// 已完成标记：正式 run 不重复；dry-run 允许多次。
	if !dryRun {
		var s model.Setting
		if err := db.Where("key = ?", migrationM2DoneKey).First(&s).Error; err == nil {
			rep.AlreadyDone = true
			return rep, nil
		}
	}

	var dvs []model.DeployVersion
	if err := db.Order("deploy_id asc, created_at asc").Find(&dvs).Error; err != nil {
		return nil, fmt.Errorf("load deploy_versions: %w", err)
	}
	rep.DeployVersionsSeen = len(dvs)
	if len(dvs) == 0 {
		if !dryRun {
			markDone(db, rep)
		}
		return rep, nil
	}

	// 按 service 分组逐条迁移；latestReleaseByService 记录每个 service
	// 时间最晚的那条 Release.ID，用于 T42 回指。
	type migrated struct {
		releaseID uint
		createdAt time.Time
	}
	latestByService := map[uint]migrated{}
	serviceSeen := map[uint]struct{}{}

	tx := db
	if !dryRun {
		tx = db.Begin()
		if tx.Error != nil {
			return nil, tx.Error
		}
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				panic(r)
			}
		}()
	}

	for _, dv := range dvs {
		serviceSeen[dv.DeployID] = struct{}{}

		// ── Artifact：provider=imported，ref=version（占位，不可再部署）
		art := model.Artifact{
			ServiceID: dv.DeployID,
			Provider:  model.ArtifactProviderImported,
			Ref:       nonEmpty(dv.Version, fmt.Sprintf("legacy-dv-%d", dv.ID)),
			CreatedAt: dv.CreatedAt,
		}
		if !dryRun {
			if err := tx.Create(&art).Error; err != nil {
				return rollback(tx, rep, err)
			}
		}
		rep.ArtifactsCreated++

		// ── EnvVarSet：仅当 dv.EnvVars 非空
		var envSetID *uint
		if dv.EnvVars != "" {
			// 校验旧密文仍可解密（否则跳过写 Skipped，避免带着烂数据走）
			if _, err := crypto.Decrypt(dv.EnvVars, aesKey); err != nil {
				rep.Skipped = append(rep.Skipped,
					fmt.Sprintf("dv#%d env_vars decrypt failed: %v", dv.ID, err))
			} else {
				es := model.EnvVarSet{
					ServiceID: dv.DeployID,
					Label:     "imported-v" + art.Ref,
					Content:   dv.EnvVars, // 保持原密文直接复用
					CreatedAt: dv.CreatedAt,
				}
				if !dryRun {
					if err := tx.Create(&es).Error; err != nil {
						return rollback(tx, rep, err)
					}
					envSetID = &es.ID
				} else {
					tmp := uint(0)
					envSetID = &tmp
				}
				rep.EnvSetsCreated++
			}
		}

		// ── ConfigFileSet：仅当 dv.ConfigFiles 是合法 JSON 数组
		var cfgSetID *uint
		if dv.ConfigFiles != "" && looksLikeJSONArray(dv.ConfigFiles) {
			cs := model.ConfigFileSet{
				ServiceID: dv.DeployID,
				Label:     "imported-v" + art.Ref,
				Files:     dv.ConfigFiles,
				CreatedAt: dv.CreatedAt,
			}
			if !dryRun {
				if err := tx.Create(&cs).Error; err != nil {
					return rollback(tx, rep, err)
				}
				cfgSetID = &cs.ID
			} else {
				tmp := uint(0)
				cfgSetID = &tmp
			}
			rep.ConfigSetsCreated++
		}

		// ── StartSpec：按 dv.Type 还原最小结构
		startSpec, _ := json.Marshal(startSpecFromDV(dv))

		// ── Release：导入态一律先建 archived，最新一条在 T42 改 active
		rel := model.Release{
			ServiceID:   dv.DeployID,
			Label:       art.Ref,
			ArtifactID:  art.ID,
			EnvSetID:    envSetID,
			ConfigSetID: cfgSetID,
			StartSpec:   string(startSpec),
			Note:        "imported from deploy_versions#" + fmt.Sprint(dv.ID),
			CreatedBy:   "migration:m2",
			Status:      model.ReleaseStatusArchived,
			CreatedAt:   dv.CreatedAt,
			UpdatedAt:   dv.CreatedAt,
		}
		if !dryRun {
			if err := tx.Create(&rel).Error; err != nil {
				return rollback(tx, rep, err)
			}
		}
		rep.ReleasesCreated++

		// ── DeployRun：迁移一条成功 run 做历史留痕（finished_at = created_at）
		finished := dv.CreatedAt
		run := model.DeployRun{
			ServiceID:     dv.DeployID,
			ReleaseID:     rel.ID,
			Status:        mapDVStatus(dv.Status),
			TriggerSource: nonEmpty(dv.TriggerSource, model.TriggerSourceManual),
			StartedAt:     dv.CreatedAt,
			FinishedAt:    &finished,
			DurationSec:   0,
			Output:        dv.Note,
			CreatedAt:     dv.CreatedAt,
		}
		if !dryRun {
			if err := tx.Create(&run).Error; err != nil {
				return rollback(tx, rep, err)
			}
		}
		rep.DeployRunsCreated++

		// 记录每个 service 的最新 Release
		prev, ok := latestByService[dv.DeployID]
		if !ok || dv.CreatedAt.After(prev.createdAt) {
			latestByService[dv.DeployID] = migrated{releaseID: rel.ID, createdAt: dv.CreatedAt}
		}
	}
	rep.ServicesTouched = len(serviceSeen)

	// ── T42：把 service.current_release_id 指向最新 Release，并把该 Release 置 active
	for sid, m := range latestByService {
		if dryRun {
			rep.CurrentReleaseSet++
			continue
		}
		if err := tx.Model(&model.Service{}).
			Where("id = ?", sid).
			Update("current_release_id", m.releaseID).Error; err != nil {
			return rollback(tx, rep, err)
		}
		if err := tx.Model(&model.Release{}).
			Where("id = ?", m.releaseID).
			Update("status", model.ReleaseStatusActive).Error; err != nil {
			return rollback(tx, rep, err)
		}
		rep.CurrentReleaseSet++
	}

	if !dryRun {
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
		markDone(db, rep)
	}
	return rep, nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func looksLikeJSONArray(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		return c == '['
	}
	return false
}

// startSpecFromDV 把 DeployVersion 的启动相关字段还原为对应 Service.Type 的 StartSpec。
// 导入后可立即被 buildStartPart 消费。
func startSpecFromDV(dv model.DeployVersion) map[string]any {
	switch dv.Type {
	case "docker":
		return map[string]any{"type": "docker", "image": dv.ImageName, "cmd": dv.StartCmd}
	case "docker-compose":
		return map[string]any{"type": "docker-compose", "file_name": nonEmpty(dv.ComposeFile, "docker-compose.yml")}
	case "native":
		return map[string]any{"type": "native", "cmd": dv.StartCmd}
	case "static":
		return map[string]any{"type": "static"}
	}
	// 未知 type：保留 raw，执行器会在 buildStartPart 里报错
	return map[string]any{"type": dv.Type, "cmd": dv.StartCmd, "image": dv.ImageName}
}

// mapDVStatus 把老状态映射到 DeployRun 枚举。
func mapDVStatus(s string) string {
	switch s {
	case "success", "":
		return model.DeployRunStatusSuccess
	case "failed", "fail", "error":
		return model.DeployRunStatusFailed
	}
	return model.DeployRunStatusSuccess
}

func markDone(db *gorm.DB, rep *M2Report) {
	payload, _ := json.Marshal(rep)
	db.Save(&model.Setting{
		Key:       migrationM2DoneKey,
		Value:     string(payload),
		UpdatedAt: time.Now(),
	})
}

// rollback 统一处理事务回滚并返回错误。仅事务对象有效；dry-run 不会走到这里。
func rollback(tx *gorm.DB, rep *M2Report, err error) (*M2Report, error) {
	_ = tx.Rollback()
	return rep, err
}
