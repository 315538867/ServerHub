package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetReleaseByID(ctx context.Context, db *gorm.DB, id uint) (domain.Release, error) {
	var r model.Release
	if err := db.WithContext(ctx).First(&r, id).Error; err != nil {
		return domain.Release{}, err
	}
	return model.ToDomainRelease(r), nil
}

func ListReleasesByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]domain.Release, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Release
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Release, len(out))
	for i, r := range out {
		result[i] = model.ToDomainRelease(r)
	}
	return result, nil
}

func ListReleasesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.Release, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Release
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Release, len(out))
	for i, r := range out {
		result[i] = model.ToDomainRelease(r)
	}
	return result, nil
}

func CreateRelease(ctx context.Context, db *gorm.DB, r *domain.Release) error {
	m := model.FromDomainRelease(*r)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainRelease(m)
	return nil
}

func SaveRelease(ctx context.Context, db *gorm.DB, r *domain.Release) error {
	m := model.FromDomainRelease(*r)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainRelease(m)
	return nil
}

func UpdateReleaseFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Release{}).Where("id = ?", id).Updates(updates).Error
}

func CountReleasesByServiceID(ctx context.Context, db *gorm.DB, serviceID uint) (int64, error) {
	var n int64
	if err := db.WithContext(ctx).Model(&model.Release{}).Where("service_id = ?", serviceID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// CountReleaseLabelLike 统计同 service 下 label 匹配给定 pattern 的记录数。
func CountReleaseLabelLike(ctx context.Context, db *gorm.DB, serviceID uint, pattern string) (int64, error) {
	var n int64
	if err := db.WithContext(ctx).Model(&model.Release{}).
		Where("service_id = ? AND label LIKE ?", serviceID, pattern).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// FindPrevRelease 返回 serviceID 下 id 严格小于 excludeID 的最近一条
// active/archived Release ID。找不到返回 0。
func FindPrevRelease(ctx context.Context, db *gorm.DB, serviceID, excludeID uint) uint {
	var r model.Release
	err := db.WithContext(ctx).
		Where("service_id = ? AND id < ? AND status IN ?",
			serviceID, excludeID,
			[]string{domain.ReleaseStatusActive, domain.ReleaseStatusArchived}).
		Order("id desc").First(&r).Error
	if err != nil {
		return 0
	}
	return r.ID
}

// GetReleaseByServiceAndID 按 (service_id, id) 查 Release。
func GetReleaseByServiceAndID(ctx context.Context, db *gorm.DB, serviceID, id uint) (domain.Release, error) {
	var r model.Release
	if err := db.WithContext(ctx).Where("id = ? AND service_id = ?", id, serviceID).First(&r).Error; err != nil {
		return domain.Release{}, err
	}
	return model.ToDomainRelease(r), nil
}

// DeleteReleaseByID 硬删 Release。
func DeleteReleaseByID(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Release{}, id).Error
}

// CountReleasesByArtifactID 统计引用指定 artifact 的 Release 数。
func CountReleasesByArtifactID(ctx context.Context, db *gorm.DB, id uint) (int64, error) {
	var n int64
	err := db.WithContext(ctx).Model(&model.Release{}).Where("artifact_id = ?", id).Count(&n).Error
	return n, err
}

// CountReleasesByEnvSetID 统计引用指定 env_set 的 Release 数。
func CountReleasesByEnvSetID(ctx context.Context, db *gorm.DB, id uint) (int64, error) {
	var n int64
	err := db.WithContext(ctx).Model(&model.Release{}).Where("env_set_id = ?", id).Count(&n).Error
	return n, err
}

// CountReleasesByConfigSetID 统计引用指定 config_set 的 Release 数。
func CountReleasesByConfigSetID(ctx context.Context, db *gorm.DB, id uint) (int64, error) {
	var n int64
	err := db.WithContext(ctx).Model(&model.Release{}).Where("config_set_id = ?", id).Count(&n).Error
	return n, err
}

// ListActiveReleaseIDsByService 返回指定 service 下所有 active Release 的 ID。
func ListActiveReleaseIDsByService(ctx context.Context, db *gorm.DB, serviceID uint) ([]uint, error) {
	var ids []uint
	err := db.WithContext(ctx).Model(&model.Release{}).
		Where("service_id = ? AND status = ?", serviceID, domain.ReleaseStatusActive).
		Pluck("id", &ids).Error
	return ids, err
}

// ListExcessReleases 返回 serviceID 下按 created_at 降序排列、跳过 keep 条后的 Release。
func ListExcessReleases(ctx context.Context, db *gorm.DB, serviceID uint, keep int) ([]domain.Release, error) {
	var cands []model.Release
	if err := db.WithContext(ctx).Where("service_id = ?", serviceID).
		Order("created_at DESC").Offset(keep).Find(&cands).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Release, len(cands))
	for i, r := range cands {
		result[i] = model.ToDomainRelease(r)
	}
	return result, nil
}

// ActivateRelease 将 release 设为 active: 更新 service.current_release_id,
// archive 其他 active release, 设置目标 release 为 active。
func ActivateRelease(ctx context.Context, db *gorm.DB, serviceID, releaseID uint) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Service{}).Where("id = ?", serviceID).
			Update("current_release_id", releaseID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Release{}).
			Where("service_id = ? AND id <> ? AND status = ?", serviceID, releaseID, domain.ReleaseStatusActive).
			Update("status", domain.ReleaseStatusArchived).Error; err != nil {
			return err
		}
		return tx.Model(&model.Release{}).Where("id = ?", releaseID).
			Update("status", domain.ReleaseStatusActive).Error
	})
}
