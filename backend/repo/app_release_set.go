package repo

import (
	"context"
	"errors"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// ErrAlreadyApplying 表示 AppReleaseSet 正在 applying 状态,CAS 竞争失败。
var ErrAlreadyApplying = errors.New("app release set is currently applying")

func GetAppReleaseSetByID(ctx context.Context, db *gorm.DB, id uint) (domain.AppReleaseSet, error) {
	var s model.AppReleaseSet
	if err := db.WithContext(ctx).First(&s, id).Error; err != nil {
		return domain.AppReleaseSet{}, err
	}
	return model.ToDomainAppReleaseSet(s), nil
}

func GetAppReleaseSetByIDAndAppID(ctx context.Context, db *gorm.DB, id, appID uint) (domain.AppReleaseSet, error) {
	var s model.AppReleaseSet
	if err := db.WithContext(ctx).Where("id = ? AND application_id = ?", id, appID).First(&s).Error; err != nil {
		return domain.AppReleaseSet{}, err
	}
	return model.ToDomainAppReleaseSet(s), nil
}

func ListAppReleaseSetsByAppID(ctx context.Context, db *gorm.DB, appID uint) ([]domain.AppReleaseSet, error) {
	var out []model.AppReleaseSet
	if err := db.WithContext(ctx).Where("application_id = ?", appID).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AppReleaseSet, len(out))
	for i, s := range out {
		result[i] = model.ToDomainAppReleaseSet(s)
	}
	return result, nil
}

func CreateAppReleaseSet(ctx context.Context, db *gorm.DB, s *domain.AppReleaseSet) error {
	m := model.FromDomainAppReleaseSet(*s)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*s = model.ToDomainAppReleaseSet(m)
	return nil
}

func UpdateAppReleaseSetFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.AppReleaseSet{}).Where("id = ?", id).Updates(updates).Error
}

// CASAppReleaseSetToApplying 乐观锁:仅当当前 status != applying 时将其置为 applying。
// 若 CAS 失败(已是 applying),返回 ErrAlreadyApplying。
func CASAppReleaseSetToApplying(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Model(&model.AppReleaseSet{}).
		Where("id = ? AND status <> ?", id, model.AppReleaseSetStatusApplying).
		Update("status", model.AppReleaseSetStatusApplying)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlreadyApplying
	}
	return nil
}

// CountAppReleaseSetLabelLike 统计同 app 下 label 匹配给定 pattern 的记录数。
func CountAppReleaseSetLabelLike(ctx context.Context, db *gorm.DB, appID uint, pattern string) (int64, error) {
	var n int64
	if err := db.WithContext(ctx).Model(&model.AppReleaseSet{}).
		Where("application_id = ? AND label LIKE ?", appID, pattern).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}
