package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func ListDeployRunsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]model.DeployRun, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.DeployRun
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func GetDeployRunByIDAndServiceID(ctx context.Context, db *gorm.DB, id, serviceID uint) (model.DeployRun, error) {
	var r model.DeployRun
	if err := db.WithContext(ctx).Where("id = ? AND service_id = ?", id, serviceID).First(&r).Error; err != nil {
		return model.DeployRun{}, err
	}
	return r, nil
}
