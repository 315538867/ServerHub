package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetCertByID(ctx context.Context, db *gorm.DB, id uint) (model.SSLCert, error) {
	var c model.SSLCert
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return model.SSLCert{}, err
	}
	return c, nil
}

func GetCertByServerAndDomain(ctx context.Context, db *gorm.DB, serverID uint, domain string) (model.SSLCert, error) {
	var c model.SSLCert
	if err := db.WithContext(ctx).Where("server_id = ? AND domain = ?", serverID, domain).First(&c).Error; err != nil {
		return model.SSLCert{}, err
	}
	return c, nil
}

func ListCerts(ctx context.Context, db *gorm.DB, serverID uint, applicationID *uint) ([]model.SSLCert, error) {
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
	return out, nil
}

func CreateCert(ctx context.Context, db *gorm.DB, c *model.SSLCert) error {
	return db.WithContext(ctx).Create(c).Error
}

func SaveCert(ctx context.Context, db *gorm.DB, c *model.SSLCert) error {
	return db.WithContext(ctx).Save(c).Error
}

func DeleteCert(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.SSLCert{}, id).Error
}
