// Package migration 承载跨版本数据迁移脚本与版本化 runner。
//
// 设计动机：v0.3.7 之前所有"启动期一次性补丁"都散落在 database.Init() 里直接
// 调 db.Exec / Migrator() / Update —— 每条逻辑都自己造一个"已经跑过"的判定（or
// 干脆每次启动重跑）。M2 数据迁移又自己起了个 -migrate=m2 CLI + settings 表
// 写 done 标记。结果是同一件事(单调一次性变更)有两套表达,且没有任何统一审计
// 入口能告诉运维"这台机器跑过哪些 schema 变更"。
//
// runner.go 把这件事收敛为单一事实来源:
//
//	schema_migrations(version TEXT PK, applied_at DATETIME)
//
// 每条 migration 用 Register(version, name, fn) 静态注册;version 是单调递增
// 的字符串(zero-padded 数字前缀,例如 "001_rename_deploys_to_services"),fn
// 在事务内执行,Run() 按 version 升序跑未 applied 的、写表、提交、继续下一条。
//
// migration 与 GORM AutoMigrate 的分工:
//   - AutoMigrate 负责"加列/加表"等可重入 schema 演进 —— 框架自己幂等。
//   - 本 runner 负责"跨版本一次性数据搬运/表重命名/旧表清理" —— 这些不是
//     AutoMigrate 能表达的,且必须只跑一次。
//
// 调用顺序(database.Init):
//
//	PRAGMA → AutoMigrate → migration.Run(db) → seed*
//
// 即:先让最新 schema 就位,再跑数据搬运。这样 migration 永远在"完整 schema
// 已存在"前提下书写,代码里 db.Model(&model.X{}) 永远成立。
package migration

import (
	"fmt"
	"sort"
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// Migration 描述一条具名版本化变更。fn 在 runner 起的事务里执行 —— 失败回滚,
// 成功后 runner 自己写 schema_migrations 行。fn 不应自己 Begin/Commit。
type Migration struct {
	Version string // e.g. "001_rename_deploys_to_services"
	Name    string // 人类可读说明,会写进 schema_migrations.name 便于排查
	Fn      func(tx *gorm.DB) error
}

// schemaMigration 是 schema_migrations 表的 ORM 模型。仅本包内部使用 ——
// 不放进 model/ 是因为它属于"基础设施表",应用层不该感知。
type schemaMigration struct {
	Version   string    `gorm:"primaryKey;column:version"`
	Name      string    `gorm:"column:name"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

var registry []Migration

// Register 把一条 migration 注册进 runner。在包 init() 里调用,顺序无关 ——
// runner 会在 Run() 时按 version 字符串升序排序。
//
// 重复 version 直接 panic,避免合并冲突时悄悄丢一条。
func Register(m Migration) {
	for _, existing := range registry {
		if existing.Version == m.Version {
			panic(fmt.Sprintf("migration: duplicate version %q", m.Version))
		}
	}
	registry = append(registry, m)
}

// Run 按 version 升序跑所有未 applied 的 migration。每条独立事务。
//
// 错误策略:任何一条失败立即返回,后续 migration 不再尝试 —— 启动失败让运维
// 立刻看到,而不是带着半 applied 的 schema 继续启动业务路由。
func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("migration: ensure schema_migrations table: %w", err)
	}

	if err := importLegacyM2Marker(db); err != nil {
		return fmt.Errorf("migration: import legacy m2 marker: %w", err)
	}

	applied := map[string]struct{}{}
	var rows []schemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return fmt.Errorf("migration: load schema_migrations: %w", err)
	}
	for _, r := range rows {
		applied[r.Version] = struct{}{}
	}

	pending := make([]Migration, 0, len(registry))
	for _, m := range registry {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		pending = append(pending, m)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Version < pending[j].Version })

	for _, m := range pending {
		if err := runOne(db, m); err != nil {
			return fmt.Errorf("migration %s (%s): %w", m.Version, m.Name, err)
		}
		fmt.Printf("migration: %s applied (%s)\n", m.Version, m.Name)
	}
	return nil
}

func runOne(db *gorm.DB, m Migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := m.Fn(tx); err != nil {
			return err
		}
		return tx.Create(&schemaMigration{
			Version:   m.Version,
			Name:      m.Name,
			AppliedAt: time.Now(),
		}).Error
	})
}

// importLegacyM2Marker 把 v0.3.7-beta 之前用 settings 表记录的
// `migration.m2.done` 标记翻译成 schema_migrations 行。M2 是项目第一条
// 跨版本数据迁移,在 runner 出现之前就先落地了 —— 升级时若不补这一步,
// 已经跑过 M2 的库会被新 runner 当成"未 applied",再走一次幂等读后还要
// 多一次空转。这里把旧标记一次性翻译过去,翻译完即删除旧 setting,
// 让 schema_migrations 真正成为唯一事实来源。
func importLegacyM2Marker(db *gorm.DB) error {
	if !db.Migrator().HasTable("settings") {
		return nil
	}
	var s model.Setting
	if err := db.Where("key = ?", migrationM2DoneKey).First(&s).Error; err != nil {
		return nil //nolint:nilerr // record-not-found 是正常路径
	}
	const m2Version = "010_m2_deploy_to_release"
	var existed schemaMigration
	if err := db.Where("version = ?", m2Version).First(&existed).Error; err == nil {
		// 双方都已有,删旧标记即可
		db.Where("key = ?", migrationM2DoneKey).Delete(&model.Setting{})
		return nil
	}
	row := schemaMigration{
		Version:   m2Version,
		Name:      "m2 deploy_versions → release model (imported from legacy settings marker)",
		AppliedAt: s.UpdatedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		return err
	}
	return db.Where("key = ?", migrationM2DoneKey).Delete(&model.Setting{}).Error
}
