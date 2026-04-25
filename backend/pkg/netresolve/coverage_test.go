package netresolve

import (
	"testing"

	"github.com/serverhub/serverhub/model"
)

// TestKindRank_All 直接覆盖 kindRank：
// 之前没有任何路径会调到它的全部分支（loopback 永远在 Resolve 第 1 步短路、
// 未知 kind 也只是 candidates 阶段被过滤）。
func TestKindRank_All(t *testing.T) {
	cases := map[string]int{
		model.NetworkKindLoopback: 0,
		model.NetworkKindPrivate:  1,
		model.NetworkKindVPN:      2,
		model.NetworkKindTunnel:   3,
		model.NetworkKindPublic:   4,
		"weird-other":             5,
		"":                        5,
	}
	for k, want := range cases {
		if got := kindRank(k); got != want {
			t.Errorf("kindRank(%q)=%d want %d", k, got, want)
		}
	}
}

// TestSort_TieBreakByKind 同 priority 时按 (private<vpn<tunnel<public) 破平。
// 给 4 条同 priority=20 的网络，sortByPriorityThenKind 后应按上述顺序排。
func TestSort_TieBreakByKind(t *testing.T) {
	in := []model.Network{
		{Kind: model.NetworkKindPublic, Priority: 20, Address: "p"},
		{Kind: model.NetworkKindTunnel, Priority: 20, Address: "t"},
		{Kind: model.NetworkKindVPN, Priority: 20, Address: "v"},
		{Kind: model.NetworkKindPrivate, Priority: 20, Address: "n"},
	}
	sortByPriorityThenKind(in)
	wantOrder := []string{"n", "v", "t", "p"}
	for i, want := range wantOrder {
		if in[i].Address != want {
			t.Errorf("idx=%d got addr=%s want %s", i, in[i].Address, want)
		}
	}
}

// TestSort_TieBreakRespectsPriorityFirst priority 不同则忽略 kindRank。
func TestSort_TieBreakRespectsPriorityFirst(t *testing.T) {
	in := []model.Network{
		{Kind: model.NetworkKindPublic, Priority: 5, Address: "p"},
		{Kind: model.NetworkKindPrivate, Priority: 10, Address: "n"},
	}
	sortByPriorityThenKind(in)
	if in[0].Address != "p" {
		t.Errorf("低 priority 的 public 应该排在前面，got=%s", in[0].Address)
	}
}

// TestCandidatesByPref_PrivateMismatched edge 没有同 NetworkID 的 private →
// 显式 pref=private 应过滤为空。
func TestCandidatesByPref_PrivateMismatched(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-B", Address: "10.0.1.2"},
	)
	got := candidatesByPref(target, model.NetworkKindPrivate, edge.ID, edge)
	if len(got) != 0 {
		t.Fatalf("不同 lan 应该过滤为空, got=%+v", got)
	}
}

// TestCandidatesByPref_VPNExplicit 显式 pref=vpn，覆盖 candidatesByPref 的
// vpn 分支（之前只测了 PrefPrivate / PrefTunnel）。
func TestCandidatesByPref_VPNExplicit(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.2"},
		// 一条 NetworkID 不同的应被过滤掉
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-Y", Address: "10.9.0.2"},
		// 不是 vpn 的也被过滤
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4"},
	)
	got := candidatesByPref(target, model.NetworkKindVPN, edge.ID, edge)
	if len(got) != 1 || got[0].Address != "10.8.0.2" {
		t.Fatalf("仅同 NetworkID 的 vpn 应保留，got=%+v", got)
	}
}

// TestShareNetworkID_EmptyID 空 NetworkID 直接走 false 分支
// （即便 edge 也有空 NetworkID 也不应误判匹配）。
func TestShareNetworkID_EmptyID(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "", Address: "10.0.0.1"},
	)
	if _, ok := shareNetworkID(edge, model.NetworkKindPrivate, ""); ok {
		t.Fatalf("空 NetworkID 不应视为匹配")
	}
}

// TestReasonFor_DefaultUnknownKind selected.Kind 在 reasonFor 的 switch 之外
// 时落到 default 分支。
func TestReasonFor_DefaultUnknownKind(t *testing.T) {
	edge := &model.Server{}
	edge.ID = 1
	target := &model.Server{}
	target.ID = 2
	got := reasonFor(model.Network{Kind: "unknown-kind"}, edge, target, PrefAuto)
	if got != "selected" {
		t.Errorf("unknown kind 应返回兜底文案, got=%q", got)
	}
}

// TestReasonFor_PublicExplicit 显式 pref=public 时 reasonFor 的 public 分支
// 走非 auto 文案（之前只测过 auto 兜底文案）。
func TestReasonFor_PublicExplicit(t *testing.T) {
	edge := &model.Server{}
	edge.ID = 1
	target := &model.Server{}
	target.ID = 2
	got := reasonFor(
		model.Network{Kind: model.NetworkKindPublic},
		edge, target, PrefPublic,
	)
	if got != "显式 pref=public" {
		t.Errorf("public 显式 pref 文案, got=%q", got)
	}
}
