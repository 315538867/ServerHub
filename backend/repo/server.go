package repo

import (
	"context"
	"errors"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetServerByID(ctx context.Context, db *gorm.DB, id uint) (domain.Server, error) {
	var s model.Server
	if err := db.WithContext(ctx).First(&s, id).Error; err != nil {
		return domain.Server{}, err
	}
	return model.ToDomainServer(s), nil
}

func ListAllServers(ctx context.Context, db *gorm.DB) ([]domain.Server, error) {
	var out []model.Server
	if err := db.WithContext(ctx).Order("id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Server, len(out))
	for i, s := range out {
		result[i] = model.ToDomainServer(s)
	}
	return result, nil
}

func ListServersByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.Server, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Server
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Server, len(out))
	for i, s := range out {
		result[i] = model.ToDomainServer(s)
	}
	return result, nil
}

func CreateServer(ctx context.Context, db *gorm.DB, s *domain.Server) error {
	m := model.FromDomainServer(*s)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*s = model.ToDomainServer(m)
	return nil
}

func SaveServer(ctx context.Context, db *gorm.DB, s *domain.Server) error {
	m := model.FromDomainServer(*s)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*s = model.ToDomainServer(m)
	return nil
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

func ListMetricsByServerID(ctx context.Context, db *gorm.DB, serverID uint, limit int) ([]domain.Metric, error) {
	q := db.WithContext(ctx).Where("server_id = ?", serverID).Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Metric
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Metric, len(out))
	for i, m := range out {
		result[i] = model.ToDomainMetric(m)
	}
	return result, nil
}

func CreateProbe(ctx context.Context, db *gorm.DB, p *domain.ServerProbe) error {
	m := model.FromDomainServerProbe(*p)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*p = model.ToDomainServerProbe(m)
	return nil
}

func CreateMetric(ctx context.Context, db *gorm.DB, m *domain.Metric) error {
	mm := model.FromDomainMetric(*m)
	return db.WithContext(ctx).Create(&mm).Error
}

var ErrNotFound = gorm.ErrRecordNotFound

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
