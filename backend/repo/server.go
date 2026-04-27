package repo

import (
	"context"
	"errors"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetServerByID(ctx context.Context, db *gorm.DB, id uint) (model.Server, error) {
	var s model.Server
	if err := db.WithContext(ctx).First(&s, id).Error; err != nil {
		return model.Server{}, err
	}
	return s, nil
}

func ListAllServers(ctx context.Context, db *gorm.DB) ([]model.Server, error) {
	var out []model.Server
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListServersByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.Server, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Server
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateServer(ctx context.Context, db *gorm.DB, s *model.Server) error {
	return db.WithContext(ctx).Create(s).Error
}

func SaveServer(ctx context.Context, db *gorm.DB, s *model.Server) error {
	return db.WithContext(ctx).Save(s).Error
}

func UpdateServerFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Server{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteServerCascade(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Server{}, id).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.Metric{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.Service{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.DBConn{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.AlertRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.AlertEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", id).Delete(&model.SSLCert{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func ListMetricsByServerID(ctx context.Context, db *gorm.DB, serverID uint, limit int) ([]model.Metric, error) {
	q := db.WithContext(ctx).Where("server_id = ?", serverID).Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Metric
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateProbe(ctx context.Context, db *gorm.DB, p *model.ServerProbe) error {
	return db.WithContext(ctx).Create(p).Error
}

func CreateMetric(ctx context.Context, db *gorm.DB, m *model.Metric) error {
	return db.WithContext(ctx).Create(m).Error
}

var ErrNotFound = gorm.ErrRecordNotFound

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
