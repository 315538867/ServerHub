// Package accessurl 根据 Application 的 expose_mode 及关联的 Ingress 计算公网访问 URL。
package accessurl

import (
	"github.com/serverhub/serverhub/domain"
)

// Compute 根据 expose_mode 和 Ingress 列表计算公网访问 URL。
//
// 规则：
//   - expose_mode=none → 返回空字符串
//   - expose_mode=site → https://{domain}
//   - expose_mode=path → https://{domain}/{site_name}
//
// 优先从已 applied 的 Ingress 获取 domain；若无匹配 Ingress 则回退到 app.Domain / app.SiteName。
func Compute(ingresses []domain.Ingress, app domain.Application) string {
	if app.ExposeMode == "none" || app.ExposeMode == "" {
		return ""
	}

	// 1. 优先从已 applied 的 Ingress 获取 domain
	for _, ig := range ingresses {
		if ig.Status == "draft" {
			continue
		}
		if app.ExposeMode == "site" && ig.MatchKind == "domain" && ig.Domain != "" {
			return "https://" + ig.Domain
		}
		if app.ExposeMode == "path" && ig.MatchKind == "path" && ig.Domain != "" && app.SiteName != "" {
			return "https://" + ig.Domain + "/" + app.SiteName
		}
	}

	// 2. 回退：无匹配 Ingress 时从 app 自身字段计算
	if app.Domain != "" {
		if app.ExposeMode == "site" {
			return "https://" + app.Domain
		}
		if app.ExposeMode == "path" && app.SiteName != "" {
			return "https://" + app.Domain + "/" + app.SiteName
		}
	}

	return ""
}
