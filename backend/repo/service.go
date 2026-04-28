package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetServiceByID(ctx context.Context, db *gorm.DB, id uint) (model.Service, error) {
	var s model.Service
	if err := db.WithContext(ctx).First(&s, id).Error; err != nil {
		return model.Service{}, err
	}
	return s, nil
}

func GetServiceByWebhookSecret(ctx context.Context, db *gorm.DB, token string) (model.Service, error) {
	var s model.Service
	if err := db.WithContext(ctx).Where("webhook_secret = ?", token).First(&s).Error; err != nil {
		return model.Service{}, err
	}
	return s, nil
}

func ListServicesByServerID(ctx context.Context, db *gorm.DB, serverID uint) ([]model.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Where("server_id = ?", serverID).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListServicesByApplicationID(ctx context.Context, db *gorm.DB, appID uint) ([]model.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Where("application_id = ?", appID).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListServicesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.Service, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Service
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateService(ctx context.Context, db *gorm.DB, s *model.Service) error {
	return db.WithContext(ctx).Create(s).Error
}

func SaveService(ctx context.Context, db *gorm.DB, s *model.Service) error {
	return db.WithContext(ctx).Save(s).Error
}

func UpdateServiceFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Service{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteService(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Service{}, id).Error
}

func ListAllServices(ctx context.Context, db *gorm.DB) ([]model.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
