package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func ListDeployRunsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]domain.DeployRun, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.DeployRun
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DeployRun, len(out))
	for i, r := range out {
		result[i] = model.ToDomainDeployRun(r)
	}
	return result, nil
}

func GetDeployRunByIDAndServiceID(ctx context.Context, db *gorm.DB, id, serviceID uint) (domain.DeployRun, error) {
	var r model.DeployRun
	if err := db.WithContext(ctx).Where("id = ? AND service_id = ?", id, serviceID).First(&r).Error; err != nil {
		return domain.DeployRun{}, err
	}
	return model.ToDomainDeployRun(r), nil
}

// CreateDeployRun 创建 deploy_run 行,回填 ID 生成值。
func CreateDeployRun(ctx context.Context, db *gorm.DB, r *domain.DeployRun) error {
	m := model.FromDomainDeployRun(*r)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainDeployRun(m)
	return nil
}

// UpdateDeployRunFields 按 ID 更新 deploy_run 字段。
func UpdateDeployRunFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.DeployRun{}).Where("id = ?", id).Updates(updates).Error
}
