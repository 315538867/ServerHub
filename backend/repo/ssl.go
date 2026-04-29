package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetCertByID(ctx context.Context, db *gorm.DB, id uint) (domain.SSLCert, error) {
	var c model.SSLCert
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return domain.SSLCert{}, err
	}
	return model.ToDomainSSLCert(c), nil
}

func GetCertByServerAndDomain(ctx context.Context, db *gorm.DB, serverID uint, domainName string) (domain.SSLCert, error) {
	var c model.SSLCert
	if err := db.WithContext(ctx).Where("server_id = ? AND domain = ?", serverID, domainName).First(&c).Error; err != nil {
		return domain.SSLCert{}, err
	}
	return model.ToDomainSSLCert(c), nil
}

func ListCerts(ctx context.Context, db *gorm.DB, serverID uint, applicationID *uint) ([]domain.SSLCert, error) {
	q := db.WithContext(ctx).Order("id desc")
	if serverID != 0 {
		q = q.Where("server_id = ?", serverID)
	}
	if applicationID != nil {
		q = q.Where("application_id = ?", *applicationID)
	}
	var out []model.SSLCert
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.SSLCert, len(out))
	for i, c := range out {
		result[i] = model.ToDomainSSLCert(c)
	}
	return result, nil
}

func CreateCert(ctx context.Context, db *gorm.DB, c *domain.SSLCert) error {
	m := model.FromDomainSSLCert(*c)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainSSLCert(m)
	return nil
}

func SaveCert(ctx context.Context, db *gorm.DB, c *domain.SSLCert) error {
	m := model.FromDomainSSLCert(*c)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainSSLCert(m)
	return nil
}

func DeleteCert(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.SSLCert{}, id).Error
}

// UpdateCertFields 按 ID 更新证书部分字段。
func UpdateCertFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.SSLCert{}).Where("id = ?", id).Updates(updates).Error
}
