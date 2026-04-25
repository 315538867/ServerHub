package takeover

import (
	"fmt"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
	"gorm.io/gorm"
)

// Request is the operator's intent: take over this discovered service and
// surface it as a Service named TargetName under the standard apps directory.
//
// AppMode 决定接管完成后 Service 的归属策略：
//   - "floating"（默认）：Service 不绑定任何 Application（ApplicationID = nil）
//   - "existing"：绑定到已存在的 App（AppID 必填，且 ServerID 必须匹配）
//   - "new"：以 AppName 为名创建一条新 Application 并绑定
//
// 为向前兼容空字符串视为 "floating"（旧调用方只传 TargetName 的情况）。
type Request struct {
	Candidate  discovery.Candidate `json:"candidate"`
	TargetName string              `json:"target_name"`

	AppMode string `json:"app_mode,omitempty"`
	AppID   *uint  `json:"app_id,omitempty"`
	AppName string `json:"app_name,omitempty"`
}

// Result bundles the outcome of a takeover attempt. Output always contains the
// full step-by-step transcript (forward + any rollback) so the UI can show one
// log regardless of success/failure. Success and RolledBack are mutually
// exclusive — RolledBack=true means the system was restored to its pre-takeover
// state; if rollback itself failed, Success and RolledBack are both false and
// the log will name backup paths the operator should inspect.
type Result struct {
	DeployID   uint   `json:"deploy_id,omitempty"`
	Success    bool   `json:"success"`
	RolledBack bool   `json:"rolled_back"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// Run executes the takeover plan for req on server. It dispatches by
// Candidate.Kind to the per-kind runner; the runners do the actual work via
// Step/RunSteps so failure rollback is uniform. This function never panics —
// any error is returned via Result.Error with the log preserved in Output.
func Run(db *gorm.DB, cfg *config.Config, server model.Server, req Request) Result {
	log := &Log{}
	res := Result{}

	if err := safeshell.ValidName(req.TargetName, 64); err != nil {
		res.Error = "target_name 非法: " + err.Error()
		res.Output = log.String()
		return res
	}

	rn, err := runner.For(&server, cfg)
	if err != nil {
		res.Error = "runner 初始化失败: " + err.Error()
		res.Output = log.String()
		return res
	}

	log.Printf("=== 接管开始 ===\n")
	log.Printf("kind=%s source_id=%s target=%s\n",
		req.Candidate.Kind, req.Candidate.SourceID, req.TargetName)
	log.Printf("server=%s host=%s\n", server.Name, server.Host)

	// Pre-flight that's universal across kinds: target dir must not exist.
	target := TargetDir(req.TargetName)
	if err := EnsureAbsent(rn, target); err != nil {
		res.Error = err.Error()
		res.Output = log.String()
		return res
	}

	// Existing Deploy with same (server, kind, source_id) means the candidate
	// was already imported via the passive flow — refuse to double-take-over.
	var existing model.Service
	q := db.Where("server_id = ? AND source_kind = ? AND source_id = ?",
		server.ID, req.Candidate.Kind, req.Candidate.SourceID).First(&existing)
	if q.Error == nil {
		res.Error = fmt.Sprintf("该服务已存在对应 Deploy（id=%d, name=%s）", existing.ID, existing.Name)
		res.Output = log.String()
		return res
	}

	switch req.Candidate.Kind {
	case discovery.KindNginx:
		err = runStatic(db, rn, log, server, req, &res)
	case discovery.KindCompose:
		err = runCompose(db, rn, log, server, req, &res)
	case discovery.KindDocker:
		err = runDocker(db, rn, log, server, req, &res)
	case discovery.KindSystemd:
		err = runSystemd(db, rn, log, server, req, &res)
	default:
		err = fmt.Errorf("不支持的 kind: %s", req.Candidate.Kind)
	}

	if err != nil {
		res.Error = err.Error()
		// runStatic decides whether RolledBack flips; default false here.
		log.Printf("\n=== 接管失败 ===\n%v\n", err)
	} else {
		res.Success = true
		log.Printf("\n=== 接管成功 ===\n")
	}
	res.Output = log.String()
	return res
}
