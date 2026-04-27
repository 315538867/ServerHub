// Package usecase: takeover.go 是接管入口,替代 v1 pkg/takeover.Run。
//
// 职责切分(R4 后):
//   - adapters/source/<kind> 的 Scanner.Takeover 只负责"远端文件/进程"侧 step 链
//   - 本文件负责: 全局校验、target 目录预检、查重、Application 绑定、
//     Service/EnvVarSet 写入、deploy_runs 收口
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/core/source"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/internal/stepkit"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// TakeoverRequest 是接管意图。AppMode 决定 Service 与 Application 的绑定策略
// (与 v1 takeover.Request 字段语义一致):
//   - "floating"(空) ApplicationID=nil, 不 upsert App
//   - "existing"     AppID 必填, App 必须存在且 ServerID 匹配
//   - "new"          AppName 必填, 全局 unique, 同名拒绝
type TakeoverRequest struct {
	Cand       source.Candidate `json:"candidate"`
	TargetName string           `json:"target_name"`
	AppMode    string           `json:"app_mode,omitempty"`
	AppID      *uint            `json:"app_id,omitempty"`
	AppName    string           `json:"app_name,omitempty"`
}

// TakeoverResult: Output 始终包含完整流水(含 rollback);Success/RolledBack 互斥。
type TakeoverResult struct {
	ServiceID  uint   `json:"service_id,omitempty"`
	Success    bool   `json:"success"`
	RolledBack bool   `json:"rolled_back"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// RunTakeover 执行接管:
//  1. 校验 SvcName + 查 (server, kind, source_id) 重复
//  2. target 目录必须不存在(stepkit.EnsureAbsent)
//  3. 派发到 source.Default.Get(kind).Takeover —— 远端 step 链由 stepkit 自动回滚
//  4. 远端成功后写 Service + Application 绑定 + EnvVarSet + deploy_runs(takeover trigger)
//
// 远端步骤失败 → RolledBack=true,DB 不动。
// DB 步骤失败 → 远端文件已就位但落库失败,Output 含警告,Error 提示运维手动重试 import。
func RunTakeover(ctx context.Context, db *gorm.DB, cfg *config.Config,
	srv *model.Server, r infra.Runner, req TakeoverRequest) TakeoverResult {

	res := TakeoverResult{}
	log := &stepkit.Log{}

	if err := safeshell.ValidName(req.TargetName, 64); err != nil {
		res.Error = "target_name 非法: " + err.Error()
		res.Output = log.String()
		return res
	}

	sc, err := source.Default.Get(req.Cand.Kind)
	if err != nil {
		res.Error = err.Error()
		res.Output = log.String()
		return res
	}

	log.Printf("=== 接管开始 ===\n")
	log.Printf("kind=%s source_id=%s target=%s\n",
		req.Cand.Kind, req.Cand.SourceID, req.TargetName)
	log.Printf("server=%s host=%s\n", srv.Name, srv.Host)

	target := stepkit.TargetDir(req.TargetName)
	if err := stepkit.EnsureAbsent(ctx, r, target); err != nil {
		res.Error = err.Error()
		res.Output = log.String()
		return res
	}

	var existing model.Service
	q := db.Where("server_id = ? AND source_kind = ? AND source_id = ?",
		srv.ID, req.Cand.Kind, req.Cand.SourceID).First(&existing)
	if q.Error == nil {
		res.Error = fmt.Sprintf("该服务已存在对应 Service(id=%d, name=%s)", existing.ID, existing.Name)
		res.Output = log.String()
		return res
	}

	tc := source.TakeoverContext{
		Server:  &domain.Server{ID: srv.ID, Name: srv.Name, Host: srv.Host, Port: srv.Port, Username: srv.Username},
		Runner:  r,
		Cand:    req.Cand,
		SvcName: req.TargetName,
	}
	if err := sc.Takeover(ctx, tc); err != nil {
		// stepkit 内部已尝试自动回滚,失败信息已写进 adapter 自己的 log。
		// 这里把 err 透传,运维需要看 server 端日志才能拿到完整 step 流水。
		res.Error = err.Error()
		res.RolledBack = true
		log.Printf("\n=== 接管失败(已尝试回滚)===\n%v\n", err)
		res.Output = log.String()
		return res
	}

	// === 远端 OK,落库 ===
	svcType := req.Cand.Suggested.Type
	if svcType == "" {
		svcType = model.ServiceTypeNative
	}
	svc := model.Service{
		Name:              req.TargetName,
		ServerID:          srv.ID,
		Type:              svcType,
		WorkDir:           target,
		ExposedPort:       firstPortInt(req.Cand.Suggested.Ports),
		SourceKind:        req.Cand.Kind,
		SourceID:          req.Cand.SourceID,
		SourceFingerprint: sc.Fingerprint(req.Cand),
		SyncStatus:        "synced",
	}
	appID, err := attachToApplication(db, &svc, req)
	if err != nil {
		res.Error = "application 绑定失败: " + err.Error()
		log.Printf("⚠ %s\n", res.Error)
		res.Output = log.String()
		return res
	}
	if err := db.Create(&svc).Error; err != nil {
		res.Error = "DB 写入失败: " + err.Error()
		log.Printf("⚠ Service 写入失败(主机已迁移完成):%v\n", err)
		res.Output = log.String()
		return res
	}
	if appID > 0 {
		finalizeApplicationLink(db, appID, svc.ID)
	}
	if envJSON, ok := encodeImportedEnv(req.Cand); ok && cfg.Security.AESKey != "" {
		if enc, encErr := crypto.Encrypt(envJSON, cfg.Security.AESKey); encErr == nil {
			_ = db.Create(&model.EnvVarSet{
				ServiceID: svc.ID,
				Label:     "imported",
				Content:   enc,
			}).Error
		} else {
			log.Printf("⚠ env-set 加密失败: %v\n", encErr)
		}
	}

	// 注:接管事件不写 deploy_runs —— DeployRun 的 INV-8 强制 ReleaseID 非空,
	// 而接管阶段尚未有 Release(用户后续在 Releases Tab 创建首个 Release 时
	// 才会有 release_id)。审计转由 audit 中间件在 api/discovery 写入。

	log.Printf("Service 已创建: id=%d name=%s\n", svc.ID, svc.Name)
	log.Printf("\n=== 接管成功 ===\n")
	res.ServiceID = svc.ID
	res.Success = true
	res.Output = log.String()
	return res
}

// attachToApplication 在 Service 落库前按 AppMode 决定归属。返回的 appID > 0
// 时,Service 写入后 caller 应调 finalizeApplicationLink。
func attachToApplication(db *gorm.DB, svc *model.Service, req TakeoverRequest) (uint, error) {
	mode := req.AppMode
	if mode == "" {
		mode = "floating"
	}
	switch mode {
	case "floating":
		svc.ApplicationID = nil
		return 0, nil
	case "existing":
		if req.AppID == nil || *req.AppID == 0 {
			return 0, errors.New("app_mode=existing 需要提供 app_id")
		}
		var app model.Application
		if err := db.First(&app, *req.AppID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, fmt.Errorf("app_id=%d 不存在", *req.AppID)
			}
			return 0, err
		}
		if app.ServerID != svc.ServerID {
			return 0, fmt.Errorf("app_id=%d 绑定在 server_id=%d, 与当前 server_id=%d 不符",
				app.ID, app.ServerID, svc.ServerID)
		}
		appID := app.ID
		svc.ApplicationID = &appID
		return appID, nil
	case "new":
		name := req.AppName
		if name == "" {
			name = req.TargetName
		}
		if err := safeshell.ValidName(name, 64); err != nil {
			return 0, fmt.Errorf("app_name 非法: %w", err)
		}
		var existing model.Application
		err := db.Where("name = ?", name).First(&existing).Error
		if err == nil {
			return 0, fmt.Errorf("应用 %q 已存在(server_id=%d), 请用 existing 模式绑定",
				name, existing.ServerID)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		app := model.Application{Name: name, ServerID: svc.ServerID}
		if e := db.Create(&app).Error; e != nil {
			return 0, e
		}
		appID := app.ID
		svc.ApplicationID = &appID
		return appID, nil
	default:
		return 0, fmt.Errorf("未知 app_mode=%q (可选 floating|existing|new)", mode)
	}
}

func finalizeApplicationLink(db *gorm.DB, appID, serviceID uint) {
	db.Model(&model.Application{}).
		Where("id = ? AND primary_service_id IS NULL", appID).
		Update("primary_service_id", serviceID)
}

func firstPortInt(ports []string) int {
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 取冒号前的数字(host:container 形如 "8080:80" → 8080),否则整段试 atoi。
		if i := strings.IndexByte(p, ':'); i >= 0 {
			p = p[:i]
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return 0
}
