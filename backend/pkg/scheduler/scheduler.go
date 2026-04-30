package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

var collectSem = semaphore.NewWeighted(8)

// Start launches the background metrics collection loop.
func Start(db *gorm.DB, cfg *config.Config) {
	interval := time.Duration(cfg.Scheduler.MetricsIntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[scheduler] PANIC: %v\n", r)
			}
		}()
		fmt.Printf("[scheduler] started, interval=%v\n", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		collectAll(db, cfg) // immediate first run
		for range ticker.C {
			collectAll(db, cfg)
		}
	}()
}

func collectAll(db *gorm.DB, cfg *config.Config) {
	var servers []model.Server
	if err := db.Find(&servers).Error; err != nil {
		fmt.Printf("[scheduler] db.Find servers error: %v\n", err)
		return
	}
	ctx := context.Background()
	for _, s := range servers {
		s := s
		if err := collectSem.Acquire(ctx, 1); err != nil {
			continue
		}
		go func() {
			defer collectSem.Release(1)
			collectOne(db, cfg, s)
		}()
	}
}

func collectOne(db *gorm.DB, cfg *config.Config, s model.Server) {
	now := time.Now()
	var metrics *sshpool.MetricsResult
	var err error
	probe := model.ServerProbe{ServerID: s.ID, Result: "online", CreatedAt: now}

	// R3 起 server 在线状态由 server_probes 时序表派生:本函数不论成败都恰好 INSERT
	// 一条 probe,显式 offline 仅由 SSH dial 失败触发(主机不可达);其它失败(指标
	// 采集错)仍计为 online + ErrMsg,因为 dial 已经成功证明主机存活。
	if s.Type == "local" {
		metrics, err = sshpool.CollectLocalMetrics()
		if err != nil {
			fmt.Printf("[scheduler] local metrics error server %d: %v\n", s.ID, err)
			probe.ErrMsg = err.Error()
			if cerr := db.Create(&probe).Error; cerr != nil {
				fmt.Printf("[scheduler] create probe error: %v\n", cerr)
			}
			return
		}
	} else {
		cred, derr := decryptCred(s, cfg.Security.AESKey)
		if derr != nil {
			return
		}
		dialStart := time.Now()
		client, derr := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		probe.LatencyMs = int(time.Since(dialStart).Milliseconds())
		if derr != nil {
			probe.Result = "offline"
			probe.ErrMsg = derr.Error()
			db.Create(&probe)
			go checkAlerts(db, cfg, s.ID, 0, 0, 0, true)
			return
		}
		metrics, err = sshpool.CollectMetrics(client)
		if err != nil {
			fmt.Printf("[scheduler] metrics error server %d: %v\n", s.ID, err)
			probe.ErrMsg = err.Error()
			db.Create(&probe)
			return
		}
	}

	if cerr := db.Create(&probe).Error; cerr != nil {
		fmt.Printf("[scheduler] create probe error server %d: %v\n", s.ID, cerr)
	}
	if cerr := db.Create(&model.Metric{
		ServerID: s.ID,
		CPU:      metrics.CPU,
		Mem:      metrics.Mem,
		Disk:     metrics.Disk,
		Load1:    metrics.Load1,
		Uptime:   metrics.Uptime,
	}).Error; cerr != nil {
		fmt.Printf("[scheduler] create metric error server %d: %v\n", s.ID, cerr)
	}
	go checkAlerts(db, cfg, s.ID, metrics.CPU, metrics.Mem, metrics.Disk, false)
}

func decryptCred(s model.Server, aesKey string) (string, error) {
	switch s.AuthType {
	case "key":
		if s.PrivateKey == "" {
			return "", nil
		}
		return crypto.Decrypt(s.PrivateKey, aesKey)
	default:
		if s.Password == "" {
			return "", nil
		}
		return crypto.Decrypt(s.Password, aesKey)
	}
}
