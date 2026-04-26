package scheduler

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/acme"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/runner"
)

// CertRenewWindow 是触发续签的提前量：到期前 30 天就尝试 renew。
//
// 选 30 天的依据：Let's Encrypt 官方建议在过期前 30 天续签，留有缓冲应对 ACME
// rate limit / DNS 抖动；同时 nginx_cert.expires_at 字段精度只到天，30 天提前
// 量足够吸收一天内的时序漂移，不会过早 renew 浪费签发额度。
const CertRenewWindow = 30 * 24 * time.Hour

// certRunnerFactory 让测试能注入 fake runner，省去 SSH 依赖。
type certRunnerFactory func(*model.Server, *config.Config) (runner.Runner, error)

var defaultCertRunnerFactory certRunnerFactory = runner.ForDedicated

// SetCertRunnerFactory 仅供测试覆盖；返回旧 factory 以便恢复。
func SetCertRunnerFactory(f certRunnerFactory) certRunnerFactory {
	old := defaultCertRunnerFactory
	defaultCertRunnerFactory = f
	return old
}

// StartCertRenewer 启动证书续签的后台 loop。
//
//   - 启动时立刻扫一次，避免重启后跳过当天窗口；
//   - 之后每小时探测一次，命中 cfg.Scheduler.CertCheckHour 且当天未跑过才执行。
//
// 单 goroutine、永不退出，错误只打日志。
func StartCertRenewer(db *gorm.DB, cfg *config.Config) {
	go func() {
		RenewExpiringCerts(db, cfg)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		lastRunDay := -1
		for range ticker.C {
			now := time.Now()
			if now.Hour() != cfg.Scheduler.CertCheckHour {
				continue
			}
			if now.Day() == lastRunDay {
				continue
			}
			RenewExpiringCerts(db, cfg)
			lastRunDay = now.Day()
		}
	}()
}

// RenewExpiringCerts 是可手动调用的同步入口（管理面板的"立刻续签"按钮，以及单测）。
//
// 返回 renewed / failed 数量，便于上层暴露指标。错误只打 stdout，不抛——
// 续签是后台 housekeeping，单条失败不能影响其他证书。
func RenewExpiringCerts(db *gorm.DB, cfg *config.Config) (renewed, failed int) {
	threshold := time.Now().Add(CertRenewWindow)
	var certs []model.SSLCert
	if err := db.Where("auto_renew = ? AND expires_at <= ? AND expires_at > ?",
		true, threshold, time.Time{}).Find(&certs).Error; err != nil {
		fmt.Printf("[cert-renew] 扫表失败: %v\n", err)
		return 0, 0
	}
	for _, cert := range certs {
		ok := renewOneCert(db, cfg, cert)
		if ok {
			renewed++
		} else {
			failed++
		}
	}
	if renewed+failed > 0 {
		fmt.Printf("[cert-renew] 完成: renewed=%d failed=%d\n", renewed, failed)
	}
	return renewed, failed
}

// renewOneCert 续签单张证书。任何步骤出错都返回 false，错误打 stdout。
func renewOneCert(db *gorm.DB, cfg *config.Config, cert model.SSLCert) bool {
	var s model.Server
	if err := db.First(&s, cert.ServerID).Error; err != nil {
		fmt.Printf("[cert-renew] cert id=%d server 加载失败: %v\n", cert.ID, err)
		return false
	}
	rn, err := defaultCertRunnerFactory(&s, cfg)
	if err != nil {
		fmt.Printf("[cert-renew] cert id=%d runner 失败: %v\n", cert.ID, err)
		return false
	}
	defer rn.Close()

	cmd, err := acme.RenewCmd(cert.Domain)
	if err != nil {
		fmt.Printf("[cert-renew] cert id=%d 域名非法: %v\n", cert.ID, err)
		return false
	}
	if out, err := rn.Run(cmd); err != nil {
		fmt.Printf("[cert-renew] cert id=%d certbot 失败: %v\n%s\n", cert.ID, err, out)
		return false
	}
	pem, err := acme.ReadPEM(rn, cert.Domain)
	if err != nil {
		fmt.Printf("[cert-renew] cert id=%d 读 PEM 失败: %v\n", cert.ID, err)
		return false
	}
	encCert, err := crypto.Encrypt(pem.Cert, cfg.Security.AESKey)
	if err != nil {
		fmt.Printf("[cert-renew] cert id=%d 加密 cert: %v\n", cert.ID, err)
		return false
	}
	encKey, err := crypto.Encrypt(pem.Key, cfg.Security.AESKey)
	if err != nil {
		fmt.Printf("[cert-renew] cert id=%d 加密 key: %v\n", cert.ID, err)
		return false
	}
	expiry, _ := acme.ParseExpiry(rn, acme.LiveCertPath(cert.Domain))
	now := time.Now()
	updates := map[string]any{
		"cert_pem":        encCert,
		"key_pem":         encKey,
		"last_renewed_at": &now,
	}
	if !expiry.IsZero() {
		updates["expires_at"] = expiry
	}
	if err := db.Model(&cert).Updates(updates).Error; err != nil {
		fmt.Printf("[cert-renew] cert id=%d DB 写回失败: %v\n", cert.ID, err)
		return false
	}
	return true
}
