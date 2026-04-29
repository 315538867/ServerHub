package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// ListLatestMetricPerServer 每个 server 取最新一条 metric（单条子查询,非 N+1）。
func ListLatestMetricPerServer(ctx context.Context, db *gorm.DB) ([]domain.Metric, error) {
	var out []model.Metric
	if err := db.WithContext(ctx).Where("id IN (?)",
		db.Model(&model.Metric{}).Select("MAX(id)").Group("server_id"),
	).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Metric, len(out))
	for i, m := range out {
		result[i] = model.ToDomainMetric(m)
	}
	return result, nil
}
