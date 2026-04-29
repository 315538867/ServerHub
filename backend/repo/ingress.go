package repo

import (
	"context"

	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

func GetIngressByID(ctx context.Context, db *gorm.DB, id uint) (domain.Ingress, error) {
	var ig model.Ingress
	if err := db.WithContext(ctx).First(&ig, id).Error; err != nil {
		return domain.Ingress{}, err
	}
	return model.ToDomainIngress(ig), nil
}

func ListIngresses(ctx context.Context, db *gorm.DB, edgeServerID *uint) ([]domain.Ingress, error) {
	q := db.WithContext(ctx).Order("id desc")
	if edgeServerID != nil {
		q = q.Where("edge_server_id = ?", *edgeServerID)
	}
	var out []model.Ingress
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Ingress, len(out))
	for i, ig := range out {
		result[i] = model.ToDomainIngress(ig)
	}
	return result, nil
}

func ListIngressesByIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.Ingress, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.Ingress
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Ingress, len(out))
	for i, ig := range out {
		result[i] = model.ToDomainIngress(ig)
	}
	return result, nil
}

func FindIngressByEdgeAndDomain(ctx context.Context, db *gorm.DB, edgeID uint, domainName string) (domain.Ingress, error) {
	var ig model.Ingress
	if err := db.WithContext(ctx).Where("edge_server_id = ? AND domain = ?", edgeID, domainName).First(&ig).Error; err != nil {
		return domain.Ingress{}, err
	}
	return model.ToDomainIngress(ig), nil
}

func CreateIngress(ctx context.Context, db *gorm.DB, ig *domain.Ingress) error {
	m := model.FromDomainIngress(*ig)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*ig = model.ToDomainIngress(m)
	return nil
}

func SaveIngress(ctx context.Context, db *gorm.DB, ig *domain.Ingress) error {
	m := model.FromDomainIngress(*ig)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*ig = model.ToDomainIngress(m)
	return nil
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

func ListAllRoutes(ctx context.Context, db *gorm.DB) ([]domain.IngressRoute, error) {
	var out []model.IngressRoute
	if err := db.WithContext(ctx).Order("ingress_id asc, sort asc, id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.IngressRoute, len(out))
	for i, r := range out {
		result[i] = model.ToDomainIngressRoute(r)
	}
	return result, nil
}

// ListIngressDomainsByEdgeID 返回指定 edge 上所有 Ingress 的 domain 列表。
func ListIngressDomainsByEdgeID(ctx context.Context, db *gorm.DB, edgeID uint) ([]string, error) {
	var out []string
	err := db.WithContext(ctx).Model(&model.Ingress{}).
		Where("edge_server_id = ?", edgeID).
		Pluck("domain", &out).Error
	return out, err
}

func ListRoutesByIngressID(ctx context.Context, db *gorm.DB, ingressID uint) ([]domain.IngressRoute, error) {
	var out []model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id = ?", ingressID).Order("sort asc, id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.IngressRoute, len(out))
	for i, r := range out {
		result[i] = model.ToDomainIngressRoute(r)
	}
	return result, nil
}

func ListRoutesByIngressIDs(ctx context.Context, db *gorm.DB, ids []uint) ([]domain.IngressRoute, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id IN ?", ids).Order("ingress_id asc, sort asc, id asc").Find(&out).Error; err != nil {
		return nil, err
	}
	result := make([]domain.IngressRoute, len(out))
	for i, r := range out {
		result[i] = model.ToDomainIngressRoute(r)
	}
	return result, nil
}

func GetRouteByID(ctx context.Context, db *gorm.DB, ingressID, routeID uint) (domain.IngressRoute, error) {
	var r model.IngressRoute
	if err := db.WithContext(ctx).Where("ingress_id = ? AND id = ?", ingressID, routeID).First(&r).Error; err != nil {
		return domain.IngressRoute{}, err
	}
	return model.ToDomainIngressRoute(r), nil
}

func CreateRoute(ctx context.Context, db *gorm.DB, r *domain.IngressRoute) error {
	m := model.FromDomainIngressRoute(*r)
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainIngressRoute(m)
	return nil
}

func SaveRoute(ctx context.Context, db *gorm.DB, r *domain.IngressRoute) error {
	m := model.FromDomainIngressRoute(*r)
	if err := db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	*r = model.ToDomainIngressRoute(m)
	return nil
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

func CreateIngressWithRoutes(ctx context.Context, db *gorm.DB, ig *domain.Ingress, routes []domain.IngressRoute) error {
	m := model.FromDomainIngress(*ig)
	modelRoutes := make([]model.IngressRoute, len(routes))
	for i, r := range routes {
		modelRoutes[i] = model.FromDomainIngressRoute(r)
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		for i := range modelRoutes {
			modelRoutes[i].IngressID = m.ID
			if err := tx.Create(&modelRoutes[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	*ig = model.ToDomainIngress(m)
	for i := range routes {
		routes[i] = model.ToDomainIngressRoute(modelRoutes[i])
	}
	return nil
}
