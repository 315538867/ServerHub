// Local in-process metrics collection (used when the "server" being measured
// is the host running ServerHub itself).
package sshpool

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

func CollectLocalMetrics() (*MetricsResult, error) {
	r := &MetricsResult{}

	// CPU: 300ms sample to mirror metricsScript window.
	if vals, err := cpu.Percent(300*time.Millisecond, false); err == nil && len(vals) > 0 {
		r.CPU = round1(vals[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		r.Mem = round1(vm.UsedPercent)
	}
	if du, err := disk.Usage("/"); err == nil {
		r.Disk = round1(du.UsedPercent)
	}
	if la, err := load.Avg(); err == nil {
		r.Load1 = round1(la.Load1)
	}
	if up, err := host.Uptime(); err == nil {
		r.Uptime = int64(up)
	}
	_ = runtime.NumCPU
	return r, nil
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
