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
// namespace (launched with `--pid=host`).
//
// 历史实现用 os.Readlink("/proc/1/ns/pid") 比较 ns inode，但在大多数现代内核
// 下，即便容器加了 CAP_SYS_ADMIN+CAP_SYS_PTRACE，readlink ns 链接也会被
// ptrace_may_access 拒绝（要 PTRACE_MODE_READ on target），返回 EACCES，
// 导致 --pid=host 也被误判为私有 PID ns。
//
// 改用 cgroup 指纹：PID 1 的 /proc/1/cgroup 内容能区分宿主 init 与容器 init。
//   - 宿主 systemd init：路径形如 "/init.scope"、"/" 或 "0::/../../init.scope"
//     （不含 docker/kubepods/containerd 段）。
//   - 容器内 init：路径形如 "0::/docker/<id>" 或 "0::/kubepods/..."（含容器
//     运行时段）。
// 我们已知自身在容器里（IsContainerized==true 才会进到这里的调用方），所以
// "PID 1 的 cgroup 不含容器运行时段" 即可推断 PID ns 与宿主共享。
func hasHostPIDNamespace() bool {
	f, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if strings.Contains(l, "/docker/") ||
			strings.Contains(l, "/docker-") ||
			strings.Contains(l, "kubepods") ||
			strings.Contains(l, "containerd") ||
			strings.Contains(l, "/lxc/") ||
			strings.Contains(l, "/podman") {
			return false
		}
	}
	return true
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
