package repo

import (
	"context"
	"time"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

// rules

func ListAllAlertRules(ctx context.Context, db *gorm.DB) ([]domain.AlertRule, error) {
	var out []model.AlertRule
	if err := db.WithContext(ctx).Order("id asc").Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AlertRule, len(out))
	for i, r := range out {
		result[i] = model.ToDomainAlertRule(r)
	}
	return result, nil
}

func GetAlertRuleByID(ctx context.Context, db *gorm.DB, id uint) (domain.AlertRule, error) {
	var r model.AlertRule
	if err := db.WithContext(ctx).First(&r, id).Error; err != nil {
		return domain.AlertRule{}, err
	}
	return model.ToDomainAlertRule(r), nil
}

func ListAlertRules(ctx context.Context, db *gorm.DB, serverID *uint) ([]domain.AlertRule, error) {
	q := db.WithContext(ctx).Order("id desc")
	if serverID != nil {
		q = q.Where("server_id = ?", *serverID)
	}
	var out []model.AlertRule
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AlertRule, len(out))
	for i, r := range out {
		result[i] = model.ToDomainAlertRule(r)
	}
	return result, nil
}

func CreateAlertRule(ctx context.Context, db *gorm.DB, r *domain.AlertRule) error {
	m := model.FromDomainAlertRule(*r)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainAlertRule(m)
	return nil
}

func SaveAlertRule(ctx context.Context, db *gorm.DB, r *domain.AlertRule) error {
	m := model.FromDomainAlertRule(*r)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainAlertRule(m)
	return nil
}

func DeleteAlertRule(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.AlertRule{}, id).Error
}

// events

func ListAlertEventsPaginated(ctx context.Context, db *gorm.DB, offset, limit int) ([]domain.AlertEvent, int64, error) {
	var total int64
	if err := db.WithContext(ctx).Model(&model.AlertEvent{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.AlertEvent
	if err := db.WithContext(ctx).Order("sent_at desc").Offset(offset).Limit(limit).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	result := make([]domain.AlertEvent, len(out))
	for i, e := range out {
		result[i] = model.ToDomainAlertEvent(e)
	}
	return result, total, nil
}

func ListAlertEvents(ctx context.Context, db *gorm.DB, serverID *uint, limit int) ([]domain.AlertEvent, error) {
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
	result := make([]domain.AlertEvent, len(out))
	for i, e := range out {
		result[i] = model.ToDomainAlertEvent(e)
	}
	return result, nil
}

func CreateAlertEvent(ctx context.Context, db *gorm.DB, e *domain.AlertEvent) error {
	m := model.FromDomainAlertEvent(*e)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*e = model.ToDomainAlertEvent(m)
	return nil
}

func PruneAlertEventsBefore(ctx context.Context, db *gorm.DB, before time.Time) error {
	return db.WithContext(ctx).Where("sent_at < ?", before).Delete(&model.AlertEvent{}).Error
}

// channels

func ListAllNotifyChannels(ctx context.Context, db *gorm.DB) ([]domain.NotifyChannel, error) {
	var out []model.NotifyChannel
	if err := db.WithContext(ctx).Order("id asc").Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.NotifyChannel, len(out))
	for i, c := range out {
		result[i] = model.ToDomainNotifyChannel(c)
	}
	return result, nil
}

func ListNotifyChannels(ctx context.Context, db *gorm.DB) ([]domain.NotifyChannel, error) {
	var out []model.NotifyChannel
	if err := db.WithContext(ctx).Order("id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.NotifyChannel, len(out))
	for i, c := range out {
		result[i] = model.ToDomainNotifyChannel(c)
	}
	return result, nil
}

func GetNotifyChannelByID(ctx context.Context, db *gorm.DB, id uint) (domain.NotifyChannel, error) {
	var c model.NotifyChannel
	if err := db.WithContext(ctx).First(&c, id).Error; err != nil {
		return domain.NotifyChannel{}, err
	}
	return model.ToDomainNotifyChannel(c), nil
}

func CreateNotifyChannel(ctx context.Context, db *gorm.DB, c *domain.NotifyChannel) error {
	m := model.FromDomainNotifyChannel(*c)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainNotifyChannel(m)
	return nil
}

func SaveNotifyChannel(ctx context.Context, db *gorm.DB, c *domain.NotifyChannel) error {
	m := model.FromDomainNotifyChannel(*c)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*c = model.ToDomainNotifyChannel(m)
	return nil
}

func DeleteNotifyChannel(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.NotifyChannel{}, id).Error
}
