package repo

import (
	"context"
	"errors"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetNginxProfileByEdgeID(ctx context.Context, db *gorm.DB, edgeID uint) (model.NginxProfile, error) {
	var p model.NginxProfile
	if err := db.WithContext(ctx).Where("edge_server_id = ?", edgeID).First(&p).Error; err != nil {
		return model.NginxProfile{}, err
	}
	return p, nil
}

func UpsertNginxProfile(ctx context.Context, db *gorm.DB, edgeID uint, updates map[string]any) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.NginxProfile
		err := tx.Where("edge_server_id = ?", edgeID).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row := model.NginxProfile{EdgeServerID: edgeID}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			return tx.Model(&model.NginxProfile{}).Where("id = ?", row.ID).Updates(updates).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&model.NginxProfile{}).Where("id = ?", p.ID).Updates(updates).Error
	})
}
