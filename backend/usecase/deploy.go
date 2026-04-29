// Package usecase 是 v2 的应用编排层。
//
// deploy.go 实现 Release apply 主入口,替代 v1 pkg/deployer.ApplyRelease。
// 与 v1 行为差异:零(命令字节级一致,状态机推进与回滚语义保持)。
//
// 关键流程(单次 apply):
//  1. load svc/rel/art (domain.*)
//  2. svcType → adapter Kind 映射,runtime.MustGet 取 adapter
//  3. adapter.BuildStartCmd 拿到 start 段
//  4. cmdbuild.{BuildEnvPrefix, BuildConfigFilesPart, BuildFetchPart, WorkdirSetup, Assemble} 装配整段 bash
//  5. runner.For(srv) 执行
//  6. 成功:切 current_release_id + archive 老 active;失败 + AutoRollback + depth=0:递归 apply 上一条
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/core/runtime"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/internal/cmdbuild"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ApplyRelease 对指定 Service 应用指定 Release。
// triggerSource: manual | webhook | schedule | api | auto_rollback。
// onLine 流式日志回调,可为 nil。
//
// 失败 + Service.AutoRollbackOnFail=true + 存在上一条 active Release 时,
// 自动以 trigger_source=auto_rollback 再 Apply 一次,递归深度上限 1。
func ApplyRelease(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string)) (*domain.DeployRun, error) {
	return applyDepth(db, cfg, serviceID, releaseID, triggerSource, onLine, 0)
}

func applyDepth(db *gorm.DB, cfg *config.Config, serviceID, releaseID uint,
	triggerSource string, onLine func(string), depth int) (*domain.DeployRun, error) {

	ctx := context.Background()

	svc, err := repo.GetServiceByID(ctx, db, serviceID)
	if err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}
	rel, err := repo.GetReleaseByServiceAndID(ctx, db, serviceID, releaseID)
	if err != nil {
		return nil, fmt.Errorf("release not found: %w", err)
	}
	art, err := repo.GetArtifactByIDAndServiceID(ctx, db, rel.ArtifactID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}
	if art.Provider == domain.ArtifactProviderImported {
		return nil, errors.New("imported artifact is read-only and cannot be applied")
	}

	srv, err := repo.GetServerByID(ctx, db, svc.ServerID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	run := domain.DeployRun{
		ServiceID:     serviceID,
		ReleaseID:     releaseID,
		Status:        domain.DeployRunStatusRunning,
		TriggerSource: triggerSource,
		StartedAt:     time.Now(),
	}
	if err := repo.CreateDeployRun(ctx, db, &run); err != nil {
		return nil, err
	}

	cmd, buildErr := buildReleaseCmd(svc, rel, art, db, cfg.Security.AESKey)
	if buildErr != nil {
		finishDeployRun(db, &run, false, "build command failed: "+buildErr.Error())
		return &run, buildErr
	}
	if onLine != nil {
		onLine("$ " + cmd)
	}

	rn, err := runner.For(&srv, cfg)
	if err != nil {
		finishDeployRun(db, &run, false, "runner: "+err.Error())
		return &run, err
	}

	output, runErr := execStreaming(rn, cmd, onLine)
	success := runErr == nil
	finishDeployRun(db, &run, success, output)

	if success {
		if err := repo.ActivateRelease(ctx, db, serviceID, rel.ID); err != nil {
			return &run, fmt.Errorf("activate release: %w", err)
		}
		return &run, nil
	}

	if svc.AutoRollbackOnFail && depth < 1 {
		prevID := repo.FindPrevRelease(ctx, db, serviceID, rel.ID)
		if prevID > 0 {
			rbRun, _ := applyDepth(db, cfg, serviceID, prevID,
				domain.TriggerSourceAutoRollback, onLine, depth+1)
			if rbRun != nil {
				updates := map[string]any{
					"status":               domain.DeployRunStatusRolledBack,
					"rollback_from_run_id": &rbRun.ID,
				}
				_ = repo.UpdateDeployRunFields(ctx, db, run.ID, updates)
				run.Status = domain.DeployRunStatusRolledBack
				run.RollbackFromRunID = &rbRun.ID
			}
		}
	}
	return &run, runErr
}

// buildReleaseCmd 把 Release 三维 + Service.Type + Artifact.Provider 装成完整 bash 命令。
func buildReleaseCmd(svc domain.Service, rel domain.Release, art domain.Artifact,
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
func buildStartPart(svc domain.Service, rel domain.Release, art domain.Artifact) (string, error) {
	kind := mapServiceTypeToKind(svc.Type)
	if kind == "" {
		return "", fmt.Errorf("unsupported service type: %s", svc.Type)
	}
	adapter, err := runtime.Default.Get(kind)
	if err != nil {
		return "", err
	}
	r := &domain.Release{
		ID:               rel.ID,
		ServiceID:        rel.ServiceID,
		Label:            rel.Label,
		StartSpec:        rel.StartSpec,
		ArtifactProvider: art.Provider,
		ArtifactRef:      art.Ref,
	}
	return adapter.BuildStartCmd(&svc, r)
}

func mapServiceTypeToKind(t domain.ServiceType) string {
	switch t {
	case domain.ServiceTypeDocker:
		return string(domain.ServiceTypeDocker)
	case domain.ServiceTypeDockerCompose:
		return string(domain.ServiceTypeCompose)
	case domain.ServiceTypeNative:
		return string(domain.ServiceTypeNative)
	case domain.ServiceTypeStatic:
		return string(domain.ServiceTypeStatic)
	}
	return ""
}

func finishDeployRun(db *gorm.DB, run *domain.DeployRun, success bool, output string) {
	now := time.Now()
	dur := int(now.Sub(run.StartedAt).Seconds())
	status := domain.DeployRunStatusSuccess
	if !success {
		status = domain.DeployRunStatusFailed
	}
	updates := map[string]any{
		"status":       status,
		"finished_at":  &now,
		"duration_sec": dur,
		"output":       output,
	}
	_ = repo.UpdateDeployRunFields(context.Background(), db, run.ID, updates)
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
