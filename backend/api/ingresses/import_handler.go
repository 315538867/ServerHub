package ingresses

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
)

// importRunnerFactory 让单测覆盖 runner 获取——生产路径直接用 runner.For，
// 测试通过 SetImportRunnerFactory 注入桩来摸 fake config 内容。
type importRunnerFactory func(*model.Server, *config.Config) (runner.Runner, error)

var defaultImportRunnerFactory importRunnerFactory = runner.For

// SetImportRunnerFactory 仅供测试覆盖；返回旧值以便测试 t.Cleanup 恢复。
func SetImportRunnerFactory(f importRunnerFactory) importRunnerFactory {
	old := defaultImportRunnerFactory
	if f == nil {
		defaultImportRunnerFactory = runner.For
	} else {
		defaultImportRunnerFactory = f
	}
	return old
}

// RegisterImportRoutes 把 nginx → Ingress 接管相关的端点挂到 group。
//
// 当前只暴露一个 GET：扫描指定 edge 上 sites-available 内的反代 vhost，把
// 解析结果（ServerName / Listen / 一组 IngressRoute 候选）回吐给前端，由前端
// 走"新建 Ingress"模态完成最终落库——也就是说 import 端点本身不写 DB，落库
// 仍走现有 POST /ingresses（保证一切校验/事务路径与人工新建完全一致）。
func RegisterImportRoutes(group *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	group.GET("edges/:server_id/import-candidates", importCandidatesHandler(db, cfg))
}

// ImportCandidatesResp 是 GET 的响应载荷。Errors 一般为空，留口子让 runner
// 失败 / 单文件 cat 失败时把非致命错误带出来。
type ImportCandidatesResp struct {
	Candidates []discovery.IngressProxyCandidate `json:"candidates"`
	Errors     []string                          `json:"errors,omitempty"`
}

func importCandidatesHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		var s model.Server
		if err := db.First(&s, edgeID).Error; err != nil {
			resp.NotFound(c, "edge_server 不存在")
			return
		}
		rn, err := defaultImportRunnerFactory(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}
		cands, scanErr := discovery.ScanNginxIngressProxy(rn)
		out := ImportCandidatesResp{Candidates: cands}
		if scanErr != nil {
			out.Errors = append(out.Errors, scanErr.Error())
		}
		// AlreadyManaged：当前 edge 下已有同 domain 的 Ingress 视作"已接管"，
		// 前端据此把"导入"按钮置灰，避免重复落库导致 unique(edge_id, domain)
		// 冲突的 500。
		if len(cands) > 0 {
			var existing []string
			db.Model(&model.Ingress{}).
				Where("edge_server_id = ?", edgeID).
				Pluck("domain", &existing)
			known := make(map[string]struct{}, len(existing))
			for _, d := range existing {
				known[d] = struct{}{}
			}
			for i := range out.Candidates {
				if _, hit := known[out.Candidates[i].ServerName]; hit {
					out.Candidates[i].AlreadyManaged = true
				}
			}
		}
		resp.OK(c, out)
	}
}
