package ingresses

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/serverhub/serverhub/pkg/nginxpresets"
	"github.com/serverhub/serverhub/pkg/resp"
)

// RegisterPresetRoutes 暴露 IngressRoute.Extra 模板渲染端点。
//
// 端点不写库——只把前端表单参数 → nginx 指令片段文本回吐,前端再把结果
// 填到 Extra textarea。设计取舍:
//   - 不在 IngressRoute 上加 PresetMeta 字段,因为预设产物一旦让用户编辑过
//     就无法可靠回写,弱保留也只是技术债;
//   - 不在 server 端把预设直接写进路由,保持"预设是 UI 便利层"的语义;
//   - 渲染逻辑放在 nginxpresets 包,这里只做参数 binding + 错误转 400。
func RegisterPresetRoutes(group *gin.RouterGroup, _ *gorm.DB) {
	group.POST("presets/render", presetsRenderHandler())
}

// presetReq 用 RawMessage 装 params,具体形状由 kind 决定——比起为每种 kind
// 写一个带 oneOf 的联合体,RawMessage + 二次 unmarshal 更直白也更易扩展。
type presetReq struct {
	Kind   nginxpresets.Kind `json:"kind" binding:"required"`
	Params json.RawMessage   `json:"params"`
}

type presetResp struct {
	Extra string `json:"extra"`
}

func presetsRenderHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req presetReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		extra, err := renderPreset(req.Kind, req.Params)
		if err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		resp.OK(c, presetResp{Extra: extra})
	}
}

// renderPreset 派发到 nginxpresets.BuildXxx;集中放这里方便单测覆盖三条分支。
func renderPreset(kind nginxpresets.Kind, params json.RawMessage) (string, error) {
	switch kind {
	case nginxpresets.KindRateLimit:
		var opts nginxpresets.RateLimitOpts
		if err := unmarshalParams(params, &opts); err != nil {
			return "", err
		}
		return nginxpresets.BuildRateLimit(opts)
	case nginxpresets.KindCache:
		var opts nginxpresets.CacheOpts
		if err := unmarshalParams(params, &opts); err != nil {
			return "", err
		}
		return nginxpresets.BuildCache(opts)
	case nginxpresets.KindSecurity:
		var opts nginxpresets.SecurityOpts
		if err := unmarshalParams(params, &opts); err != nil {
			return "", err
		}
		return nginxpresets.BuildSecurity(opts)
	default:
		return "", errors.New("kind 未知,允许 ratelimit / cache / security")
	}
}

// unmarshalParams 把空 params 当成 "{}",这样调用者传 {kind:"security"} 也能
// 走到 BuildXxx 的"没启用任何项"分支拿到统一的错误信息(而不是 binding 报 nil)。
func unmarshalParams(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	return json.Unmarshal(raw, dst)
}
