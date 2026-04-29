// Package usecase: ssl.go 收口 SSL 证书子域业务逻辑。
//
// 包含 Server 查找、runner 获取、远端写文件、PEM 加密落库（persistCert）、
// 过期时间解析与 cert 列表查询。handler 只负责 DTO 解析 / 参数校验 / 回响应。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/serverhub/serverhub/adapters/ingress/nginx/ssl"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/sftppool"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ── Server / runner ──────────────────────────────────────────────────────────

// GetServerByID 查询服务器；不存在时返回 gorm.ErrRecordNotFound。
func GetServerByID(ctx context.Context, db *gorm.DB, id uint) (domain.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.GetServerByID(ctx, db, id)
}

// GetServerRunner 查 Server 并创建 runner（非 dedicated，无需 Close）。
func GetServerRunner(ctx context.Context, db *gorm.DB, cfg *config.Config, serverID uint) (runner.Runner, domain.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	s, err := repo.GetServerByID(ctx, db, serverID)
	if err != nil {
		return nil, domain.Server{}, err
	}
	r, err := runner.For(&s, cfg)
	if err != nil {
		return nil, domain.Server{}, err
	}
	return r, s, nil
}

// GetServerDedicatedRunner 查 Server 并创建 dedicated runner（调用方需 defer Close）。
func GetServerDedicatedRunner(ctx context.Context, db *gorm.DB, cfg *config.Config, serverID uint) (runner.Runner, domain.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	s, err := repo.GetServerByID(ctx, db, serverID)
	if err != nil {
		return nil, domain.Server{}, err
	}
	r, err := runner.ForDedicated(&s, cfg)
	if err != nil {
		return nil, domain.Server{}, err
	}
	return r, s, nil
}

// ── 远端文件 ─────────────────────────────────────────────────────────────────

// WriteRemoteFile 将内容写到目标服务器文件。本地走 os.WriteFile，远程走 SFTP。
func WriteRemoteFile(rn runner.Runner, serverID uint, path, content string, mode os.FileMode) error {
	if rn.IsLocal() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), mode)
	}
	cli := runner.SSHClient(rn)
	if cli == nil {
		return fmt.Errorf("no ssh client")
	}
	sc, err := sftppool.Get(serverID, cli)
	if err != nil {
		return err
	}
	f, err := sc.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return sc.Chmod(path, mode)
}

// ── 证书解析 ─────────────────────────────────────────────────────────────────

// ParseExpiryFromPEM 用本地 openssl 解析证书 notAfter 时间。失败返回零值（expires_at 非强必需字段）。
func ParseExpiryFromPEM(certPEM string) (time.Time, error) {
	tmp, err := os.CreateTemp("", "sh-cert-*.pem")
	if err != nil {
		return time.Time{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(certPEM); err != nil {
		_ = tmp.Close()
		return time.Time{}, err
	}
	_ = tmp.Close()
	out, err := runLocalCmd("openssl x509 -enddate -noout -in " + ShellQuote(tmp.Name()))
	if err != nil {
		return time.Time{}, err
	}
	out = strings.TrimSpace(out)
	after, found := strings.CutPrefix(out, "notAfter=")
	if !found {
		return time.Time{}, fmt.Errorf("openssl 输出异常: %s", out)
	}
	after = strings.TrimSpace(after)
	t, err := time.Parse("Jan  2 15:04:05 2006 GMT", after)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05 2006 GMT", after)
	}
	return t, err
}

// runLocalCmd 跑一条本地 shell 命令并返回合并输出。
func runLocalCmd(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

// ── 证书持久化 ───────────────────────────────────────────────────────────────

// PersistCert 将 PEM 加密落库并写入远端 canonical 路径。Issuer / AutoRenew / LastRenewedAt
// 由调用方决定；domain + serverID 维度 upsert。
func PersistCert(
	ctx context.Context,
	db *gorm.DB, rn runner.Runner, cfg *config.Config,
	serverID uint, domainName, certPEM, keyPEM, issuer string,
	autoRenew bool, markRenew bool,
) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	encCert, err := crypto.Encrypt(certPEM, cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("加密 cert: %w", err)
	}
	encKey, err := crypto.Encrypt(keyPEM, cfg.Security.AESKey)
	if err != nil {
		return fmt.Errorf("加密 key: %w", err)
	}
	certPath, keyPath := ssl.CertCanonicalPaths(domainName)

	// 远端先落盘——上传后立即可用。
	if err := WriteRemoteFile(rn, serverID, certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("写入远端 cert: %w", err)
	}
	if err := WriteRemoteFile(rn, serverID, keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("写入远端 key: %w", err)
	}

	expiry, _ := ParseExpiryFromPEM(certPEM)

	now := time.Now()
	updates := map[string]any{
		"cert_path":  certPath,
		"key_path":   keyPath,
		"cert_pem":   encCert,
		"key_pem":    encKey,
		"issuer":     issuer,
		"expires_at": expiry,
		"auto_renew": autoRenew,
	}
	if markRenew {
		updates["last_renewed_at"] = &now
	}

	existing, err := repo.GetCertByServerAndDomain(ctx, db, serverID, domainName)
	switch {
	case err == nil:
		return repo.UpdateCertFields(ctx, db, existing.ID, updates)
	case err == gorm.ErrRecordNotFound:
		cert := domain.SSLCert{
			ServerID:  serverID,
			Domain:    domainName,
			CertPath:  certPath,
			KeyPath:   keyPath,
			CertPEM:   encCert,
			KeyPEM:    encKey,
			Issuer:    issuer,
			ExpiresAt: expiry,
			AutoRenew: autoRenew,
		}
		if markRenew {
			cert.LastRenewedAt = &now
		}
		return repo.CreateCert(ctx, db, &cert)
	default:
		return err
	}
}

// ── cert CRUD 包装 ───────────────────────────────────────────────────────────

// ListSSLCerts 列出某服务器的证书列表，可选按 application_id 过滤。
func ListSSLCerts(ctx context.Context, db *gorm.DB, serverID uint, applicationID *uint) ([]domain.SSLCert, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.ListCerts(ctx, db, serverID, applicationID)
}

// GetCertByServerAndCertID 按 serverID + certID 查询证书（确保归属校验）。
func GetCertByServerAndCertID(ctx context.Context, db *gorm.DB, serverID, certID uint) (domain.SSLCert, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cert, err := repo.GetCertByID(ctx, db, certID)
	if err != nil {
		return domain.SSLCert{}, err
	}
	if cert.ServerID != serverID {
		return domain.SSLCert{}, gorm.ErrRecordNotFound
	}
	return cert, nil
}

// DeleteSSLCert 按 ID 删除证书。
func DeleteSSLCert(ctx context.Context, db *gorm.DB, id uint) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.DeleteCert(ctx, db, id)
}
