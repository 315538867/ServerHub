package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// audit_log

// ListAuditLogsFiltered 按可选条件分页列出审计日志。
func ListAuditLogsFiltered(ctx context.Context, db *gorm.DB, username, path, status string, offset, limit int) ([]domain.AuditLog, int64, error) {
	q := db.WithContext(ctx).Model(&model.AuditLog{})
	if username != "" {
		q = q.Where("username LIKE ?", username+"%")
	}
	if path != "" {
		q = q.Where("path LIKE ?", path+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.AuditLog
	if err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	result := make([]domain.AuditLog, len(out))
	for i, l := range out {
		result[i] = model.ToDomainAuditLog(l)
	}
	return result, total, nil
}

func ListAuditLogs(ctx context.Context, db *gorm.DB, offset, limit int) ([]domain.AuditLog, int64, error) {
	var out []model.AuditLog
	var total int64
	q := db.WithContext(ctx).Model(&model.AuditLog{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q2 := q.Order("id desc")
	if limit > 0 {
		q2 = q2.Limit(limit).Offset(offset)
	}
	if err := q2.Find(&out).Error; err != nil {
		return nil, 0, err
	}
	result := make([]domain.AuditLog, len(out))
	for i, l := range out {
		result[i] = model.ToDomainAuditLog(l)
	}
	return result, total, nil
}

func CreateAuditLog(ctx context.Context, db *gorm.DB, l *domain.AuditLog) error {
	m := model.FromDomainAuditLog(*l)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*l = model.ToDomainAuditLog(m)
	return nil
}

// audit_apply

func ListAuditAppliesByEdge(ctx context.Context, db *gorm.DB, edgeID uint, limit int) ([]domain.AuditApply, error) {
	q := db.WithContext(ctx).Where("edge_server_id = ?", edgeID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.AuditApply
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AuditApply, len(out))
	for i, a := range out {
		result[i] = model.ToDomainAuditApply(a)
	}
	return result, nil
}

func CreateAuditApply(ctx context.Context, db *gorm.DB, a *domain.AuditApply) error {
	m := model.FromDomainAuditApply(*a)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*a = model.ToDomainAuditApply(m)
	return nil
}
