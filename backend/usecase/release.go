// Package usecase: release.go 封装 Release 创建的跨表校验逻辑。
//
// 简单列表/单查由 handler 直调 repo；createRelease 涉及
// service 存在性 + artifact 归属校验 + auto-label 生成,放 usecase。
//
// TODO R7: 切 ports interface,移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// CreateReleaseParams 是 CreateRelease 的入参。
type CreateReleaseParams struct {
	ServiceID   uint
	Label       string
	ArtifactID  uint
	EnvSetID    *uint
	ConfigSetID *uint
	StartSpec   string // JSON string
	Note        string
}

// CreateRelease 校验 service 存在性 + artifact 归属 + 非 imported,
// 若 label 为空则自动生成,最后创建 Release。
func CreateRelease(ctx context.Context, db *gorm.DB, p CreateReleaseParams) (model.Release, error) {
	if _, err := repo.GetServiceByID(ctx, db, p.ServiceID); err != nil {
		return model.Release{}, errors.New("service not found")
	}
	art, err := repo.GetArtifactByIDAndServiceID(ctx, db, p.ArtifactID, p.ServiceID)
	if err != nil {
		return model.Release{}, errors.New("artifact not found")
	}
	if art.Provider == model.ArtifactProviderImported {
		return model.Release{}, errors.New("imported artifact cannot be used for new release; pick a real provider")
	}

	label := p.Label
	if label == "" {
		label = releaseAutoLabel(ctx, db, p.ServiceID)
	}

	rel := model.Release{
		ServiceID:   p.ServiceID,
		Label:       label,
		ArtifactID:  p.ArtifactID,
		EnvSetID:    p.EnvSetID,
		ConfigSetID: p.ConfigSetID,
		StartSpec:   p.StartSpec,
		Note:        p.Note,
		Status:      model.ReleaseStatusDraft,
	}
	if err := repo.CreateRelease(ctx, db, &rel); err != nil {
		return model.Release{}, err
	}
	return rel, nil
}

// releaseAutoLabel 生成 YYYY-MM-DD-N 兜底标签。
func releaseAutoLabel(ctx context.Context, db *gorm.DB, serviceID uint) string {
	today := time.Now().Format("2006-01-02")
	n, _ := repo.CountReleaseLabelLike(ctx, db, serviceID, today+"-%")
	return today + "-" + strconv.FormatInt(n+1, 10)
}
