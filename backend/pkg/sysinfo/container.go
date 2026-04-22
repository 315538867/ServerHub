// Package sysinfo provides runtime environment detection.
package sysinfo

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// IsContainerized reports whether the process is running inside a container.
// Detection order: /.dockerenv (docker, also written by some buildah images),
// /run/.containerenv (podman), then cgroup heuristic for k8s.
func IsContainerized() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	f, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if strings.Contains(l, "docker") || strings.Contains(l, "kubepods") || strings.Contains(l, "containerd") {
			return true
		}
	}
	return false
}

// HostGatewayIP returns the IPv4 address of the default route's next hop,
// which inside a docker container points back at the host's bridge interface.
// Used by the setup wizard to suggest the SSH target when self-managing the
// host from within the container.
//
// Falls back to "host.docker.internal" lookup, then "172.17.0.1" (default
// docker0), then empty string if everything fails.
func HostGatewayIP() string {
	if ip := defaultRouteGateway(); ip != "" {
		return ip
	}
	if ip := procNetRouteGateway(); ip != "" {
		return ip
	}
	if ips, err := net.LookupIP("host.docker.internal"); err == nil {
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return "172.17.0.1"
}

func defaultRouteGateway() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	// Format: "default via 172.30.0.1 dev eth0 ..."
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// procNetRouteGateway reads /proc/net/route directly so the lookup works on
// minimal images that don't ship the iproute2 binary. The Gateway column is
// little-endian hex.
func procNetRouteGateway() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "00000000" { // not default route
			continue
		}
		raw, err := hex.DecodeString(fields[2])
		if err != nil || len(raw) != 4 {
			return ""
		}
		// little-endian → dotted quad
		return fmt.Sprintf("%d.%d.%d.%d", raw[3], raw[2], raw[1], raw[0])
	}
	return ""
}
