// Package setup implements the first-run wizard endpoints.
//
// Flow (no JWT required — the wizard runs before any user exists):
//
//	1. GET  /panel/api/v1/setup/status         → tells the UI which steps remain
//	2. POST /panel/api/v1/setup/admin          → creates the first admin user
//	3. POST /panel/api/v1/setup/local/init     → ed25519 keygen + shell command
//	4. POST /panel/api/v1/setup/local/activate → SSH self-test, persist Server row
//
// Safety gates (enforced server-side, never relying on the UI):
//
//   - /admin    is rejected once any user exists
//   - /local/*  is rejected outside containerized runtimes AND when no admin
//     exists yet (forces the wizard sequence)
package setup

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/auditq"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshkey"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/pkg/sysinfo"
	"gorm.io/gorm"
)

const (
	setupStateRowID = 1
	setupTTL        = 30 * time.Minute
	defaultSSHPort  = 22
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/status", statusHandler(db))
	r.POST("/admin", createAdminHandler(db))
	r.POST("/local/init", initLocalHandler(db, cfg))
	r.POST("/local/activate", activateLocalHandler(db, cfg))
}

type statusResp struct {
	Containerized    bool   `json:"containerized"`
	NeedsAdmin       bool   `json:"needs_admin"`
	NeedsLocalServer bool   `json:"needs_local_server"`
	HostGateway      string `json:"host_gateway,omitempty"`
}

func statusHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		containerized := sysinfo.IsContainerized()
		var userCount int64
		db.Model(&model.User{}).Count(&userCount)

		needsLocal := false
		if containerized {
			var sshSelfCount int64
			db.Model(&model.Server{}).
				Where("type = ? AND remark LIKE ?", "ssh", "ServerHub 本机%").
				Count(&sshSelfCount)
			needsLocal = sshSelfCount == 0
		}

		out := statusResp{
			Containerized:    containerized,
			NeedsAdmin:       userCount == 0,
			NeedsLocalServer: needsLocal,
		}
		if containerized {
			out.HostGateway = sysinfo.HostGatewayIP()
		}
		resp.OK(c, out)
	}
}

type adminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func createAdminHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "用户名和密码不能为空")
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if len(req.Username) < 3 || len(req.Password) < 6 {
			resp.BadRequest(c, "用户名至少 3 字符，密码至少 6 字符")
			return
		}

		var count int64
		db.Model(&model.User{}).Count(&count)
		if count > 0 {
			auditq.Security(req.Username, c.ClientIP(), "security:setup_admin_blocked", 409, nil)
			resp.Fail(c, http.StatusConflict, 1003, "已经初始化过管理员")
			return
		}

		hash, err := crypto.BcryptHash(req.Password)
		if err != nil {
			resp.InternalError(c, "密码加密失败")
			return
		}
		now := time.Now()
		user := model.User{
			Username:  req.Username,
			Password:  hash,
			Role:      "admin",
			LastLogin: &now,
		}
		if err := db.Create(&user).Error; err != nil {
			resp.InternalError(c, "创建管理员失败: "+err.Error())
			return
		}
		auditq.Security(req.Username, c.ClientIP(), "security:setup_admin_created", 200, nil)
		resp.OK(c, gin.H{"username": user.Username})
	}
}

type initLocalReq struct {
	TargetUser string `json:"target_user"`
}

type initLocalResp struct {
	PublicKey   string `json:"public_key"`
	HostGateway string `json:"host_gateway"`
	TargetUser  string `json:"target_user"`
	Command     string `json:"command"`
	ExpiresAt   string `json:"expires_at"`
}

func initLocalHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := guardLocalSetup(db); err != nil {
			resp.Fail(c, http.StatusForbidden, 1004, err.Error())
			return
		}

		var req initLocalReq
		_ = c.ShouldBindJSON(&req)
		user := strings.TrimSpace(req.TargetUser)
		if user == "" {
			user = "ubuntu"
		}
		if !validUnixUser(user) {
			resp.BadRequest(c, "用户名格式不合法")
			return
		}

		pair, err := sshkey.GenerateEd25519("serverhub@self")
		if err != nil {
			resp.InternalError(c, "生成密钥失败: "+err.Error())
			return
		}

		encPriv, err := crypto.Encrypt(pair.PrivatePEM, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密私钥失败")
			return
		}

		gateway := sysinfo.HostGatewayIP()
		state := model.SetupState{
			ID:                  setupStateRowID,
			EncryptedPrivateKey: encPriv,
			PublicKey:           strings.TrimSpace(pair.PublicAuthorized),
			HostGateway:         gateway,
			TargetUser:          user,
			ExpiresAt:           time.Now().Add(setupTTL),
		}
		// upsert single row
		db.Where("id = ?", setupStateRowID).Delete(&model.SetupState{})
		if err := db.Create(&state).Error; err != nil {
			resp.InternalError(c, "保存临时状态失败: "+err.Error())
			return
		}

		resp.OK(c, initLocalResp{
			PublicKey:   state.PublicKey,
			HostGateway: gateway,
			TargetUser:  user,
			Command:     buildShellCommand(user, state.PublicKey),
			ExpiresAt:   state.ExpiresAt.UTC().Format(time.RFC3339),
		})
	}
}

func activateLocalHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := guardLocalSetup(db); err != nil {
			resp.Fail(c, http.StatusForbidden, 1004, err.Error())
			return
		}

		var state model.SetupState
		if err := db.First(&state, setupStateRowID).Error; err != nil {
			resp.Fail(c, http.StatusBadRequest, 1005, "请先生成密钥")
			return
		}
		if time.Now().After(state.ExpiresAt) {
			resp.Fail(c, http.StatusGone, 1006, "密钥已过期，请重新生成")
			return
		}

		privPEM, err := crypto.Decrypt(state.EncryptedPrivateKey, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "私钥解密失败")
			return
		}

		// Try the dial. SSH host-key TOFU lives in sshpool; we pass serverID=0
		// (ephemeral) so the test doesn't pollute the host-key table — the
		// real Server row will be created right after success and start fresh.
		client, err := sshpool.Dial(state.HostGateway, defaultSSHPort, state.TargetUser, "key", privPEM)
		if err != nil {
			resp.Fail(c, http.StatusBadRequest, 1007,
				fmt.Sprintf("SSH 连接失败 (%s@%s:%d): %v",
					state.TargetUser, state.HostGateway, defaultSSHPort, err))
			return
		}
		_ = client.Close()

		// Persist as a regular SSH server row.
		encPriv, err := crypto.Encrypt(privPEM, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "加密私钥失败")
			return
		}
		now := time.Now()
		server := model.Server{
			Name:        "本机 (SSH)",
			Type:        "ssh",
			Host:        state.HostGateway,
			Port:        defaultSSHPort,
			Username:    state.TargetUser,
			AuthType:    "key",
			PrivateKey:  encPriv,
			Remark:      "ServerHub 本机（向导自动创建）",
			Status:      "online",
			LastCheckAt: &now,
		}
		if err := db.Create(&server).Error; err != nil {
			resp.InternalError(c, "保存服务器失败: "+err.Error())
			return
		}

		// Cleanup transient row.
		db.Where("id = ?", setupStateRowID).Delete(&model.SetupState{})

		resp.OK(c, gin.H{"server_id": server.ID, "name": server.Name})
	}
}

// guardLocalSetup ensures local-server setup is only callable inside a
// container AND after admin creation.
func guardLocalSetup(db *gorm.DB) error {
	if !sysinfo.IsContainerized() {
		return errors.New("非容器环境无需本机纳管")
	}
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count == 0 {
		return errors.New("请先创建管理员账号")
	}
	var sshSelfCount int64
	db.Model(&model.Server{}).
		Where("type = ? AND remark LIKE ?", "ssh", "ServerHub 本机%").
		Count(&sshSelfCount)
	if sshSelfCount > 0 {
		return errors.New("本机已纳管，无需重复初始化")
	}
	return nil
}

func buildShellCommand(user, pubKey string) string {
	pub := strings.TrimSpace(pubKey)
	return strings.Join([]string{
		fmt.Sprintf("sudo mkdir -p /home/%s/.ssh && \\", user),
		fmt.Sprintf("echo '%s' | sudo tee -a /home/%s/.ssh/authorized_keys >/dev/null && \\", pub, user),
		fmt.Sprintf("sudo chown -R %s:%s /home/%s/.ssh && \\", user, user, user),
		fmt.Sprintf("sudo chmod 700 /home/%s/.ssh && sudo chmod 600 /home/%s/.ssh/authorized_keys && \\", user, user),
		fmt.Sprintf("echo '%s ALL=(ALL) NOPASSWD:ALL' | sudo tee /etc/sudoers.d/serverhub-%s >/dev/null && \\", user, user),
		fmt.Sprintf("sudo chmod 440 /etc/sudoers.d/serverhub-%s", user),
	}, "\n")
}

func validUnixUser(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '_':
			// always ok
		case (r >= '0' && r <= '9') || r == '-' || r == '.':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
