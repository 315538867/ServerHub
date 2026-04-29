package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetArtifactByID(ctx context.Context, db *gorm.DB, id uint) (domain.Artifact, error) {
	var a model.Artifact
	if err := db.WithContext(ctx).First(&a, id).Error; err != nil {
		return domain.Artifact{}, err
	}
	return model.ToDomainArtifact(a), nil
}

func GetArtifactByIDAndServiceID(ctx context.Context, db *gorm.DB, id, serviceID uint) (domain.Artifact, error) {
	var a model.Artifact
	if err := db.WithContext(ctx).Where("id = ? AND service_id = ?", id, serviceID).First(&a).Error; err != nil {
		return domain.Artifact{}, err
	}
	return model.ToDomainArtifact(a), nil
}

func ListArtifactsByServiceID(ctx context.Context, db *gorm.DB, serviceID uint, limit int) ([]domain.Artifact, error) {
	q := db.WithContext(ctx).Where("service_id = ?", serviceID).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.Artifact
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Artifact, len(out))
	for i, a := range out {
		result[i] = model.ToDomainArtifact(a)
	}
	return result, nil
}

func CreateArtifact(ctx context.Context, db *gorm.DB, a *domain.Artifact) error {
	m := model.FromDomainArtifact(*a)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*a = model.ToDomainArtifact(m)
	return nil
}

// DeleteArtifactByID 硬删 Artifact。
func DeleteArtifactByID(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&model.Artifact{}, id).Error
}

// ListUploadArtifactRefs 返回所有 provider=upload 的 Artifact ref 列表。
func ListUploadArtifactRefs(ctx context.Context, db *gorm.DB) ([]string, error) {
	var refs []string
	err := db.WithContext(ctx).Model(&model.Artifact{}).
		Where("provider = ?", domain.ArtifactProviderUpload).
		Pluck("ref", &refs).Error
	return refs, err
}
