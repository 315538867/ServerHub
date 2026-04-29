// Package usecase: application.go 封装 Application CRUD、Service 挂载/卸载、
// ingress 反向视图等业务逻辑。
//
// handler 只负责 DTO 解析 / 鉴权 / 调 usecase / 回响应四件事。
// derive.AppStatus 仍在 derive 包内执行,usecase 透传 db 给它。
//
// TODO R7: 切 ports interface,移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/serverhub/serverhub/derive"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ── 入参 / 出参 ─────────────────────────────────────────────────────────────

// AppWithStatus 是 Application + 派生状态。
type AppWithStatus struct {
	domain.Application
	Status string `json:"status"`
}

// AppIngressDTO 是 application 反向 ingress 视图的返回结构。
type AppIngressDTO struct {
	domain.Ingress
	EdgeServerName string                `json:"edge_server_name"`
	MatchingRoutes []domain.IngressRoute `json:"matching_routes"`
}

// ── list / get ──────────────────────────────────────────────────────────────

// ListApplications 列出应用,可按 serverID 过滤,并附带派生状态。
func ListApplications(ctx context.Context, db *gorm.DB, serverID *uint) ([]AppWithStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	apps, err := repo.ListApplications(ctx, db, serverID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	statusMap := derive.AppStatus(db, ids)
	out := make([]AppWithStatus, len(apps))
	for i, a := range apps {
		out[i] = AppWithStatus{Application: a, Status: statusMap[a.ID].Result}
	}
	return out, nil
}

// GetApplication 返回单个应用及派生状态。
func GetApplication(ctx context.Context, db *gorm.DB, id uint) (AppWithStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	app, err := repo.GetApplicationByID(ctx, db, id)
	if err != nil {
		return AppWithStatus{}, err
	}
	m := derive.AppStatus(db, []uint{app.ID})
	return AppWithStatus{Application: app, Status: m[app.ID].Result}, nil
}

// ── create / update / delete ────────────────────────────────────────────────

// CreateApplication 校验 server 存在性 + 创建应用。
func CreateApplication(ctx context.Context, db *gorm.DB, app *domain.Application) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err := repo.GetServerByID(ctx, db, app.ServerID); err != nil {
		return errors.New("服务器不存在")
	}
	return repo.CreateApplication(ctx, db, app)
}

// UpdateApplication 加载 + 保存应用,并返回带状态的结果。
func UpdateApplication(ctx context.Context, db *gorm.DB, app *domain.Application) (AppWithStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := repo.SaveApplication(ctx, db, app); err != nil {
		return AppWithStatus{}, err
	}
	m := derive.AppStatus(db, []uint{app.ID})
	return AppWithStatus{Application: *app, Status: m[app.ID].Result}, nil
}

// DeleteApplication 删除应用。
func DeleteApplication(ctx context.Context, db *gorm.DB, id uint) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return repo.DeleteApplication(ctx, db, id)
}

// ── service 挂载/卸载 ───────────────────────────────────────────────────────

// AttachService 把 service 挂到 application;若 app 没有主服务则自动设置。
func AttachService(ctx context.Context, db *gorm.DB, appID, serviceID uint) (domain.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	app, err := repo.GetApplicationByID(ctx, db, appID)
	if err != nil {
		return domain.Service{}, errors.New("应用不存在")
	}
	svc, err := repo.GetServiceByID(ctx, db, serviceID)
	if err != nil {
		return domain.Service{}, errors.New("服务不存在")
	}
	if svc.ServerID != app.ServerID {
		return domain.Service{}, errors.New("服务与应用不在同一服务器，不可挂载")
	}
	svc.ApplicationID = &appID
	if err := repo.SaveService(ctx, db, &svc); err != nil {
		return domain.Service{}, err
	}
	if app.PrimaryServiceID == nil {
		_ = repo.UpdatePrimaryService(ctx, db, appID, &svc.ID)
	}
	return svc, nil
}

// DetachService 把 service 从 application 卸载;若是主服务则清空。
func DetachService(ctx context.Context, db *gorm.DB, appID, serviceID uint) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	svc, err := repo.GetServiceByID(ctx, db, serviceID)
	if err != nil {
		return errors.New("服务不存在")
	}
	if svc.ApplicationID == nil || *svc.ApplicationID != appID {
		return errors.New("该服务未挂在此应用下")
	}
	svc.ApplicationID = nil
	if err := repo.SaveService(ctx, db, &svc); err != nil {
		return err
	}
	_ = repo.ClearPrimaryServiceIfMatch(ctx, db, appID, serviceID)
	return nil
}

// ── ingress 反向视图 ────────────────────────────────────────────────────────

// ListAppIngresses 返回引用了本 app 任一 Service 的所有 Ingress,
// 每条附带 EdgeServerName + 命中的子路由。
//
// SQL 预算：4 条（services + allRoutes + ingresses + servers）。
func ListAppIngresses(ctx context.Context, db *gorm.DB, appID uint) ([]AppIngressDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	services, err := repo.ListServicesByApplicationID(ctx, db, appID)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return []AppIngressDTO{}, nil
	}
	sidSet := make(map[uint]struct{}, len(services))
	for _, s := range services {
		sidSet[s.ID] = struct{}{}
	}

	// 全表扫路由,内存过滤 upstream.type=service && service_id ∈ sidSet
	routes, err := repo.ListAllRoutes(ctx, db)
	if err != nil {
		return nil, err
	}
	routesByIngress := map[uint][]domain.IngressRoute{}
	for _, rt := range routes {
		if rt.Upstream.Type != "service" || rt.Upstream.ServiceID == nil {
			continue
		}
		if _, ok := sidSet[*rt.Upstream.ServiceID]; !ok {
			continue
		}
		routesByIngress[rt.IngressID] = append(routesByIngress[rt.IngressID], rt)
	}
	if len(routesByIngress) == 0 {
		return []AppIngressDTO{}, nil
	}

	ingressIDs := make([]uint, 0, len(routesByIngress))
	for id := range routesByIngress {
		ingressIDs = append(ingressIDs, id)
	}
	ingresses, err := repo.ListIngressesByIDs(ctx, db, ingressIDs)
	if err != nil {
		return nil, err
	}

	serverIDSet := map[uint]struct{}{}
	for _, ig := range ingresses {
		serverIDSet[ig.EdgeServerID] = struct{}{}
	}
	nameByID := map[uint]string{}
	if len(serverIDSet) > 0 {
		sids := make([]uint, 0, len(serverIDSet))
		for id := range serverIDSet {
			sids = append(sids, id)
		}
		servers, err := repo.ListServersByIDs(ctx, db, sids)
		if err == nil {
			for _, s := range servers {
				nameByID[s.ID] = s.Name
			}
		}
	}

	out := make([]AppIngressDTO, 0, len(ingresses))
	for _, ig := range ingresses {
		out = append(out, AppIngressDTO{
			Ingress:        ig,
			EdgeServerName: nameByID[ig.EdgeServerID],
			MatchingRoutes: routesByIngress[ig.ID],
		})
	}
	return out, nil
}
