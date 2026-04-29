package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetApplicationByID(ctx context.Context, db *gorm.DB, id uint) (domain.Application, error) {
	var a model.Application
	if err := db.WithContext(ctx).First(&a, id).Error; err != nil {
		return domain.Application{}, err
	}
	return model.ToDomainApplication(a), nil
}

func ListApplications(ctx context.Context, db *gorm.DB, serverID *uint) ([]domain.Application, error) {
	q := db.WithContext(ctx).Order("id asc")
	if serverID != nil {
		q = q.Where("server_id = ?", *serverID)
	}
	var out []model.Application
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Application, len(out))
	for i, a := range out {
		result[i] = model.ToDomainApplication(a)
	}
	return result, nil
}

func ListAllApplications(ctx context.Context, db *gorm.DB) ([]domain.Application, error) {
	var out []model.Application
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Application, len(out))
	for i, a := range out {
		result[i] = model.ToDomainApplication(a)
	}
	return result, nil
}

func ListApplicationsByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.Application, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Application
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Application, len(out))
	for i, a := range out {
		result[i] = model.ToDomainApplication(a)
	}
	return result, nil
}

func CreateApplication(ctx context.Context, db *gorm.DB, a *domain.Application) error {
	m := model.FromDomainApplication(*a)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*a = model.ToDomainApplication(m)
	return nil
}

func SaveApplication(ctx context.Context, db *gorm.DB, a *domain.Application) error {
	m := model.FromDomainApplication(*a)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*a = model.ToDomainApplication(m)
	return nil
}

func DeleteApplication(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Application{}, id).Error
}

// GetApplicationByName 按名称查 Application。
func GetApplicationByName(ctx context.Context, db *gorm.DB, name string) (domain.Application, error) {
	var a model.Application
	if err := db.WithContext(ctx).Where("name = ?", name).First(&a).Error; err != nil {
		return domain.Application{}, err
	}
	return model.ToDomainApplication(a), nil
}

func UpdateApplicationFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Application{}).Where("id = ?", id).Updates(updates).Error
}

func UpdatePrimaryService(ctx context.Context, db *gorm.DB, appID uint, serviceID *uint) error {
	return db.WithContext(ctx).Model(&model.Application{}).Where("id = ?", appID).Update("primary_service_id", serviceID).Error
}

// ClearPrimaryServiceIfMatch 若 app 的主服务恰好是 serviceID,则置空。
func ClearPrimaryServiceIfMatch(ctx context.Context, db *gorm.DB, appID, serviceID uint) error {
	return db.WithContext(ctx).Model(&model.Application{}).
		Where("id = ? AND primary_service_id = ?", appID, serviceID).
		Update("primary_service_id", nil).Error
}
