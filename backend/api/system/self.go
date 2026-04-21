package system

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/pkg/resp"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	selfSampleInterval = 5 * time.Second
	selfHistoryCap     = 12
)

var (
	selfStartTime = time.Now()
	selfProc      *process.Process
	selfOnce      sync.Once

	selfMu      sync.RWMutex
	selfCPUHist = make([]float64, 0, selfHistoryCap)
	selfMemHist = make([]float64, 0, selfHistoryCap)
)

func selfInit() {
	selfOnce.Do(func() {
		p, err := process.NewProcess(int32(os.Getpid()))
		if err == nil {
			selfProc = p
			_, _ = p.CPUPercent()
			selfSampleOnce()
			go selfSampleLoop()
		}
	})
}

func selfSampleOnce() {
	if selfProc == nil {
		return
	}
	cpu, _ := selfProc.CPUPercent()
	var rssMB float64
	if mi, err := selfProc.MemoryInfo(); err == nil && mi != nil {
		rssMB = float64(mi.RSS) / 1024 / 1024
	}
	selfMu.Lock()
	selfCPUHist = pushRing(selfCPUHist, cpu)
	selfMemHist = pushRing(selfMemHist, rssMB)
	selfMu.Unlock()
}

func selfSampleLoop() {
	t := time.NewTicker(selfSampleInterval)
	defer t.Stop()
	for range t.C {
		selfSampleOnce()
	}
}

func pushRing(s []float64, v float64) []float64 {
	if len(s) >= selfHistoryCap {
		s = s[1:]
	}
	return append(s, v)
}

type selfHistory struct {
	CPU []float64 `json:"cpu"`
	Mem []float64 `json:"mem"`
}

type SelfMetricsResp struct {
	CPUPercent  float64     `json:"cpu_percent"`
	MemRSS      uint64      `json:"mem_rss"`
	MemSys      uint64      `json:"mem_sys"`
	Goroutines  int         `json:"goroutines"`
	Uptime      int64       `json:"uptime"`
	Connections int         `json:"connections"`
	NumCPU      int         `json:"num_cpu"`
	History     selfHistory `json:"history"`
}

func selfMetricsHandler(c *gin.Context) {
	selfInit()

	var (
		cpu  float64
		rss  uint64
		conn int
	)
	if selfProc != nil {
		if v, err := selfProc.CPUPercent(); err == nil {
			cpu = v
		}
		if mi, err := selfProc.MemoryInfo(); err == nil && mi != nil {
			rss = mi.RSS
		}
		if cs, err := gnet.ConnectionsPid("inet", selfProc.Pid); err == nil {
			conn = len(cs)
		}
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	selfMu.RLock()
	cpuHist := append([]float64(nil), selfCPUHist...)
	memHist := append([]float64(nil), selfMemHist...)
	selfMu.RUnlock()

	resp.OK(c, SelfMetricsResp{
		CPUPercent:  cpu,
		MemRSS:      rss,
		MemSys:      ms.Sys,
		Goroutines:  runtime.NumGoroutine(),
		Uptime:      int64(time.Since(selfStartTime).Seconds()),
		Connections: conn,
		NumCPU:      runtime.NumCPU(),
		History:     selfHistory{CPU: cpuHist, Mem: memHist},
	})
}

func RegisterSelfRoutes(r *gin.RouterGroup) {
	selfInit()
	r.GET("", selfMetricsHandler)
}
