// release_retention.go 实现 Release + Artifact 保留策略,平移自 v1 pkg/deployer.release_prune.go。
//
// 行为不变:
//  1. 每个 Service 默认保留 MaxReleasesPerService 个 Release(FIFO 按 created_at 淘汰)
//  2. 当前 active Release + service.current_release_id 指向的 Release 永不删
//  3. 删 Release 时连带回收孤儿 Artifact / EnvVarSet / ConfigFileSet 与磁盘文件
//  4. PruneOrphanArtifactFiles 扫 data_dir/artifacts 删除 DB 无引用的物理文件
package usecase

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// MaxReleasesPerService Release 默认保留窗口。可由 settings.release_keep_per_service 覆盖。
const MaxReleasesPerService = 10

// ArtifactsDataDir 由 main 在启动时注入,供 prune 定位 upload 物理文件。
// 空字符串表示不清理磁盘文件(只删 DB 行)。
var ArtifactsDataDir string

func artifactsDataDir() string { return ArtifactsDataDir }

// PruneReleases 对单个 Service 按保留窗口淘汰超限 Release。
// keep<=0 时视为关闭保留(直接返回)。
// 总是保留:当前 active 的 Release、service.current_release_id 指向的 Release。
func PruneReleases(db *gorm.DB, serviceID uint, keep int) {
	if keep <= 0 {
		return
	}

	var svc model.Service
	if err := db.Select("id", "current_release_id").First(&svc, serviceID).Error; err != nil {
		return
	}

	protected := map[uint]struct{}{}
	if svc.CurrentReleaseID != nil {
		protected[*svc.CurrentReleaseID] = struct{}{}
	}
	var activeIDs []uint
	db.Model(&model.Release{}).
		Where("service_id = ? AND status = ?", serviceID, model.ReleaseStatusActive).
		Pluck("id", &activeIDs)
	for _, id := range activeIDs {
		protected[id] = struct{}{}
	}

	var candidates []model.Release
	db.Where("service_id = ?", serviceID).
		Order("created_at DESC").
		Offset(keep).
		Find(&candidates)

	for _, rel := range candidates {
		if _, ok := protected[rel.ID]; ok {
			continue
		}
		deleteRelease(db, rel)
	}
}

// PruneAllReleases 对所有 Service 执行保留策略。每日保留任务入口。
func PruneAllReleases(db *gorm.DB, keep int) {
	if keep <= 0 {
		return
	}
	var ids []uint
	db.Model(&model.Service{}).Pluck("id", &ids)
	for _, id := range ids {
		PruneReleases(db, id, keep)
	}
}

func deleteRelease(db *gorm.DB, rel model.Release) {
	artifactID := rel.ArtifactID
	envSetID := rel.EnvSetID
	cfgSetID := rel.ConfigSetID

	if err := db.Delete(&rel).Error; err != nil {
		log.Printf("[release-prune] delete release#%d: %v", rel.ID, err)
		return
	}

	if artifactID != 0 {
		if count := countReleasesByArtifact(db, artifactID); count == 0 {
			var art model.Artifact
			if err := db.First(&art, artifactID).Error; err == nil {
				_ = db.Delete(&art).Error
				tryRemoveUploadFile(art)
			}
		}
	}
	if envSetID != nil {
		if count := countReleasesByEnvSet(db, *envSetID); count == 0 {
			db.Delete(&model.EnvVarSet{}, *envSetID)
		}
	}
	if cfgSetID != nil {
		if count := countReleasesByConfigSet(db, *cfgSetID); count == 0 {
			db.Delete(&model.ConfigFileSet{}, *cfgSetID)
		}
	}
}

func countReleasesByArtifact(db *gorm.DB, id uint) int64 {
	var n int64
	db.Model(&model.Release{}).Where("artifact_id = ?", id).Count(&n)
	return n
}

func countReleasesByEnvSet(db *gorm.DB, id uint) int64 {
	var n int64
	db.Model(&model.Release{}).Where("env_set_id = ?", id).Count(&n)
	return n
}

func countReleasesByConfigSet(db *gorm.DB, id uint) int64 {
	var n int64
	db.Model(&model.Release{}).Where("config_set_id = ?", id).Count(&n)
	return n
}

// PruneOrphanArtifactFiles 扫描 data_dir/artifacts/ 下所有文件,删除 DB 中没有
// 对应 upload Artifact 记录的孤儿。DB 没有=磁盘文件是孤儿。
//
// 安全:仅扫 artifacts/ 子树,不递归到符号链接。
func PruneOrphanArtifactFiles(db *gorm.DB) {
	dataDir := artifactsDataDir()
	if dataDir == "" {
		return
	}
	root := filepath.Join(dataDir, "artifacts")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}

	var refs []string
	db.Model(&model.Artifact{}).
		Where("provider = ?", model.ArtifactProviderUpload).
		Pluck("ref", &refs)
	known := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		known[filepath.Clean(r)] = struct{}{}
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dataDir, path)
		if rerr != nil {
			return nil
		}
		if _, ok := known[filepath.Clean(rel)]; ok {
			return nil
		}
		if err := os.Remove(path); err == nil {
			log.Printf("[artifact-gc] removed orphan %s", rel)
		}
		return nil
	})
}

// tryRemoveUploadFile 仅对 upload provider 有效;Ref 是相对 data_dir 的路径。
// 其余 provider(docker/http/git/script/imported)没有面板本地物理文件可删。
func tryRemoveUploadFile(art model.Artifact) {
	if art.Provider != model.ArtifactProviderUpload || art.Ref == "" {
		return
	}
	clean := filepath.Clean(art.Ref)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || !strings.HasPrefix(clean, "artifacts/") {
		log.Printf("[release-prune] refuse to remove suspicious artifact ref: %q", art.Ref)
		return
	}
	dataDir := artifactsDataDir()
	if dataDir == "" {
		return
	}
	full := filepath.Join(dataDir, clean)
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		log.Printf("[release-prune] remove %s: %v", full, err)
	}
}
