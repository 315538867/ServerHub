package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetApplicationByID(ctx context.Context, db *gorm.DB, id uint) (model.Application, error) {
	var a model.Application
	if err := db.WithContext(ctx).First(&a, id).Error; err != nil {
		return model.Application{}, err
	}
	return a, nil
}

func ListAllApplications(ctx context.Context, db *gorm.DB) ([]model.Application, error) {
	var out []model.Application
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListApplicationsByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.Application, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Application
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateApplication(ctx context.Context, db *gorm.DB, a *model.Application) error {
	return db.WithContext(ctx).Create(a).Error
}

func SaveApplication(ctx context.Context, db *gorm.DB, a *model.Application) error {
	return db.WithContext(ctx).Save(a).Error
}

func DeleteApplication(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Application{}, id).Error
}

func UpdateApplicationFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Application{}).Where("id = ?", id).Updates(updates).Error
}

func UpdatePrimaryService(ctx context.Context, db *gorm.DB, appID uint, serviceID *uint) error {
	return db.WithContext(ctx).Model(&model.Application{}).Where("id = ?", appID).Update("primary_service_id", serviceID).Error
}
