package repo

import (
	"context"
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// rules

func ListAllAlertRules(ctx context.Context, db *gorm.DB) ([]model.AlertRule, error) {
	var out []model.AlertRule
	if err := db.WithContext(ctx).Order("id asc").Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func GetAlertRuleByID(ctx context.Context, db *gorm.DB, id uint) (model.AlertRule, error) {
	var r model.AlertRule
	if err := db.WithContext(ctx).First(&r, id).Error; err != nil {
		return model.AlertRule{}, err
	}
	return r, nil
}

func ListAlertRules(ctx context.Context, db *gorm.DB, serverID *uint) ([]model.AlertRule, error) {
	q := db.WithContext(ctx).Order("id desc")
	if serverID != nil {
		q = q.Where("server_id = ?", *serverID)
	}
	var out []model.AlertRule
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateAlertRule(ctx context.Context, db *gorm.DB, r *model.AlertRule) error {
	return db.WithContext(ctx).Create(r).Error
}

func SaveAlertRule(ctx context.Context, db *gorm.DB, r *model.AlertRule) error {
	return db.WithContext(ctx).Save(r).Error
}

func DeleteAlertRule(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.AlertRule{}, id).Error
}

// events

func ListAlertEventsPaginated(ctx context.Context, db *gorm.DB, offset, limit int) ([]model.AlertEvent, int64, error) {
	var total int64
	if err := db.WithContext(ctx).Model(&model.AlertEvent{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.AlertEvent
	if err := db.WithContext(ctx).Order("sent_at desc").Offset(offset).Limit(limit).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func ListAlertEvents(ctx context.Context, db *gorm.DB, serverID *uint, limit int) ([]model.AlertEvent, error) {
	q := db.WithContext(ctx).Order("id desc")
	if serverID != nil {
		q = q.Where("server_id = ?", *serverID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.AlertEvent
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateAlertEvent(ctx context.Context, db *gorm.DB, e *model.AlertEvent) error {
	return db.WithContext(ctx).Create(e).Error
}

func PruneAlertEventsBefore(ctx context.Context, db *gorm.DB, before time.Time) error {
	return db.WithContext(ctx).Where("sent_at < ?", before).Delete(&model.AlertEvent{}).Error
}

// channels

func ListAllNotifyChannels(ctx context.Context, db *gorm.DB) ([]model.NotifyChannel, error) {
	var out []model.NotifyChannel
	if err := db.WithContext(ctx).Order("id asc").Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListNotifyChannels(ctx context.Context, db *gorm.DB) ([]model.NotifyChannel, error) {
	var out []model.NotifyChannel
	if err := db.WithContext(ctx).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func GetNotifyChannelByID(ctx context.Context, db *gorm.DB, id uint) (model.NotifyChannel, error) {
	var c model.NotifyChannel
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return model.NotifyChannel{}, err
	}
	return c, nil
}

func CreateNotifyChannel(ctx context.Context, db *gorm.DB, c *model.NotifyChannel) error {
	return db.WithContext(ctx).Create(c).Error
}

func SaveNotifyChannel(ctx context.Context, db *gorm.DB, c *model.NotifyChannel) error {
	return db.WithContext(ctx).Save(c).Error
}

func DeleteNotifyChannel(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.NotifyChannel{}, id).Error
}
