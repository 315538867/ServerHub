package servers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/resp"
	"gorm.io/gorm"
)

// listNetworksHandler 返回 server 的 Networks 列表（含 AfterFind 自动补的 loopback）。
func listNetworksHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		var s model.Server
		if err := db.First(&s, id).Error; err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		// AfterFind 已注入 loopback。
		out := s.Networks
		if out == nil {
			out = model.Networks{}
		}
		resp.OK(c, gin.H{"networks": out})
	}
}

// updateNetworksReq 整体替换语义：前端传完整 Networks 数组，后端覆盖。
// loopback 行由 BeforeSave 钩子兜底注入，不要求前端必须带。
type updateNetworksReq struct {
	Networks []model.Network `json:"networks"`
}

// updateNetworksHandler 整体替换 Server.Networks。
//
// 校验、去重、loopback 注入、默认 priority 全部由 model.Server.BeforeSave 完成，
// 这里只做：1) 拒绝用户上送 loopback（避免 NetworkID/Address 漂移）；
// 2) 校验数量上限避免 JSON 列爆炸。
func updateNetworksHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		var s model.Server
		if err := db.First(&s, id).Error; err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		var req updateNetworksReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		if len(req.Networks) > 32 {
			resp.BadRequest(c, "networks 条目过多（>32）")
			return
		}
		// 过滤 loopback：钩子会重新注入，前端不应直接编辑
		filtered := make(model.Networks, 0, len(req.Networks))
		for _, n := range req.Networks {
			if n.Kind == model.NetworkKindLoopback {
				continue
			}
			filtered = append(filtered, n)
		}
		s.Networks = filtered
		// Save 触发 BeforeSave 钩子做 Validate / 去重 / 注入 loopback
		if err := db.Save(&s).Error; err != nil {
			resp.Fail(c, http.StatusBadRequest, 4001, "校验失败: "+err.Error())
			return
		}
		// 重新读一次，让 AfterFind 给到完整 Networks
		var fresh model.Server
		db.First(&fresh, id)
		resp.OK(c, gin.H{"networks": fresh.Networks})
	}
}
