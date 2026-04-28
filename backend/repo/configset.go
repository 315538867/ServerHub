package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func ListConfigSetsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint) ([]model.ConfigFileSet, error) {
	var out []model.ConfigFileSet
	if err := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateConfigSet(ctx context.Context, db *gorm.DB, c *model.ConfigFileSet) error {
	return db.WithContext(ctx).Create(c).Error
}
