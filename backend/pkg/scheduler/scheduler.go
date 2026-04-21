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
	db.Find(&servers)
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

	if s.Type == "local" {
		metrics, err = sshpool.CollectLocalMetrics()
		if err != nil {
			fmt.Printf("[scheduler] local metrics error: %v\n", err)
			db.Model(&s).Updates(map[string]any{"status": "online", "last_check_at": now})
			return
		}
	} else {
		cred, derr := decryptCred(s, cfg.Security.AESKey)
		if derr != nil {
			return
		}
		client, derr := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		if derr != nil {
			db.Model(&s).Updates(map[string]any{"status": "offline", "last_check_at": now})
			go checkAlerts(db, cfg, s.ID, 0, 0, 0, true)
			return
		}
		metrics, err = sshpool.CollectMetrics(client)
		if err != nil {
			fmt.Printf("[scheduler] metrics error server %d: %v\n", s.ID, err)
			db.Model(&s).Updates(map[string]any{"status": "online", "last_check_at": now})
			return
		}
	}

	db.Create(&model.Metric{
		ServerID: s.ID,
		CPU:      metrics.CPU,
		Mem:      metrics.Mem,
		Disk:     metrics.Disk,
		Load1:    metrics.Load1,
		Uptime:   metrics.Uptime,
	})
	db.Model(&s).Updates(map[string]any{"status": "online", "last_check_at": now})
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
