package ingresses

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/discovery"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
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
	// Phase Nginx-P3D: 确认导入并归档原文件 / 还原归档
	group.POST("edges/:server_id/import-confirm", importConfirmHandler(db, cfg))
	group.POST(":id/restore", restoreHandler(db, cfg))
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
		// 跨机 upstream 标记:扫描出来的 proxy_pass 主机,如果命中**另一台**已注册
		// Server 的 Host / Networks[].Address,就把 server id+name 回吐给前端,让
		// 用户在导入时一眼看见"这条 proxy_pass 指向另一台机,接管后建议切到 service
		// upstream 让 Resolver 自动选最优网络"。同 edge 自身的 loopback / private
		// 命中**不**视作跨机,避免对"normal 接管自家进程"打扰。
		if len(cands) > 0 {
			annotateCrossServer(out.Candidates, db, edgeID)
		}
		resp.OK(c, out)
	}
}

// annotateCrossServer 把跨机 proxy_pass 标记到 candidates 的 routes 上。
//
// 实现:把 db 里所有 Server 的 Host + Networks[].Address 摊成 host→(id,name) 表,
// 然后对每条 route 取 ProxyPassHost 查表。命中**当前 edge** 的不算跨机。
func annotateCrossServer(cands []discovery.IngressProxyCandidate, db *gorm.DB, edgeID uint) {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		return // 跨机标记是 best-effort,DB 抖一下不影响主流程
	}
	type srvRef struct {
		id   uint
		name string
	}
	lookup := make(map[string]srvRef, len(servers)*2)
	for _, s := range servers {
		ref := srvRef{id: s.ID, name: s.Name}
		if h := strings.TrimSpace(s.Host); h != "" {
			lookup[h] = ref
		}
		for _, n := range s.Networks {
			// loopback Address 永远是 127.0.0.1,不能用来跨机匹配——任何 edge 上
			// 看到 proxy_pass http://127.0.0.1 都是"自家进程"。
			if n.Kind == model.NetworkKindLoopback {
				continue
			}
			if a := strings.TrimSpace(n.Address); a != "" {
				// 多台机若共用同一非 loopback Address(配置错误,极罕见) 后写覆盖
				// 前写——跨机标记本就是 best-effort 提示,精确度让位于代码简单。
				lookup[a] = ref
			}
		}
	}
	for i := range cands {
		for j := range cands[i].Routes {
			host, ok := discovery.ProxyPassHost(cands[i].Routes[j].ProxyPass)
			if !ok {
				continue
			}
			ref, hit := lookup[host]
			if !hit || ref.id == edgeID {
				continue
			}
			cands[i].Routes[j].CrossServerID = ref.id
			cands[i].Routes[j].CrossServerName = ref.name
		}
	}
}

// ── Phase Nginx-P3D: import-confirm（归档 + 落库）/ restore（还原） ─────────

// importConfirmReq 是「确认导入」一次的入参——前端把扫描结果原样回吐,
// 加上 server_id 路径段定位 edge。设计取舍:不复用 createReq,因为
// 接管路径必须先 mv 原文件再写 DB,逻辑链路与"纯新建"不同。
type importConfirmReq struct {
	ConfigFile string                `json:"config_file" binding:"required"`
	ServerName string                `json:"server_name" binding:"required"`
	Listen     string                `json:"listen"`
	Routes     []importConfirmRoute  `json:"routes"`
}

type importConfirmRoute struct {
	Path      string `json:"path" binding:"required"`
	ProxyPass string `json:"proxy_pass" binding:"required"`
	WebSocket bool   `json:"websocket"`
	Extra     string `json:"extra"`
}

// 接管端点只允许处理这几个目录下的文件——nginx 标准布局,且与 discovery 扫描
// 的范围对齐。任何其它路径(/etc/passwd / /var/log/...)都会被拒绝,降低
// "前端伪造 config_file 让后端 mv 任意文件"的攻击面。
var approvedConfPrefixes = []string{
	"/etc/nginx/sites-enabled/",
	"/etc/nginx/sites-available/",
	"/etc/nginx/conf.d/",
}

func isApprovedNginxConfPath(p string) bool {
	if p == "" {
		return false
	}
	if !filepath.IsAbs(p) {
		return false
	}
	if filepath.Clean(p) != p {
		return false
	}
	// 防 .. 跨目录、防文件名带换行 / 反引号 / 引号导致 shell 二次解析
	if strings.ContainsAny(p, "\n\r\t\x00`$;&|<>*?\"'\\") {
		return false
	}
	base := filepath.Base(p)
	if base == "" || base == "." || base == ".." {
		return false
	}
	for _, pf := range approvedConfPrefixes {
		if strings.HasPrefix(p, pf) && len(p) > len(pf) {
			return true
		}
	}
	return false
}

// archiveTarget 计算"归档后的远端绝对路径"。
// 路径含时间戳目录是为了:
//   - 同一文件多次接管时不互相覆盖（理论上很难发生,但留这个逃生口便宜）
//   - 运维 ls 归档目录时能按时间排序
//
// 时间戳精确到纳秒避免 1 秒内连续接管两条 ingress 时撞名。
func archiveTarget(originalConfigPath string, now time.Time) string {
	dir := fmt.Sprintf("/etc/nginx/.serverhub-archive/%d", now.UnixNano())
	return filepath.Join(dir, filepath.Base(originalConfigPath))
}

func importConfirmHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		edgeID, ok := parseUintParam(c, "server_id")
		if !ok {
			return
		}
		var req importConfirmReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if !isApprovedNginxConfPath(req.ConfigFile) {
			resp.BadRequest(c, "config_file 必须落在 sites-enabled/sites-available/conf.d 下,且不含 shell 元字符")
			return
		}
		if err := safeshell.NginxValue(req.ServerName); err != nil {
			resp.BadRequest(c, "server_name 非法: "+err.Error())
			return
		}
		// 路由本身的 Extra 也要过 NginxBlock,与 routeReq 链路对齐。
		for i := range req.Routes {
			if err := safeshell.NginxBlock(req.Routes[i].Extra); err != nil {
				resp.BadRequest(c, fmt.Sprintf("route[%d].extra 非法: %s", i, err.Error()))
				return
			}
			if req.Routes[i].Path == "" {
				resp.BadRequest(c, fmt.Sprintf("route[%d].path 为空", i))
				return
			}
		}

		var s model.Server
		if err := db.First(&s, edgeID).Error; err != nil {
			resp.NotFound(c, "edge_server 不存在")
			return
		}

		// 同 edge 同 domain 已被接管 → 拒,避免 unique(edge_id, domain) 冲突 500。
		var dup int64
		db.Model(&model.Ingress{}).
			Where("edge_server_id = ? AND domain = ?", edgeID, req.ServerName).
			Count(&dup)
		if dup > 0 {
			resp.BadRequest(c, "该 server_name 已存在 Ingress")
			return
		}

		rn, err := defaultImportRunnerFactory(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}

		archPath := archiveTarget(req.ConfigFile, time.Now())
		archDir := filepath.Dir(archPath)
		// sudo 是因为 sites-enabled 通常 root:root,与现有 Apply 链路一致。
		mvCmd := fmt.Sprintf(
			"sudo -n mkdir -p %s && sudo -n mv %s %s",
			safeshell.Quote(archDir),
			safeshell.Quote(req.ConfigFile),
			safeshell.Quote(archPath),
		)
		if out, err := rn.Run(mvCmd); err != nil {
			resp.InternalError(c, "归档失败: "+err.Error()+" / "+out)
			return
		}

		// 落库失败时把文件 mv 回去——避免半状态(文件归档了但 DB 没有,用户只
		// 能去 ssh 手动还原)。回滚 mv 失败时只能记日志,继续把 5xx 报给前端。
		ig := model.Ingress{
			EdgeServerID:       edgeID,
			MatchKind:          "domain",
			Domain:             req.ServerName,
			DefaultPath:        "/",
			Status:             "pending",
			ArchivePath:        archPath,
			OriginalConfigPath: req.ConfigFile,
		}
		txErr := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&ig).Error; err != nil {
				return err
			}
			for idx, r := range req.Routes {
				ir := model.IngressRoute{
					IngressID: ig.ID,
					Sort:      idx * 10,
					Path:      r.Path,
					Protocol:  "http",
					Upstream:  model.IngressUpstream{Type: "raw", RawURL: r.ProxyPass},
					WebSocket: r.WebSocket,
					Extra:     r.Extra,
				}
				if err := tx.Create(&ir).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if txErr != nil {
			rollback := fmt.Sprintf(
				"sudo -n mv %s %s",
				safeshell.Quote(archPath),
				safeshell.Quote(req.ConfigFile),
			)
			_, _ = rn.Run(rollback)
			resp.InternalError(c, "落库失败,已尝试回滚归档: "+txErr.Error())
			return
		}
		resp.OK(c, ig)
	}
}

// restoreHandler 把"接管而来"的 Ingress 还原:
//   1. 把 ArchivePath 文件 mv 回 OriginalConfigPath
//   2. 删除 Ingress + Routes
//
// 注意:**不**自动调用 nginxops.Apply——还原后 .serverhub.d/<edge>/sites/
// 下本 Ingress 渲染产物会在下次 apply 时由 reconciler 检测到本地"应有"列表
// 缺失自动删除。让用户显式点应用配置,与现有"删除 Ingress"语义一致。
func restoreHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseUintParam(c, "id")
		if !ok {
			return
		}
		ig, err := loadIngress(db, id)
		if err != nil {
			resp.NotFound(c, "ingress 不存在")
			return
		}
		if ig.ArchivePath == "" || ig.OriginalConfigPath == "" {
			resp.BadRequest(c, "本 Ingress 不是接管来源,无 archive 可还原")
			return
		}
		// 防御:虽然写入时已验过 OriginalConfigPath,这里再走一次白名单——
		// DB 行可能被人为篡改,绝对不能 mv 到 /etc/passwd 之类的目录。
		if !isApprovedNginxConfPath(ig.OriginalConfigPath) {
			resp.InternalError(c, "Ingress.original_config_path 不在白名单,拒绝还原")
			return
		}
		var s model.Server
		if err := db.First(&s, ig.EdgeServerID).Error; err != nil {
			resp.InternalError(c, "edge_server 加载失败: "+err.Error())
			return
		}
		rn, err := defaultImportRunnerFactory(&s, cfg)
		if err != nil {
			resp.InternalError(c, "runner: "+err.Error())
			return
		}
		mvCmd := fmt.Sprintf(
			"sudo -n mv %s %s",
			safeshell.Quote(ig.ArchivePath),
			safeshell.Quote(ig.OriginalConfigPath),
		)
		if out, err := rn.Run(mvCmd); err != nil {
			resp.InternalError(c, "还原失败: "+err.Error()+" / "+out)
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("ingress_id = ?", id).Delete(&model.IngressRoute{}).Error; err != nil {
				return err
			}
			return tx.Delete(&model.Ingress{}, id).Error
		}); err != nil {
			// DB 删失败时归档文件已经 mv 回去了——状态变成"文件已还原但 DB
			// 还认 Ingress 存在",下次 apply 会渲染冲突 vhost。把错误暴露给
			// 用户,比静默回滚 mv 更安全(后者可能把用户刚还原的文件再 mv 走)。
			resp.InternalError(c, "已还原文件但 DB 删除失败,请人工介入: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"restored": true, "original_config_path": ig.OriginalConfigPath})
	}
}
