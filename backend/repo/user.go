package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetUserByID(ctx context.Context, db *gorm.DB, id uint) (model.User, error) {
	var u model.User
	if err := db.WithContext(ctx).First(&u, id).Error; err != nil {
		return model.User{}, err
	}
	return u, nil
}

func GetUserByUsername(ctx context.Context, db *gorm.DB, username string) (model.User, error) {
	var u model.User
	if err := db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return model.User{}, err
	}
	return u, nil
}

func CountUsers(ctx context.Context, db *gorm.DB) (int64, error) {
	var n int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func CountUsersByUsernameExcludingID(ctx context.Context, db *gorm.DB, username string, excludeID uint) (int64, error) {
	var n int64
	if err := db.WithContext(ctx).Model(&model.User{}).Where("username = ? AND id <> ?", username, excludeID).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func CreateUser(ctx context.Context, db *gorm.DB, u *model.User) error {
	return db.WithContext(ctx).Create(u).Error
}

func SaveUser(ctx context.Context, db *gorm.DB, u *model.User) error {
	return db.WithContext(ctx).Save(u).Error
}

func ListUsersByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.User
	if err := db.WithContext(ctx).Select("id, username").Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func UpdateUserFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func UpdateUserLoginMeta(ctx context.Context, db *gorm.DB, id uint, lastLogin any, lastIP string) error {
	return db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login": lastLogin,
		"last_ip":    lastIP,
	}).Error
}

func UpdateUserMFAMeta(ctx context.Context, db *gorm.DB, id uint, secret string, enabled bool) error {
	return db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"mfa_secret":  secret,
		"mfa_enabled": enabled,
	}).Error
}

func ClearUserMFAMeta(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"mfa_secret":     "",
		"mfa_enabled":    false,
		"last_totp_step": 0,
	}).Error
}

// AdvanceUserLastTOTPStep 仅当 step 严格大于 last_totp_step 时推进，用于防重放。
// 若 RowsAffected == 0，表示 replay 或用户不存在。
func AdvanceUserLastTOTPStep(ctx context.Context, db *gorm.DB, id uint, step int64) (int64, error) {
	res := db.WithContext(ctx).Model(&model.User{}).
		Where("id = ? AND last_totp_step < ?", id, step).
		Update("last_totp_step", step)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
