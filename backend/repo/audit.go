package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// audit_log

func ListAuditLogs(ctx context.Context, db *gorm.DB, offset, limit int) ([]model.AuditLog, int64, error) {
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
	return out, total, nil
}

func CreateAuditLog(ctx context.Context, db *gorm.DB, l *model.AuditLog) error {
	return db.WithContext(ctx).Create(l).Error
}

// audit_apply

func ListAuditAppliesByEdge(ctx context.Context, db *gorm.DB, edgeID uint, limit int) ([]model.AuditApply, error) {
	q := db.WithContext(ctx).Where("edge_server_id = ?", edgeID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.AuditApply
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateAuditApply(ctx context.Context, db *gorm.DB, a *model.AuditApply) error {
	return db.WithContext(ctx).Create(a).Error
}
