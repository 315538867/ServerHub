// Package sysinfo provides runtime environment detection.
package sysinfo

import (
	"bufio"
	"os"
	"strings"
)

// Capability represents what the running binary can do to "the local host".
//   - CapFull:   裸机，或容器挂载了 /host + --pid=host + sock，可经 nsenter
//     管控宿主 systemd / 文件 / docker。
//   - CapDocker: 仅挂 docker.sock；只能通过 socket 管控宿主 docker 引擎，
//     不能读/改宿主文件、systemd。
//   - CapNone:   容器既没挂 sock 也没挂宿主根；UI 不应显示"本机"卡片。
const (
	CapFull   = "full"
	CapDocker = "docker"
	CapNone   = "none"
)

// LocalCapability reports what operations the current process can perform
// against the host it runs on. Called at boot by seedLocalServer to decide
// whether to create a Type="local" Server row and what Capability to stamp.
func LocalCapability() string {
	if !IsContainerized() {
		return CapFull
	}
	hasSock := hasDockerSocket()
	hasHostRoot := dirExists("/host")
	hostPID := hasHostPIDNamespace()
	if hasSock && hasHostRoot && hostPID {
		return CapFull
	}
	if hasSock {
		return CapDocker
	}
	return CapNone
}

func hasDockerSocket() bool {
	fi, err := os.Stat("/var/run/docker.sock")
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSocket != 0
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// hasHostPIDNamespace reports whether the container shares the host PID
// namespace (launched with `--pid=host`). Detected by comparing our own
// PID namespace inode with /proc/1/ns/pid — in a shared namespace they
// match; in a private namespace PID 1 is the container's init.
func hasHostPIDNamespace() bool {
	self, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return false
	}
	one, err := os.Readlink("/proc/1/ns/pid")
	if err != nil {
		return false
	}
	return self == one
}

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

// HostGatewayIP / 路由探测工具已随首次引导 SSH 自管路径删除。
// 容器现在通过挂载 docker.sock + /host + --pid=host 直接拥有本机能力，
// 不再需要"猜宿主 IP 然后回连 SSH"。
