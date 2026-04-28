package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetReleaseByID(ctx context.Context, db *gorm.DB, id uint) (model.Release, error) {
	var r model.Release
	if err := db.WithContext(ctx).First(&r, id).Error; err != nil {
		return model.Release{}, err
	}
	return r, nil
}

func ListReleasesByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]model.Release, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Release
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListReleasesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.Release, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Release
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateRelease(ctx context.Context, db *gorm.DB, r *model.Release) error {
	return db.WithContext(ctx).Create(r).Error
}

func SaveRelease(ctx context.Context, db *gorm.DB, r *model.Release) error {
	return db.WithContext(ctx).Save(r).Error
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
			[]string{model.ReleaseStatusActive, model.ReleaseStatusArchived}).
		Order("id desc").First(&r).Error
	if err != nil {
		return 0
	}
	return r.ID
}
