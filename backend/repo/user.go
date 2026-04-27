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

func UpdateUserFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}
