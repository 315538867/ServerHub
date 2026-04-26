// m2_deploy_to_release.go 实现 M2 数据迁移:把老 deploy_versions 表按时间顺序
// 拆成 Artifact(provider=imported) + EnvVarSet + ConfigFileSet + Release +
// DeployRun。
//
// 调用面有两条:
//   1. 正常启动时由 runner.go 在事务里调 m2Core(tx, aesKey, false),
//      version="010_m2_deploy_to_release"。已 applied 自动跳过。
//   2. CLI `-migrate=m2-dryrun -config ...` 走 RunM2Dryrun,只统计不写库,
//      不进 schema_migrations。
//
// 旧 settings 表里的 `migration.m2.done` 标记由 runner.importLegacyM2Marker
// 翻译为 schema_migrations 行,翻译完后旧标记被删除。
//
// 为什么不用 model.DeployVersion:从 P-D 起 deploy_versions 表与 model 都被
// 删除,但本 migration 仍要在"残留旧库"上跑得动 —— 把列形态冻结成包内私有
// struct legacyDeployVersion,绑定 tx.Table("deploy_versions"),与未来 model
// 演进彻底解耦。
package migration

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"gorm.io/gorm"
)

// migrationM2DoneKey 仅 importLegacyM2Marker 里读一次老标记 —— 新 runner 不
// 再写它。
const migrationM2DoneKey = "migration.m2.done"

// M2Version 是 m2 在 schema_migrations 里的版本号,留在常量是为了让
// database.Init 的注册点和 runner.importLegacyM2Marker 共享同一个字面量。
const M2Version = "010_m2_deploy_to_release"

// legacyDeployVersion 是 deploy_versions 表在 v0.3.7 之前的列形态快照。
// model.DeployVersion 在 P-D 被删,但本 migration 仍要能在残留 deploy_versions
// 表上跑;把列定义钉在这里,与 model 演进解耦。
//
// 字段集与原 model.DeployVersion 一一对应,只是去掉 GORM tag,因为 m2 走
// tx.Table("deploy_versions").Find(&[]legacyDeployVersion) 而非 ORM Model。
type legacyDeployVersion struct {
	ID            uint
	DeployID      uint
	Version       string
	Status        string
	TriggerSource string
	Type          string
	WorkDir       string
	ComposeFile   string
	StartCmd      string
	ImageName     string
	Runtime       string
	ConfigFiles   string
	EnvVars       string
	DeployLogID   uint
	Note          string
	CreatedAt     time.Time
	ArchivedAt    *time.Time
}

// M2Report 聚合一次迁移的写入计数,dry-run/real 共用。
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

// lastM2Report 捕获最近一次 m2 真跑(非 dryrun)的报告,供 CLI `-migrate=m2`
// 在 runner.Run 跑完后取出打印。runner 的 Fn 签名只有 error,无法直接回吐
// 报告;这里用包级 var 兜底。
//
// 不是线程安全的 —— 但 m2 只在启动期/单次 CLI 调用时跑,没有并发场景。
var lastM2Report *M2Report

// LastM2Report 返回最近一次 m2 真跑的报告(若有)。CLI 用,业务层不该读。
func LastM2Report() *M2Report { return lastM2Report }

// RegisterM2 把 M2 注册进 runner。在 main 启动早期、Run 之前调用 —— 因为
// m2 的 Fn 需要解密旧密文,无法在包 init() 静态注册。
func RegisterM2(aesKey string) {
	Register(Migration{
		Version: M2Version,
		Name:    "m2 deploy_versions → release model",
		Fn: func(tx *gorm.DB) error {
			rep, err := m2Core(tx, aesKey, false)
			if err != nil {
				return err
			}
			lastM2Report = rep
			return nil
		},
	})
}

// RunM2Dryrun 是 CLI `-migrate=m2-dryrun` 的入口。只统计、不写库、不进
// schema_migrations。
func RunM2Dryrun(db *gorm.DB, aesKey string) (*M2Report, error) {
	return m2Core(db, aesKey, true)
}

// m2Core 是 M2 迁移的真正实现。dryRun=true 仅统计;dryRun=false 直接在
// 传入的 tx 上写 —— 调用方负责事务边界(runner 包了一层事务)。
//
// 在 deploy_versions 表不存在的全新空库上,直接 no-op 返回空报告 —— P-D 之后
// 新装库压根不会出现这张表。
func m2Core(tx *gorm.DB, aesKey string, dryRun bool) (*M2Report, error) {
	rep := &M2Report{DryRun: dryRun, Skipped: []string{}}

	if !tx.Migrator().HasTable("deploy_versions") {
		return rep, nil
	}

	var dvs []legacyDeployVersion
	if err := tx.Table("deploy_versions").
		Order("deploy_id asc, created_at asc").Find(&dvs).Error; err != nil {
		return nil, fmt.Errorf("load deploy_versions: %w", err)
	}
	rep.DeployVersionsSeen = len(dvs)
	if len(dvs) == 0 {
		return rep, nil
	}

	type migrated struct {
		releaseID uint
		createdAt time.Time
	}
	latestByService := map[uint]migrated{}
	serviceSeen := map[uint]struct{}{}

	for _, dv := range dvs {
		serviceSeen[dv.DeployID] = struct{}{}

		// ── Artifact:provider=imported,ref=version(占位,不可再部署)
		art := model.Artifact{
			ServiceID: dv.DeployID,
			Provider:  model.ArtifactProviderImported,
			Ref:       nonEmpty(dv.Version, fmt.Sprintf("legacy-dv-%d", dv.ID)),
			CreatedAt: dv.CreatedAt,
		}
		if !dryRun {
			if err := tx.Create(&art).Error; err != nil {
				return nil, err
			}
		}
		rep.ArtifactsCreated++

		// ── EnvVarSet:仅当 dv.EnvVars 非空且能解密
		var envSetID *uint
		if dv.EnvVars != "" {
			if _, err := crypto.Decrypt(dv.EnvVars, aesKey); err != nil {
				rep.Skipped = append(rep.Skipped,
					fmt.Sprintf("dv#%d env_vars decrypt failed: %v", dv.ID, err))
			} else {
				es := model.EnvVarSet{
					ServiceID: dv.DeployID,
					Label:     "imported-v" + art.Ref,
					Content:   dv.EnvVars,
					CreatedAt: dv.CreatedAt,
				}
				if !dryRun {
					if err := tx.Create(&es).Error; err != nil {
						return nil, err
					}
					envSetID = &es.ID
				} else {
					tmp := uint(0)
					envSetID = &tmp
				}
				rep.EnvSetsCreated++
			}
		}

		// ── ConfigFileSet:仅当 dv.ConfigFiles 是合法 JSON 数组
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
					return nil, err
				}
				cfgSetID = &cs.ID
			} else {
				tmp := uint(0)
				cfgSetID = &tmp
			}
			rep.ConfigSetsCreated++
		}

		// ── StartSpec:按 dv.Type 还原最小结构
		startSpec, _ := json.Marshal(startSpecFromDV(dv))

		// ── Release:导入态先建 archived,最新一条在 T42 改 active
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
				return nil, err
			}
		}
		rep.ReleasesCreated++

		// ── DeployRun:迁移一条成功 run 做历史留痕
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
				return nil, err
			}
		}
		rep.DeployRunsCreated++

		prev, ok := latestByService[dv.DeployID]
		if !ok || dv.CreatedAt.After(prev.createdAt) {
			latestByService[dv.DeployID] = migrated{releaseID: rel.ID, createdAt: dv.CreatedAt}
		}
	}
	rep.ServicesTouched = len(serviceSeen)

	// ── T42:把 service.current_release_id 指向最新 Release,该 Release 置 active
	for sid, m := range latestByService {
		if dryRun {
			rep.CurrentReleaseSet++
			continue
		}
		if err := tx.Model(&model.Service{}).
			Where("id = ?", sid).
			Update("current_release_id", m.releaseID).Error; err != nil {
			return nil, err
		}
		if err := tx.Model(&model.Release{}).
			Where("id = ?", m.releaseID).
			Update("status", model.ReleaseStatusActive).Error; err != nil {
			return nil, err
		}
		rep.CurrentReleaseSet++
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

// startSpecFromDV 把 legacyDeployVersion 的启动相关字段还原为对应 Service.Type
// 的 StartSpec。导入后可立即被 buildStartPart 消费。
func startSpecFromDV(dv legacyDeployVersion) map[string]any {
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
