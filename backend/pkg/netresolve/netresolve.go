// Package netresolve 实现 Resolver 算法：根据 (edge, target, port, pref) 选出
// edge 视角下访问 target 的最优 upstream URL。
//
// 设计目标：跨机 nginx upstream 自动适配复杂网络（同机短路 / 内网 / VPN /
// 反向隧道 / 公网兜底），让用户在 IngressRoute 上勾选 Service 即可，不用
// 手填 IP。
//
// 核心约定：
//   - 内网/VPN/隧道 优先于 公网（成本/延迟/安全）
//   - 公网 永远是兜底，但永不被禁用
//   - 用户可显式 network_pref 强��走某条路径
//
// 不依赖 DB / runner / 网络 IO，纯函数，便于单测覆盖。
package netresolve

import (
	"errors"
	"fmt"
	"sort"

	"github.com/serverhub/serverhub/model"
)

// 网络偏好常量。空字符串视为 PrefAuto（向后兼容旧数据）。
const (
	PrefAuto     = "auto"
	PrefLoopback = "loopback"
	PrefPrivate  = "private"
	PrefVPN      = "vpn"
	PrefTunnel   = "tunnel"
	PrefPublic   = "public"
)

// Result 是 Resolve 的返回值，除最终 URL 外还附带选中的 Network 与人类可读
// 解释，便于 UI 在 apply 之前给用户预览「将解析为 X」。
type Result struct {
	URL             string
	SelectedNetwork model.Network
	Reason          string
}

// Resolve 是 Resolver 主入口。
//
//   - edge:         落地 nginx 的服务器
//   - target:       应用所在服务器
//   - port:         默认端口（被 overridePort 覆盖）
//   - pref:         网络偏好；空 / "auto" 等价
//   - overrideHost: 非空时直接用作最终 host
//   - overridePort: >0 时直接用作最终 port
//
// 返回错误的情形：
//   - target 没有任何符合 pref 的可达网络
//   - pref=auto 且 target 完全没有可达网络（甚至连 public 都没有）
func Resolve(
	edge, target *model.Server,
	port int,
	pref string,
	overrideHost string,
	overridePort int,
) (Result, error) {
	if edge == nil || target == nil {
		return Result{}, errors.New("netresolve: edge 和 target 都不能为空")
	}
	if pref == "" {
		pref = PrefAuto
	}

	// 1) 同机短路：edge==target 直接走 loopback
	if edge.ID == target.ID {
		host := overrideHost
		if host == "" {
			host = "127.0.0.1"
		}
		p := port
		if overridePort > 0 {
			p = overridePort
		}
		return Result{
			URL: buildURL(host, p),
			SelectedNetwork: model.Network{
				Kind:    model.NetworkKindLoopback,
				Address: "127.0.0.1",
				Label:   "同机短路",
			},
			Reason: "edge == target，使用 loopback",
		}, nil
	}

	// 2) 选候选
	var candidates []model.Network
	if pref == PrefAuto {
		candidates = candidatesAuto(edge, target)
	} else {
		candidates = candidatesByPref(target, pref, edge.ID, edge)
	}
	if len(candidates) == 0 {
		return Result{}, fmt.Errorf("netresolve: target=%d 没有 pref=%s 下的可达网络", target.ID, pref)
	}

	// 3) 排序：priority asc，同 priority 按 kind 顺序（private<vpn<tunnel<public）
	sortByPriorityThenKind(candidates)
	selected := candidates[0]

	// 4) 拼装 URL
	host := overrideHost
	if host == "" {
		host = selected.Address
	}
	p := port
	if overridePort > 0 {
		p = overridePort
	}
	return Result{
		URL:             buildURL(host, p),
		SelectedNetwork: selected,
		Reason:          reasonFor(selected, edge, target, pref),
	}, nil
}

// candidatesAuto 收集 auto 模式下所有可达候选。
//
// 规则：
//   - tunnel：edge.id 在 target.network.reachable_from 列表里
//   - private/vpn：edge 与 target 有同 NetworkID 的同 kind 条目
//   - public：永远算候选（priority 默认 100，自然落到最后）
//   - loopback：跳过（同机短路在 Resolve 第 1 步处理）
func candidatesAuto(edge, target *model.Server) []model.Network {
	out := make([]model.Network, 0, len(target.Networks))
	for _, n := range target.Networks {
		switch n.Kind {
		case model.NetworkKindLoopback:
			continue
		case model.NetworkKindTunnel:
			if canEdgeReachTunnel(n, edge.ID) {
				out = append(out, n)
			}
		case model.NetworkKindPrivate, model.NetworkKindVPN:
			if _, ok := shareNetworkID(edge, n.Kind, n.NetworkID); ok {
				out = append(out, n)
			}
		case model.NetworkKindPublic:
			out = append(out, n)
		}
	}
	return out
}

// candidatesByPref 收集指定 pref 下的候选。
//
// 显式 pref 时仍要做可达性校验：
//   - tunnel 必须 edge ∈ reachable_from
//   - private/vpn 必须 edge 有同 NetworkID
//   - public/loopback 无校验（loopback 走不到这里，pref=loopback 由
//     Resolve 第 1 步处理）
func candidatesByPref(target *model.Server, pref string, edgeID uint, edge *model.Server) []model.Network {
	out := make([]model.Network, 0)
	for _, n := range target.Networks {
		if n.Kind != pref {
			continue
		}
		switch pref {
		case model.NetworkKindTunnel:
			if !canEdgeReachTunnel(n, edgeID) {
				continue
			}
		case model.NetworkKindPrivate, model.NetworkKindVPN:
			if _, ok := shareNetworkID(edge, pref, n.NetworkID); !ok {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

// canEdgeReachTunnel 检查 edge 是否在 tunnel.reachable_from 列表里。
func canEdgeReachTunnel(n model.Network, edgeID uint) bool {
	for _, id := range n.ReachableFrom {
		if id == edgeID {
			return true
		}
	}
	return false
}

// shareNetworkID 检查 edge 是否有 kind+NetworkID 都匹配的网络。返回 edge 侧
// 匹配到的 Network（用于 reasonFor 解释，目前未使用但留作扩展）。
func shareNetworkID(edge *model.Server, kind, networkID string) (model.Network, bool) {
	if networkID == "" {
		return model.Network{}, false
	}
	for _, n := range edge.Networks {
		if n.Kind == kind && n.NetworkID == networkID {
			return n, true
		}
	}
	return model.Network{}, false
}

// kindRank 返回 kind 的排序权重：内网 < VPN < 隧道 < 公网。
// 同 priority 时按这个顺序破平。
func kindRank(kind string) int {
	switch kind {
	case model.NetworkKindLoopback:
		return 0
	case model.NetworkKindPrivate:
		return 1
	case model.NetworkKindVPN:
		return 2
	case model.NetworkKindTunnel:
		return 3
	case model.NetworkKindPublic:
		return 4
	default:
		return 5
	}
}

// sortByPriorityThenKind 按 (priority asc, kindRank asc) 排序。
func sortByPriorityThenKind(nets []model.Network) {
	sort.SliceStable(nets, func(i, j int) bool {
		if nets[i].Priority != nets[j].Priority {
			return nets[i].Priority < nets[j].Priority
		}
		return kindRank(nets[i].Kind) < kindRank(nets[j].Kind)
	})
}

// buildURL 拼最终 HTTP URL。schema 固定 http——TLS 终结在 edge nginx，
// 到 upstream 的连接默认明文（用户可在 IngressRoute.Extra 自行覆盖）。
func buildURL(host string, port int) string {
	if port <= 0 {
		return fmt.Sprintf("http://%s", host)
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// reasonFor 生成人类可读的选择解释，给 UI 预览用。
func reasonFor(selected model.Network, edge, target *model.Server, pref string) string {
	switch selected.Kind {
	case model.NetworkKindPrivate:
		return fmt.Sprintf("内网匹配 network_id=%s", selected.NetworkID)
	case model.NetworkKindVPN:
		return fmt.Sprintf("VPN 匹配 network_id=%s", selected.NetworkID)
	case model.NetworkKindTunnel:
		return fmt.Sprintf("反向隧道（edge=%d 可达）", edge.ID)
	case model.NetworkKindPublic:
		if pref == PrefAuto {
			return "未找到内网/VPN/隧道路径，兜底走公网"
		}
		return "显式 pref=public"
	default:
		return "selected"
	}
}
