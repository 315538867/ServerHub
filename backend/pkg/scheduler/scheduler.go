package scheduler

import (
	"fmt"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"gorm.io/gorm"
)

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
	for _, s := range servers {
		go collectOne(db, cfg, s)
	}
}

func collectOne(db *gorm.DB, cfg *config.Config, s model.Server) {
	cred, err := decryptCred(s, cfg.Security.AESKey)
	if err != nil {
		return
	}

	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	now := time.Now()
	if err != nil {
		db.Model(&s).Updates(map[string]any{"status": "offline", "last_check_at": now})
		go checkAlerts(db, cfg, s.ID, 0, 0, 0, true)
		return
	}

	metrics, err := sshpool.CollectMetrics(client)
	if err != nil {
		// connection succeeded but metrics failed — still mark online
		fmt.Printf("[scheduler] metrics error server %d: %v\n", s.ID, err)
		db.Model(&s).Updates(map[string]any{"status": "online", "last_check_at": now})
		return
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
