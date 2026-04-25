package netresolve

import (
	"strings"
	"testing"

	"github.com/serverhub/serverhub/model"
)

// helper: 构造一台 server，自动塞入 loopback。
func mkServer(id uint, nets ...model.Network) *model.Server {
	s := &model.Server{}
	s.ID = id
	s.Networks = append(model.Networks{
		{Kind: model.NetworkKindLoopback, NetworkID: lopID(id), Address: "127.0.0.1", Priority: 0},
	}, nets...)
	return s
}

func lopID(id uint) string {
	// 对应 model.Server.loopbackNetwork 的命名约定
	return "lo-" + uintStr(id)
}

func uintStr(id uint) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}

func TestT1_SameServerLoopback(t *testing.T) {
	s := mkServer(1)
	r, err := Resolve(s, s, 8080, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.URL != "http://127.0.0.1:8080" {
		t.Fatalf("url=%s", r.URL)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindLoopback {
		t.Fatalf("kind=%s", r.SelectedNetwork.Kind)
	}
}

func TestT2_SameLAN(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1", Priority: 10},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2", Priority: 10},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4", Priority: 100},
	)
	r, err := Resolve(edge, target, 80, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.URL != "http://10.0.0.2:80" {
		t.Fatalf("url=%s", r.URL)
	}
}

func TestT3_VPNOnly(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.1", Priority: 20},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.2", Priority: 20},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.1.1.1", Priority: 100},
	)
	r, err := Resolve(edge, target, 80, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindVPN || r.URL != "http://10.8.0.2:80" {
		t.Fatalf("got %+v", r)
	}
}

func TestT4_TunnelReachable(t *testing.T) {
	edge := mkServer(1)
	target := mkServer(2,
		model.Network{
			Kind:          model.NetworkKindTunnel,
			NetworkID:     "tun-A",
			Address:       "127.0.0.1:7000",
			Priority:      30,
			ReachableFrom: []uint{1},
		},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.1.1.1", Priority: 100},
	)
	r, err := Resolve(edge, target, 0, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindTunnel {
		t.Fatalf("kind=%s url=%s", r.SelectedNetwork.Kind, r.URL)
	}
}

func TestT5_PublicFallback(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-B", Address: "10.0.1.2"},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4"},
	)
	r, err := Resolve(edge, target, 443, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindPublic {
		t.Fatalf("expect public got %+v", r.SelectedNetwork)
	}
	if !strings.Contains(r.Reason, "兜底") {
		t.Fatalf("reason=%s", r.Reason)
	}
}

func TestT6_NoCandidate(t *testing.T) {
	edge := mkServer(1)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2"},
	)
	_, err := Resolve(edge, target, 80, "", "", 0)
	if err == nil {
		t.Fatal("expect err")
	}
}

func TestT7_PrefPublicForce(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2"},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4"},
	)
	r, err := Resolve(edge, target, 80, PrefPublic, "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindPublic {
		t.Fatalf("got %+v", r.SelectedNetwork)
	}
}

func TestT8_PrefMissing(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4"},
	)
	_, err := Resolve(edge, target, 80, PrefPrivate, "", 0)
	if err == nil {
		t.Fatal("expect err since target lacks private")
	}
}

func TestT9_OverrideHost(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2"},
	)
	r, err := Resolve(edge, target, 80, "", "custom.local", 9999)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.URL != "http://custom.local:9999" {
		t.Fatalf("url=%s", r.URL)
	}
}

func TestT10_PrioritySort(t *testing.T) {
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2", Priority: 50},
		model.Network{Kind: model.NetworkKindVPN, NetworkID: "vpn-X", Address: "10.8.0.2", Priority: 20},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.2.3.4", Priority: 100},
	)
	// VPN priority(20) < Private(50) → 应该选 VPN
	r, err := Resolve(edge, target, 80, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindVPN {
		t.Fatalf("expect vpn got %+v", r.SelectedNetwork)
	}
}

func TestT11_MultiLANSegments(t *testing.T) {
	// edge 在 lan-A，target 同时挂 lan-B 和 lan-A——应只匹配 lan-A
	edge := mkServer(1,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.1"},
	)
	target := mkServer(2,
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-B", Address: "10.0.1.2"},
		model.Network{Kind: model.NetworkKindPrivate, NetworkID: "lan-A", Address: "10.0.0.2"},
	)
	r, err := Resolve(edge, target, 80, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.URL != "http://10.0.0.2:80" {
		t.Fatalf("url=%s reason=%s", r.URL, r.Reason)
	}
}

func TestNilGuards(t *testing.T) {
	if _, err := Resolve(nil, nil, 80, "", "", 0); err == nil {
		t.Fatal("expect err")
	}
}

func TestTunnelUnreachable(t *testing.T) {
	edge := mkServer(1)
	target := mkServer(2,
		model.Network{
			Kind:          model.NetworkKindTunnel,
			NetworkID:     "tun-A",
			Address:       "127.0.0.1:7000",
			ReachableFrom: []uint{99}, // 不含 edge.id=1
		},
		model.Network{Kind: model.NetworkKindPublic, NetworkID: "public", Address: "1.1.1.1"},
	)
	r, err := Resolve(edge, target, 80, "", "", 0)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.SelectedNetwork.Kind != model.NetworkKindPublic {
		t.Fatalf("expect public, got %s", r.SelectedNetwork.Kind)
	}
}

func TestPrefTunnelUnreachable(t *testing.T) {
	edge := mkServer(1)
	target := mkServer(2,
		model.Network{
			Kind:          model.NetworkKindTunnel,
			Address:       "127.0.0.1:7000",
			ReachableFrom: []uint{99},
		},
	)
	if _, err := Resolve(edge, target, 80, PrefTunnel, "", 0); err == nil {
		t.Fatal("expect err since tunnel unreachable for edge")
	}
}

func TestBuildURLNoPort(t *testing.T) {
	if got := buildURL("h", 0); got != "http://h" {
		t.Fatalf("got=%s", got)
	}
}
