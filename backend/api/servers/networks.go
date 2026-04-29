package servers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/repo"
)

func listNetworksHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		s, err := repo.GetServerByID(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "服务器不存在")
			return
		}
		out := s.Networks
		if out == nil {
			out = []domain.Network{}
		}
		resp.OK(c, gin.H{"networks": out})
	}
}

type updateNetworksReq struct {
	Networks []domain.Network `json:"networks"`
}

func updateNetworksHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			resp.BadRequest(c, "服务器 ID 无效")
			return
		}
		s, err := repo.GetServerByID(ctx, db, uint(id))
		if err != nil {
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
		filtered := make([]domain.Network, 0, len(req.Networks))
		for _, n := range req.Networks {
			if n.Kind == domain.NetworkKindLoopback {
				continue
			}
			filtered = append(filtered, n)
		}
		s.Networks = filtered
		if err := repo.SaveServer(ctx, db, &s); err != nil {
			resp.Fail(c, http.StatusBadRequest, 4001, "校验失败: "+err.Error())
			return
		}
		fresh, err := repo.GetServerByID(ctx, db, uint(id))
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"networks": fresh.Networks})
	}
}
