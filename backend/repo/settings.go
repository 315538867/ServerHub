package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ListAllSettings(ctx context.Context, db *gorm.DB) ([]domain.Setting, error) {
	var out []model.Setting
	if err := db.WithContext(ctx).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Setting, len(out))
	for i, s := range out {
		result[i] = model.ToDomainSetting(s)
	}
	return result, nil
}

func GetSetting(ctx context.Context, db *gorm.DB, key string) (domain.Setting, error) {
	var s model.Setting
	if err := db.WithContext(ctx).Where("key = ?", key).First(&s).Error; err != nil {
		return domain.Setting{}, err
	}
	return model.ToDomainSetting(s), nil
}

func UpsertSetting(ctx context.Context, db *gorm.DB, key, value string) error {
	return db.WithContext(ctx).Save(&model.Setting{Key: key, Value: value}).Error
}

func UpsertSettingsBulk(ctx context.Context, db *gorm.DB, items map[string]string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, v := range items {
			s := model.Setting{Key: k, Value: v}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value"}),
			}).Create(&s).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ReplaceAllSettings(ctx context.Context, db *gorm.DB, items []domain.Setting) error {
	modelItems := make([]model.Setting, len(items))
	for i, item := range items {
		modelItems[i] = model.FromDomainSetting(item)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.Setting{}).Error; err != nil {
			return err
		}
		if len(modelItems) == 0 {
			return nil
		}
		return tx.Create(&modelItems).Error
	})
}
