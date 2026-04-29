package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func ListConfigSetsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint) ([]domain.ConfigFileSet, error) {
	var out []model.ConfigFileSet
	if err := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.ConfigFileSet, len(out))
	for i, c := range out {
		result[i] = model.ToDomainConfigFileSet(c)
	}
	return result, nil
}

func CreateConfigSet(ctx context.Context, db *gorm.DB, c *domain.ConfigFileSet) error {
	m := model.FromDomainConfigFileSet(*c)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainConfigFileSet(m)
	return nil
}

// DeleteConfigFileSetByID 硬删 ConfigFileSet。
func DeleteConfigFileSetByID(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.ConfigFileSet{}, id).Error
}
