package repo

import (
	"context"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetIngressByID(ctx context.Context, db *gorm.DB, id uint) (model.Ingress, error) {
	var ig model.Ingress
	if err := db.WithContext(ctx).First(&ig, id).Error; err != nil {
		return model.Ingress{}, err
	}
	return ig, nil
}

func ListIngresses(ctx context.Context, db *gorm.DB, edgeServerID *uint) ([]model.Ingress, error) {
	q := db.WithContext(ctx).Order("id desc")
	if edgeServerID != nil {
		q = q.Where("edge_server_id = ?", *edgeServerID)
	}
	var out []model.Ingress
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListIngressesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.Ingress, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Ingress
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func FindIngressByEdgeAndDomain(ctx context.Context, db *gorm.DB, edgeID uint, domain string) (model.Ingress, error) {
	var ig model.Ingress
	if err := db.WithContext(ctx).Where("edge_server_id = ? AND domain = ?", edgeID, domain).First(&ig).Error; err != nil {
		return model.Ingress{}, err
	}
	return ig, nil
}

func CreateIngress(ctx context.Context, db *gorm.DB, ig *model.Ingress) error {
	return db.WithContext(ctx).Create(ig).Error
}

func SaveIngress(ctx context.Context, db *gorm.DB, ig *model.Ingress) error {
	return db.WithContext(ctx).Save(ig).Error
}

func UpdateIngressFields(ctx context.Context, db *gorm.DB, id uint, updates map[string]any) error {
	return db.WithContext(ctx).Model(&model.Ingress{}).Where("id = ?", id).Updates(updates).Error
}

func MarkIngressPending(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&model.Ingress{}).Where("id = ?", id).Update("status", "pending").Error
}

func DeleteIngressCascade(ctx context.Context, db *gorm.DB, ingressID uint) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ingress_id = ?", ingressID).Delete(&model.IngressRoute{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Ingress{}, ingressID).Error; err != nil {
			return err
		}
		return nil
	})
}

func ListRoutesByIngressID(ctx context.Context, db *gorm.DB, ingressID uint) ([]model.IngressRoute, error) {
	var out []model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id = ?", ingressID).Order("sort asc, id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func ListRoutesByIngressIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]model.IngressRoute, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id IN ?", ids).Order("ingress_id asc, sort asc, id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func GetRouteByID(ctx context.Context, db *gorm.DB, ingressID, routeID uint) (model.IngressRoute, error) {
	var r model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id = ? AND id = ?", ingressID, routeID).First(&r).Error; err != nil {
		return model.IngressRoute{}, err
	}
	return r, nil
}

func CreateRoute(ctx context.Context, db *gorm.DB, r *model.IngressRoute) error {
	return db.WithContext(ctx).Create(r).Error
}

func SaveRoute(ctx context.Context, db *gorm.DB, r *model.IngressRoute) error {
	return db.WithContext(ctx).Save(r).Error
}

func DeleteRoute(ctx context.Context, db *gorm.DB, ingressID, routeID uint) error {
	return db.WithContext(ctx).Where("ingress_id = ? AND id = ?", ingressID, routeID).Delete(&model.IngressRoute{}).Error
}

// CountRouteConflicts 检查同 ingress 下是否已存在相同 path 的路由。
// excludeRouteID > 0 时排除自身（update 场景）。
func CountRouteConflicts(ctx context.Context, db *gorm.DB, ingressID uint, path string, excludeRouteID uint) (int64, error) {
	q := db.WithContext(ctx).Model(&model.IngressRoute{}).
		Where("ingress_id = ? AND path = ?", ingressID, path)
	if excludeRouteID > 0 {
		q = q.Where("id <> ?", excludeRouteID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// CountStreamPortConflicts 检查同 edge 下是否已存在相同 listen_port 的 tcp/udp 路由。
// excludeRouteID > 0 时排除自身（update 场景）。
func CountStreamPortConflicts(ctx context.Context, db *gorm.DB, edgeServerID uint, listenPort int, excludeRouteID uint) (int64, error) {
	var siblingIDs []uint
	if err := db.WithContext(ctx).Model(&model.Ingress{}).
		Where("edge_server_id = ?", edgeServerID).
		Pluck("id", &siblingIDs).Error; err != nil {
		return 0, err
	}
	if len(siblingIDs) == 0 {
		return 0, nil
	}
	q := db.WithContext(ctx).Model(&model.IngressRoute{}).
		Where("ingress_id IN ?", siblingIDs).
		Where("protocol IN ?", []string{"tcp", "udp"}).
		Where("listen_port = ?", listenPort)
	if excludeRouteID > 0 {
		q = q.Where("id <> ?", excludeRouteID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

func CreateIngressWithRoutes(ctx context.Context, db *gorm.DB, ig *model.Ingress, routes []model.IngressRoute) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ig).Error; err != nil {
			return err
		}
		for i := range routes {
			routes[i].IngressID = ig.ID
			if err := tx.Create(&routes[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
