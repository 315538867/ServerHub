// profile.go re-export internal/render 的 Profile 类型 + NormalizeProfile 函数。
//
// 设计动机:internal/render 受 Go internal/ 可见性限制不可被 adapters/ingress/nginx
// 之外的代码 import,但 api/nginx/profile_handler 需要 Profile 类型展示 + 提交规范化。
// 在已有的 profile/ 子包(放 ProbeNginxV)内顺手 re-export,职责对齐:profile/
// 子包 = "edge 上 nginx 形态的探测 + 表述"。
//
// 与 ssl/store.go 同构:都是"externally-visible facade over internal/render"。
package profile

import (
	"github.com/serverhub/serverhub/adapters/ingress/nginx/internal/render"
)

// Profile 是某台 edge 上 nginx 的具体部署形态(路径 / 命令串)。
// 与 internal/render.Profile 同型。
type Profile = render.Profile

// DefaultProfile 返回单 nginx + Debian 风格的默认 Profile,与 internal/render 同源。
func DefaultProfile() Profile { return render.DefaultProfile() }

// NormalizeProfile 把可能字段缺失的 Profile 规范化:空字段回退到 DefaultProfile()
// 对应字段。调用方通常在表单提交后跑一遍,保证后续 reconciler 用的 Profile 字段
// 全部非空。
func NormalizeProfile(p Profile) Profile { return render.NormalizeProfile(p) }
