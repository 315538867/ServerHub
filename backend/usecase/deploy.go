// Package usecase 是 v2 的应用编排层。
//
// deploy.go 实现 Release apply 主入口,替代 v1 pkg/deployer.ApplyRelease。
// 与 v1 行为差异:零(命令字节级一致,状态机推进与回滚语义保持)。
//
// 关键流程(单次 apply):
//  1. load svc/rel/art (model.*)
//  2. model→domain 转换
//  3. svcType → adapter Kind 映射,runtime.MustGet 取 adapter
//  4. adapter.BuildStartCmd 拿到 start 段
//  5. cmdbuild.{BuildEnvPrefix, BuildConfigFilesPart, BuildFetchPart, WorkdirSetup, Assemble} 装配整段 bash
//  6. runner.For(srv) 执行
//  7. 成功:切 current_release_id + archive 老 active;失败 + AutoRollback + depth=0:递归 apply 上一条
package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/internal/cmdbuild"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/runner"
	"gorm.io/gorm"
)

// ApplyRelease 对指定 Service 应用指定 Release。
// triggerSource: manual | webhook | schedule | api | auto_rollback。
// onLine 流式日志回调,可为 nil。
//
// 失败 + Service.AutoRollbackOnFail=true + 存在上一条 active Release 时,
// 自动以 trigger_source=auto_rollback 再 Apply 一次,递归深度上限 1。
func ApplyRelease(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string)) (*model.DeployRun, error) {
	return applyDepth(db, cfg, serviceID, releaseID, triggerSource, onLine, 0)
}

func applyDepth(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string), depth int) (*model.DeployRun, error) {

	var svc model.Service
	if err := db.First(&svc, serviceID).Error; err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}
	var rel model.Release
	if err := db.Where("id = ? AND service_id = ?", releaseID, serviceID).First(&rel).Error; err != nil {
		return nil, fmt.Errorf("release not found: %w", err)
	}
	var art model.Artifact
	if err := db.Where("id = ? AND service_id = ?", rel.ArtifactID, serviceID).First(&art).Error; err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}
	if art.Provider == model.ArtifactProviderImported {
		return nil, errors.New("imported artifact is read-only and cannot be applied")
	}

	var srv model.Server
	if err := db.First(&srv, svc.ServerID).Error; err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	run := model.DeployRun{
		ServiceID:     serviceID,
		ReleaseID:     releaseID,
		Status:        model.DeployRunStatusRunning,
		TriggerSource: triggerSource,
		StartedAt:     time.Now(),
	}
	if err := db.Create(&run).Error; err != nil {
		return nil, err
	}

	cmd, buildErr := buildReleaseCmd(svc, rel, art, db, cfg.Security.AESKey)
	if buildErr != nil {
		finishRun(db, &run, false, "build command failed: "+buildErr.Error())
		return &run, buildErr
	}
	if onLine != nil {
		onLine("$ " + cmd)
	}

	rn, err := runner.For(&srv, cfg)
	if err != nil {
		finishRun(db, &run, false, "runner: "+err.Error())
		return &run, err
	}

	output, runErr := execStreaming(rn, cmd, onLine)
	success := runErr == nil
	finishRun(db, &run, success, output)

	if success {
		db.Model(&svc).Update("current_release_id", rel.ID)
		db.Model(&model.Release{}).
			Where("service_id = ? AND id <> ? AND status = ?", serviceID, rel.ID, model.ReleaseStatusActive).
			Update("status", model.ReleaseStatusArchived)
		db.Model(&rel).Update("status", model.ReleaseStatusActive)
		return &run, nil
	}

	if svc.AutoRollbackOnFail && depth < 1 {
		prev := findPrevActiveRelease(db, serviceID, rel.ID)
		if prev != nil {
			rbRun, _ := applyDepth(db, cfg, serviceID, prev.ID,
				model.TriggerSourceAutoRollback, onLine, depth+1)
			if rbRun != nil {
				db.Model(&run).Updates(map[string]any{
					"status":               model.DeployRunStatusRolledBack,
					"rollback_from_run_id": &rbRun.ID,
				})
			}
		}
	}
	return &run, runErr
}

// buildReleaseCmd 把 Release 三维 + Service.Type + Artifact.Provider 装成完整 bash 命令。
// 与 v1 pkg/deployer.buildReleaseCmd 字节级等价。
func buildReleaseCmd(svc model.Service, rel model.Release, art model.Artifact,
	db *gorm.DB, aesKey string) (string, error) {

	envPrefix, err := cmdbuild.BuildEnvPrefix(db, rel.EnvSetID, aesKey)
	if err != nil {
		return "", err
	}

	parts := cmdbuild.WorkdirSetup(svc)

	configParts, err := cmdbuild.BuildConfigFilesPart(db, rel.ConfigSetID)
	if err != nil {
		return "", err
	}
	parts = append(parts, configParts...)

	fetchPart, err := cmdbuild.BuildFetchPart(art)
	if err != nil {
		return "", err
	}
	if fetchPart != "" {
		parts = append(parts, fetchPart)
	}

	startPart, err := buildStartPart(svc, rel, art)
	if err != nil {
		return "", err
	}
	if startPart != "" {
		parts = append(parts, startPart)
	}

	return cmdbuild.Assemble(envPrefix, parts...), nil
}

// buildStartPart 通过 runtime.Registry 路由到对应 adapter,委托其 BuildStartCmd。
//
// 模型类型 → adapter Kind 映射:
//
//	"docker"          → "docker"
//	"docker-compose"  → "compose"
//	"native"          → "native"
//	"static"          → "static"
func buildStartPart(svc model.Service, rel model.Release, art model.Artifact) (string, error) {
	kind := mapServiceTypeToKind(svc.Type)
	if kind == "" {
		return "", fmt.Errorf("unsupported service type: %s", svc.Type)
	}
	adapter, err := runtime.Default.Get(kind)
	if err != nil {
		return "", err
	}
	svcDom := toDomainService(svc)
	relDom := toDomainRelease(rel, art)
	return adapter.BuildStartCmd(svcDom, relDom)
}

func mapServiceTypeToKind(t string) string {
	switch t {
	case model.ServiceTypeDocker:
		return string(domain.ServiceTypeDocker)
	case model.ServiceTypeDockerCompose:
		return string(domain.ServiceTypeCompose)
	case model.ServiceTypeNative:
		return string(domain.ServiceTypeNative)
	case model.ServiceTypeStatic:
		return string(domain.ServiceTypeStatic)
	}
	return ""
}

func toDomainService(s model.Service) *domain.Service {
	return &domain.Service{
		ID:                 s.ID,
		Name:               s.Name,
		Type:               domain.ServiceType(mapServiceTypeToKind(s.Type)),
		ServerID:           s.ServerID,
		WorkDir:            s.WorkDir,
		AutoRollbackOnFail: s.AutoRollbackOnFail,
		CurrentReleaseID:   s.CurrentReleaseID,
	}
}

func toDomainRelease(r model.Release, art model.Artifact) *domain.Release {
	return &domain.Release{
		ID:               r.ID,
		ServiceID:        r.ServiceID,
		Version:          r.Label,
		StartSpec:        r.StartSpec,
		ArtifactProvider: art.Provider,
		ArtifactRef:      art.Ref,
	}
}

func findPrevActiveRelease(db *gorm.DB, serviceID, excludeID uint) *model.Release {
	var prev model.Release
	err := db.Where("service_id = ? AND id <> ? AND status IN ?",
		serviceID, excludeID, []string{model.ReleaseStatusActive, model.ReleaseStatusArchived}).
		Order("id desc").First(&prev).Error
	if err != nil {
		return nil
	}
	return &prev
}

func finishRun(db *gorm.DB, run *model.DeployRun, success bool, output string) {
	now := time.Now()
	dur := int(now.Sub(run.StartedAt).Seconds())
	status := model.DeployRunStatusSuccess
	if !success {
		status = model.DeployRunStatusFailed
	}
	db.Model(run).Updates(map[string]any{
		"status":       status,
		"finished_at":  &now,
		"duration_sec": dur,
		"output":       output,
	})
	run.Status = status
	run.FinishedAt = &now
	run.DurationSec = dur
	run.Output = output
}

func execStreaming(rn runner.Runner, cmd string, onLine func(string)) (string, error) {
	out, err := rn.Run(cmd)
	if onLine != nil && out != "" {
		for _, line := range strings.Split(out, "\n") {
			onLine(line)
		}
	}
	return out, err
}
