// migrations.go 注册 v0.3.7 之前散落在 database.Init 里的所有"启动期一次性
// 补丁"。每条 migration 与原补丁一一对应,语义零变化 —— 只是搬到 runner 表
// 达"版本化、单次执行、有审计"。
//
// version 编号约定:三位 zero-padded 数字 + 下划线 + snake_case 名,数字段
// 单调递增。M2 数据迁移单独占 010,给 schema 补丁段(001-009)留足空间。
package migration

import (
	"fmt"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"gorm.io/gorm"
)

// legacyServiceForFingerprint003 冻结 003 跑 backfill 时所见的 services 列形态。
//
// 为什么不用 model.Service：fingerprint 算法的输入字段(ImageName/WorkDir/
// ComposeFile)在 P-D 之后会从 model.Service 上消失,但 003 这条 migration 的
// 历史语义必须永远成立 —— 不论运行哪个版本的二进制,只要库里还有未 backfill
// 的旧行,算出来的 SHA1 必须与首次 backfill 时一致,否则"已接管"标记跨升级
// 失效。把列定义钉成包内私有 struct + tx.Table("services") 即可与 model 演进
// 解耦。
//
// 字段集只取算法实际读的:ID/SourceKind/SourceID 用于 query+update,
// ImageName/WorkDir/ComposeFile 喂给 discovery.Fingerprint。
type legacyServiceForFingerprint003 struct {
	ID                uint
	SourceKind        string
	SourceID          string
	SourceFingerprint string
	ImageName         string
	WorkDir           string
	ComposeFile       string
}

func init() {
	Register(Migration{
		Version: "001_rename_deploys_to_services",
		Name:    "rename legacy deploys table to services",
		Fn:      migrateRenameDeploysToServices,
	})
	Register(Migration{
		Version: "002_drop_setup_states",
		Name:    "drop deprecated setup_states table",
		Fn:      migrateDropSetupStates,
	})
	Register(Migration{
		Version: "003_backfill_source_fingerprint",
		Name:    "backfill Service.SourceFingerprint for legacy rows",
		Fn:      migrateBackfillFingerprints,
	})
	Register(Migration{
		Version: "004_backfill_application_run_server_id",
		Name:    "backfill Application.RunServerID from ServerID",
		Fn:      migrateBackfillRunServerID,
	})
	Register(Migration{
		Version: "020_drop_deploy_versions_and_legacy_service_cols",
		Name:    "drop deploy_versions table + services.{compose_file,start_cmd,runtime,config_files}",
		Fn:      migrateDropLegacyDeployTables,
	})
	Register(Migration{
		Version: "021_drop_service_version_cols",
		Name:    "drop services.{desired_version,actual_version}",
		Fn:      migrateDropServiceVersionCols,
	})
	Register(Migration{
		Version: "022_drop_service_env_vars_col",
		Name:    "drop services.env_vars",
		Fn:      migrateDropServiceEnvVarsCol,
	})
}

// migrateRenameDeploysToServices 把 v0.2 之前的 deploys 表改名 services。
// Service 模型声明 TableName()="services",升级时若旧表还在,AutoMigrate 会
// 静默建空 services 表与旧 deploys 并存,旧数据被孤立。
//
// 难点:本 migration 必须发生在 AutoMigrate 之前 —— 但 runner 调用顺序是
// AutoMigrate→Run。所以这条规则交给 database.Init 在 AutoMigrate 前面单独
// 处理(它本来就在那里),migration 注册仅做"事后审计"用:把这条记录写到
// schema_migrations 让运维可见。Fn 这里再 HasTable 检查一次,确保即使
// init 顺序未来调换也安全。
func migrateRenameDeploysToServices(tx *gorm.DB) error {
	if tx.Migrator().HasTable("deploys") && !tx.Migrator().HasTable("services") {
		if err := tx.Migrator().RenameTable("deploys", "services"); err != nil {
			return fmt.Errorf("rename deploys→services: %w", err)
		}
	}
	return nil
}

// migrateDropSetupStates 删 v0.3.7-beta.16 起弃用的 setup_states 表 ——
// 早期 setup 向导会在该表存本机 SSH 临时密钥,新流程已不再生成,残留密文
// 没有清理路径,这里一次性收掉。
func migrateDropSetupStates(tx *gorm.DB) error {
	return tx.Exec("DROP TABLE IF EXISTS setup_states").Error
}

// migrateBackfillFingerprints 给历史 Service 行补 SourceFingerprint。Discovery
// 在 v0.3.5 引入,之前导入的 Service 行 SourceFingerprint 为空,升级后第一
// 次扫描会把"已接管"标记全部丢失。
//
// 重建一个最小 Candidate 喂给 discovery.Fingerprint,只填指纹算法用得到
// 的字段。
func migrateBackfillFingerprints(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("services") {
		return nil
	}
	var rows []legacyServiceForFingerprint003
	if err := tx.Table("services").
		Where("source_fingerprint = '' AND source_kind != ''").
		Find(&rows).Error; err != nil {
		return err
	}
	for _, s := range rows {
		cand := discovery.Candidate{
			Kind:     s.SourceKind,
			SourceID: s.SourceID,
			Suggested: discovery.SuggestedDeploy{
				ImageName:   s.ImageName,
				WorkDir:     s.WorkDir,
				ComposeFile: s.ComposeFile,
			},
		}
		fp := discovery.Fingerprint(cand)
		if err := tx.Table("services").Where("id = ?", s.ID).
			Update("source_fingerprint", fp).Error; err != nil {
			return err
		}
	}
	if len(rows) > 0 {
		fmt.Printf("migration 003: backfilled %d Service.SourceFingerprint rows\n", len(rows))
	}
	return nil
}

// migrateBackfillRunServerID 给 applications.run_server_id=0 的旧行用 server_id
// 回填。RunServerID 字段在 Nginx-P0 引入,旧行该列为零值,需要一次性 reconcile。
//
// 直接走 Exec 而非 Update,绕开 BeforeSave 钩子(钩子假设至少一边非零,但
// 干净库初始化阶段两边都可能是 0)。
func migrateBackfillRunServerID(tx *gorm.DB) error {
	res := tx.Exec("UPDATE applications SET run_server_id = server_id WHERE run_server_id = 0 AND server_id != 0")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		fmt.Printf("migration 004: backfilled %d applications.run_server_id rows\n", res.RowsAffected)
	}
	return nil
}

// migrateDropLegacyDeployTables 收口 P-D:把 deploy_versions 表与 services 上
// 4 列(compose_file/start_cmd/runtime/config_files)从物理 schema 删除。
//
// 时序前提:
//   - m2(010) 必须已 applied 或 deploy_versions 已为空 —— 真实历史快照已经
//     全部折算成 Release/Artifact/EnvVarSet/ConfigFileSet/DeployRun。runner 按
//     版本号升序跑,本 migration 编号 020 在 010 之后,语义上自然排在 m2 后面。
//   - 上层代码已不再读 services.{compose_file,start_cmd,runtime,config_files}
//     与 model.DeployVersion(P-D 已删字段/删 model)。这条 migration 之前的
//     新二进制不会去读这些列,纯做物理收尾。
//
// 可重入:DropTable/DropColumn 都在缺失时 no-op;但 schema_migrations 写过
// 一次后就不会再跑,以下分支主要是为了在残留旧库上幂等。
func migrateDropLegacyDeployTables(tx *gorm.DB) error {
	// 索引在 SQLite 上随表 drop,但保险起见显式删一次,避免新装库被早期版本
	// 残留索引名拌住。
	if err := tx.Exec("DROP INDEX IF EXISTS idx_deploy_ver_deploy_created").Error; err != nil {
		return fmt.Errorf("drop idx_deploy_ver_deploy_created: %w", err)
	}
	if tx.Migrator().HasTable("deploy_versions") {
		if err := tx.Migrator().DropTable("deploy_versions"); err != nil {
			return fmt.Errorf("drop deploy_versions: %w", err)
		}
	}
	if tx.Migrator().HasTable("services") {
		for _, col := range []string{"compose_file", "start_cmd", "runtime", "config_files"} {
			if !tx.Migrator().HasColumn("services", col) {
				continue
			}
			if err := tx.Migrator().DropColumn("services", col); err != nil {
				return fmt.Errorf("drop services.%s: %w", col, err)
			}
		}
	}
	return nil
}

// migrateDropServiceVersionCols 收口 P-E:把 services 上 desired_version /
// actual_version 两列从物理 schema 删除。
//
// 语义前提:
//   - M3 起版本号由 Release.Label 表达,DeployRun 记录每次部署的目标 release_id;
//     reconciler 也已不读这两列(pkg/scheduler/reconciler.go 注释 "DesiredVersion ↔
//     ActualVersion drift check is gone" 是历史佐证)。
//   - 4 个 takeover 写入点 (compose/docker/static/systemd) 与上层读路径在 P-E
//     同步删除,本 migration 之前的新二进制不会再写这两列,这里只做物理收尾。
//
// 可重入:HasColumn 守卫确保旧库 / 新装库都安全 no-op。
func migrateDropServiceVersionCols(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("services") {
		return nil
	}
	for _, col := range []string{"desired_version", "actual_version"} {
		if !tx.Migrator().HasColumn("services", col) {
			continue
		}
		if err := tx.Migrator().DropColumn("services", col); err != nil {
			return fmt.Errorf("drop services.%s: %w", col, err)
		}
	}
	return nil
}

// legacyServiceEnvVars022 冻结 022 跑 backfill 时所见的 services 列形态。
// 与 003 同款 snapshot pattern:env_vars 列在本 migration 之后会从 model
// 上消失,但读旧值的算法语义必须钉死。
type legacyServiceEnvVars022 struct {
	ID      uint
	EnvVars string
}

// migrateDropServiceEnvVarsCol 收口 P-F:把 services.env_vars 列从物理 schema
// 删除,删之前先把残留数据迁到 env_var_sets。
//
// 语义前提:
//   - M3 起环境变量由 EnvVarSet 表达,Release 通过 EnvVarSetID 引用;
//     /panel/api/v1/services/:id/env 只读端点早在 P-B 已删,model 注释里"保留
//     供该端点展示"是过时描述。
//   - P-F 同期把 importer.go 的 d.EnvVars=enc 改写为新建 EnvVarSet(Label=
//     "imported")。本 migration 之前的新二进制不再写 env_vars。
//
// Backfill:m2(010) 只折算 deploy_versions.env_vars,不碰 services.env_vars。
// 旧库里 importer 直接写在 Service 上的环境变量是孤儿,DropColumn 前必须
// 给每个非空 services.env_vars 建一条 EnvVarSet(Label="legacy-svc-env"),
// 否则旧装机 import 过来的 env 就会丢。Content 直接搬密文(已加密的 JSON),
// 不解密验证 —— 即便密钥早被换掉,密文留在 EnvVarSet 里至少给运维人工恢复
// 留个抓手,比直接 DROP 强。
//
// 可重入:HasColumn 守卫确保旧库 / 新装库都安全 no-op。Backfill 只挑
// services.env_vars != '' 的行,跑过一次后该列还在但被 DropColumn 收尾;
// schema_migrations 写过一次后整条 migration 不会再跑,无需做"已 backfill"
// 标记。
func migrateDropServiceEnvVarsCol(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("services") {
		return nil
	}
	if !tx.Migrator().HasColumn("services", "env_vars") {
		return nil
	}
	var rows []legacyServiceEnvVars022
	if err := tx.Table("services").
		Select("id, env_vars").
		Where("env_vars != ''").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("scan services.env_vars: %w", err)
	}
	now := time.Now()
	for _, r := range rows {
		es := model.EnvVarSet{
			ServiceID: r.ID,
			Label:     "legacy-svc-env",
			Content:   r.EnvVars,
			CreatedAt: now,
		}
		if err := tx.Create(&es).Error; err != nil {
			return fmt.Errorf("backfill env_var_set for service#%d: %w", r.ID, err)
		}
	}
	if len(rows) > 0 {
		fmt.Printf("migration 022: backfilled %d services.env_vars rows into env_var_sets\n", len(rows))
	}
	if err := tx.Migrator().DropColumn("services", "env_vars"); err != nil {
		return fmt.Errorf("drop services.env_vars: %w", err)
	}
	return nil
}
