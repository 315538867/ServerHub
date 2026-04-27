package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func ListAllSettings(ctx context.Context, db *gorm.DB) ([]model.Setting, error) {
	var out []model.Setting
	if err := db.WithContext(ctx).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func GetSetting(ctx context.Context, db *gorm.DB, key string) (model.Setting, error) {
	var s model.Setting
	if err := db.WithContext(ctx).Where("key = ?", key).First(&s).Error; err != nil {
		return model.Setting{}, err
	}
	return s, nil
}

func UpsertSetting(ctx context.Context, db *gorm.DB, key, value string) error {
	return db.WithContext(ctx).Save(&model.Setting{Key: key, Value: value}).Error
}

func ReplaceAllSettings(ctx context.Context, db *gorm.DB, items []model.Setting) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.Setting{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}
