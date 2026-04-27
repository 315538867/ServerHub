package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetDBConnByID(ctx context.Context, db *gorm.DB, id uint) (model.DBConn, error) {
	var c model.DBConn
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return model.DBConn{}, err
	}
	return c, nil
}

func ListDBConnsByServerID(ctx context.Context, db *gorm.DB, serverID uint) ([]model.DBConn, error) {
	var out []model.DBConn
	if err := db.WithContext(ctx).Where("server_id = ?", serverID).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListAllDBConns(ctx context.Context, db *gorm.DB) ([]model.DBConn, error) {
	var out []model.DBConn
	if err := db.WithContext(ctx).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateDBConn(ctx context.Context, db *gorm.DB, c *model.DBConn) error {
	return db.WithContext(ctx).Create(c).Error
}

func SaveDBConn(ctx context.Context, db *gorm.DB, c *model.DBConn) error {
	return db.WithContext(ctx).Save(c).Error
}

func DeleteDBConn(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.DBConn{}, id).Error
}
