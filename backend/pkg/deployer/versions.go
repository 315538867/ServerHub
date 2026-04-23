package deployer

import (
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// MaxVersionsPerDeploy bounds the rolling history window kept per deploy.
const MaxVersionsPerDeploy = 7

// SnapshotDeploy persists a DeployVersion row from the given Deploy + log id.
// Returns the created version's ID (0 on error).
func SnapshotDeploy(db *gorm.DB, d model.Service, logID uint, triggerSource, note string) uint {
	v := model.DeployVersion{
		DeployID:      d.ID,
		Version:       versionLabel(d),
		Status:        "success",
		TriggerSource: triggerSource,
		Type:          d.Type,
		WorkDir:       d.WorkDir,
		ComposeFile:   d.ComposeFile,
		StartCmd:      d.StartCmd,
		ImageName:     d.ImageName,
		Runtime:       d.Runtime,
		ConfigFiles:   d.ConfigFiles,
		EnvVars:       d.EnvVars,
		DeployLogID:   logID,
		Note:          note,
	}
	if err := db.Create(&v).Error; err != nil {
		return 0
	}
	return v.ID
}

// PruneVersions keeps only the most recent `keep` versions for a deploy,
// deleting older ones.
func PruneVersions(db *gorm.DB, deployID uint, keep int) {
	if keep <= 0 {
		return
	}
	var ids []uint
	db.Model(&model.DeployVersion{}).
		Where("deploy_id = ?", deployID).
		Order("created_at DESC").
		Offset(keep).
		Pluck("id", &ids)
	if len(ids) == 0 {
		return
	}
	db.Where("id IN ?", ids).Delete(&model.DeployVersion{})
}

// PruneAllVersions runs PruneVersions for every deploy id present in the
// deploy_versions table. Intended for the daily retention task.
func PruneAllVersions(db *gorm.DB, keep int) {
	var ids []uint
	db.Model(&model.DeployVersion{}).
		Distinct("deploy_id").
		Pluck("deploy_id", &ids)
	for _, id := range ids {
		PruneVersions(db, id, keep)
	}
}

func versionLabel(d model.Service) string {
	if d.ActualVersion != "" {
		return d.ActualVersion
	}
	if d.DesiredVersion != "" {
		return d.DesiredVersion
	}
	return ""
}
