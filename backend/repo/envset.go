package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// ListEnvSetsByServiceID 返回 EnvVarSet 列表,不含加密 Content 字段。
func ListEnvSetsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint) ([]model.EnvVarSet, error) {
	var out []model.EnvVarSet
	if err := db.WithContext(ctx).
		Select("id, service_id, label, created_at").
		Where("service_id = ?", serviceID).
		Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateEnvSet(ctx context.Context, db *gorm.DB, e *model.EnvVarSet) error {
	return db.WithContext(ctx).Create(e).Error
}
