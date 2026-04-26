// migrations.go 注册 v0.3.7 之前散落在 database.Init 里的所有"启动期一次性
// 补丁"。每条 migration 与原补丁一一对应,语义零变化 —— 只是搬到 runner 表
// 达"版本化、单次执行、有审计"。
//
// version 编号约定:三位 zero-padded 数字 + 下划线 + snake_case 名,数字段
// 单调递增。M2 数据迁移单独占 010,给 schema 补丁段(001-009)留足空间。
package migration

import (
	"fmt"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"gorm.io/gorm"
)

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
	var rows []model.Service
	if err := tx.Where("source_fingerprint = '' AND source_kind != ''").Find(&rows).Error; err != nil {
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
		if err := tx.Model(&model.Service{}).Where("id = ?", s.ID).
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
