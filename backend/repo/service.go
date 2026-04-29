package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetServiceByID(ctx context.Context, db *gorm.DB, id uint) (domain.Service, error) {
	var s model.Service
	if err := db.WithContext(ctx).First(&s, id).Error; err != nil {
		return domain.Service{}, err
	}
	return model.ToDomainService(s), nil
}

func GetServiceByWebhookSecret(ctx context.Context, db *gorm.DB, token string) (domain.Service, error) {
	var s model.Service
	if err := db.WithContext(ctx).Where("webhook_secret = ?", token).First(&s).Error; err != nil {
		return domain.Service{}, err
	}
	return model.ToDomainService(s), nil
}

func ListServicesByServerID(ctx context.Context, db *gorm.DB, serverID uint) ([]domain.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Where("server_id = ?", serverID).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Service, len(out))
	for i, s := range out {
		result[i] = model.ToDomainService(s)
	}
	return result, nil
}

func ListServicesByApplicationID(ctx context.Context, db *gorm.DB, appID uint) ([]domain.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Where("application_id = ?", appID).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Service, len(out))
	for i, s := range out {
		result[i] = model.ToDomainService(s)
	}
	return result, nil
}

func ListServicesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.Service, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Service
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Service, len(out))
	for i, s := range out {
		result[i] = model.ToDomainService(s)
	}
	return result, nil
}

func CreateService(ctx context.Context, db *gorm.DB, s *domain.Service) error {
	m := model.FromDomainService(*s)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*s = model.ToDomainService(m)
	return nil
}

func SaveService(ctx context.Context, db *gorm.DB, s *domain.Service) error {
	m := model.FromDomainService(*s)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*s = model.ToDomainService(m)
	return nil
}

func UpdateServiceFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Service{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteService(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Service{}, id).Error
}

func ListAllServices(ctx context.Context, db *gorm.DB) ([]domain.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Service, len(out))
	for i, s := range out {
		result[i] = model.ToDomainService(s)
	}
	return result, nil
}

// ListAutoSyncServices 返回 auto_sync=true 且 current_release_id 不为空的 Service。
func ListAutoSyncServices(ctx context.Context, db *gorm.DB) ([]domain.Service, error) {
	var out []model.Service
	if err := db.WithContext(ctx).
		Where("auto_sync = ? AND current_release_id IS NOT NULL", true).
		Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Service, len(out))
	for i, s := range out {
		result[i] = model.ToDomainService(s)
	}
	return result, nil
}

// CASServiceSyncStatus 仅当 sync_status != "syncing" 时更新为 target。
// 返回 ErrAlreadyApplying 表示已被另一 reconciler goroutine 处理。
func CASServiceSyncStatus(ctx context.Context, db *gorm.DB, id uint, target string) error {
	res := db.WithContext(ctx).Model(&model.Service{}).
		Where("id = ? AND sync_status != ?", id, "syncing").
		Update("sync_status", target)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlreadyApplying
	}
	return nil
}

// UpdateServiceSyncStatus 直接更新 sync_status 字段。
func UpdateServiceSyncStatus(ctx context.Context, db *gorm.DB, id uint, status string) error {
	return db.WithContext(ctx).Model(&model.Service{}).
		Where("id = ?", id).
		Update("sync_status", status).Error
}

// ListServiceFingerprintsByServer 返回指定 server 上所有非空 source_fingerprint 列表。
func ListServiceFingerprintsByServer(ctx context.Context, db *gorm.DB, serverID uint) ([]string, error) {
	var out []string
	err := db.WithContext(ctx).Model(&model.Service{}).
		Where("server_id = ? AND source_fingerprint != ''", serverID).
		Pluck("source_fingerprint", &out).Error
	return out, err
}

// ListAllServiceIDs 返回所有 Service ID 列表。
func ListAllServiceIDs(ctx context.Context, db *gorm.DB) ([]uint, error) {
	var ids []uint
	err := db.WithContext(ctx).Model(&model.Service{}).Pluck("id", &ids).Error
	return ids, err
}

// GetServiceBySource 按 (server_id, source_kind, source_id) 查唯一 Service。
func GetServiceBySource(ctx context.Context, db *gorm.DB, serverID uint, kind, sourceID string) (domain.Service, error) {
	var s model.Service
	err := db.WithContext(ctx).Where("server_id = ? AND source_kind = ? AND source_id = ?",
		serverID, kind, sourceID).First(&s).Error
	if err != nil {
		return domain.Service{}, err
	}
	return model.ToDomainService(s), nil
}
