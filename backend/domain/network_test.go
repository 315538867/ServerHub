package domain

import "testing"

// INV-1: domain.Network.Validate 校验规则

func TestNetworkValidate_ValidKinds(t *testing.T) {
	for _, kind := range []string{
		NetworkKindLoopback, NetworkKindPrivate, NetworkKindVPN,
		NetworkKindTunnel, NetworkKindPublic,
	} {
		n := &Network{Kind: kind, Address: "127.0.0.1"}
		// loopback 不需要 network_id
		if kind == NetworkKindPrivate || kind == NetworkKindVPN {
			n.NetworkID = "net-1"
		}
		if kind == NetworkKindTunnel {
			n.ReachableFrom = []uint{1}
		}
		if err := n.Validate(); err != nil {
			t.Errorf("kind=%s expected valid, got: %v", kind, err)
		}
	}
}

func TestNetworkValidate_UnknownKind(t *testing.T) {
	n := &Network{Kind: "invalid", Address: "10.0.0.1"}
	if err := n.Validate(); err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestNetworkValidate_EmptyAddress(t *testing.T) {
	n := &Network{Kind: NetworkKindPrivate, Address: "", NetworkID: "net-1"}
	if err := n.Validate(); err == nil {
		t.Error("expected error for empty address")
	}
}

func TestNetworkValidate_PrivateMissingNetworkID(t *testing.T) {
	n := &Network{Kind: NetworkKindPrivate, Address: "10.0.0.1", NetworkID: ""}
	if err := n.Validate(); err == nil {
		t.Error("expected error for private missing network_id")
	}
}

func TestNetworkValidate_VPNMissingNetworkID(t *testing.T) {
	n := &Network{Kind: NetworkKindVPN, Address: "10.0.0.1", NetworkID: ""}
	if err := n.Validate(); err == nil {
		t.Error("expected error for vpn missing network_id")
	}
}

func TestNetworkValidate_TunnelMissingReachableFrom(t *testing.T) {
	n := &Network{Kind: NetworkKindTunnel, Address: "10.0.0.1"}
	if err := n.Validate(); err == nil {
		t.Error("expected error for tunnel missing reachable_from")
	}
}

func TestNetworkValidate_PublicAutoNetworkID(t *testing.T) {
	n := &Network{Kind: NetworkKindPublic, Address: "1.2.3.4"}
	if err := n.Validate(); err != nil {
		t.Fatalf("expected valid public: %v", err)
	}
	if n.NetworkID != "public" {
		t.Errorf("expected NetworkID='public' after validate, got %s", n.NetworkID)
	}
}

func TestDefaultPriority(t *testing.T) {
	cases := []struct {
		kind string
		want int
	}{
		{NetworkKindLoopback, 0},
		{NetworkKindPrivate, 10},
		{NetworkKindVPN, 20},
		{NetworkKindTunnel, 30},
		{NetworkKindPublic, 100},
		{"unknown", 50},
	}
	for _, c := range cases {
		if got := DefaultPriority(c.kind); got != c.want {
			t.Errorf("DefaultPriority(%s)=%d want %d", c.kind, got, c.want)
		}
	}
}
