// Package ssl 暴露 nginx adapter 内部证书路径常量,供 api/ssl 等外部包使用。
//
// internal/render 不可被 adapter 外的代码直接 import(Go internal 规则),
// 因此把面向外部的 path helper 抽到 ssl/ 子包做 re-export。
package ssl

import (
	"github.com/serverhub/serverhub/adapters/ingress/nginx/internal/render"
)

// CertDir 是 nginx 落盘证书的根目录,与 internal/render.CertDir 同源。
const CertDir = render.CertDir

// CertCanonicalPaths 返回某域名在 CertDir 下的 fullchain / privkey 绝对路径。
// reconciler 写盘和 api/ssl 上传校验共用此 path 约定,保持单一事实源。
func CertCanonicalPaths(domain string) (cert, key string) {
	return render.CertCanonicalPaths(domain)
}
