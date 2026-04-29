package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetDBConnByID(ctx context.Context, db *gorm.DB, id uint) (domain.DBConn, error) {
	var c model.DBConn
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return domain.DBConn{}, err
	}
	return model.ToDomainDBConn(c), nil
}

func ListDBConnsByServerID(ctx context.Context, db *gorm.DB, serverID uint) ([]domain.DBConn, error) {
	var out []model.DBConn
	if err := db.WithContext(ctx).Where("server_id = ?", serverID).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DBConn, len(out))
	for i, c := range out {
		result[i] = model.ToDomainDBConn(c)
	}
	return result, nil
}

func ListAllDBConns(ctx context.Context, db *gorm.DB) ([]domain.DBConn, error) {
	var out []model.DBConn
	if err := db.WithContext(ctx).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DBConn, len(out))
	for i, c := range out {
		result[i] = model.ToDomainDBConn(c)
	}
	return result, nil
}

func CreateDBConn(ctx context.Context, db *gorm.DB, c *domain.DBConn) error {
	m := model.FromDomainDBConn(*c)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainDBConn(m)
	return nil
}

func SaveDBConn(ctx context.Context, db *gorm.DB, c *domain.DBConn) error {
	m := model.FromDomainDBConn(*c)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainDBConn(m)
	return nil
}

// ListDBConns 按可选条件查询数据库连接；serverID=0 时不限制，applicationID=nil 时不限制。
func ListDBConns(ctx context.Context, db *gorm.DB, serverID uint, applicationID *uint) ([]domain.DBConn, error) {
	q := db.WithContext(ctx).Order("id desc")
	if serverID != 0 {
		q = q.Where("server_id = ?", serverID)
	}
	if applicationID != nil {
		q = q.Where("application_id = ?", *applicationID)
	}
	var out []model.DBConn
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DBConn, len(out))
	for i, c := range out {
		result[i] = model.ToDomainDBConn(c)
	}
	return result, nil
}

func DeleteDBConn(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.DBConn{}, id).Error
}
