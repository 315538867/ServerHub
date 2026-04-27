package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetArtifactByID(ctx context.Context, db *gorm.DB, id uint) (model.Artifact, error) {
	var a model.Artifact
	if err := db.WithContext(ctx).First(&a, id).Error; err != nil {
		return model.Artifact{}, err
	}
	return a, nil
}

func GetArtifactByIDAndServiceID(ctx context.Context, db *gorm.DB, id, serviceID uint) (model.Artifact, error) {
	var a model.Artifact
	if err := db.WithContext(ctx).Where("id = ? AND service_id = ?", id, serviceID).First(&a).Error; err != nil {
		return model.Artifact{}, err
	}
	return a, nil
}

func ListArtifactsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]model.Artifact, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Artifact
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func CreateArtifact(ctx context.Context, db *gorm.DB, a *model.Artifact) error {
	return db.WithContext(ctx).Create(a).Error
}
