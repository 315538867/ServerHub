package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// ListEnvSetsByServiceID 返回 EnvVarSet 列表,不含加密 Content 字段。
func ListEnvSetsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint) ([]domain.EnvVarSet, error) {
	var out []model.EnvVarSet
	if err := db.WithContext(ctx).
		Select("id, service_id, label, created_at").
		Where("service_id = ?", serviceID).
		Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.EnvVarSet, len(out))
	for i, e := range out {
		result[i] = model.ToDomainEnvVarSet(e)
	}
	return result, nil
}

func CreateEnvSet(ctx context.Context, db *gorm.DB, e *domain.EnvVarSet) error {
	m := model.FromDomainEnvVarSet(*e)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*e = model.ToDomainEnvVarSet(m)
	return nil
}

// DeleteEnvVarSetByID 硬删 EnvVarSet。
func DeleteEnvVarSetByID(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.EnvVarSet{}, id).Error
}
