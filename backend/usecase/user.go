// Package usecase: user.go 收口 identity 子域业务逻辑。
//
// 包含登录、读取/更新 profile、TOTP 启停与校验、首管理员初始化。
// handler 只负责 DTO 解析 / token 解析 / 回响应。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/auditq"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/totp"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrOldPasswordWrong   = errors.New("旧密码错误")
	ErrUsernameExists     = errors.New("用户名已存在")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrMFASecretInvalid   = errors.New("TOTP 密钥无效")
	ErrTOTPCodeInvalid    = errors.New("验证码错误")
	ErrTOTPCodeReplayed   = errors.New("验证码已被使用")
	ErrAdminAlreadySetup  = errors.New("已经初始化过管理员")
)

// LoginResult 描述登录结果：要么直接返回 token，要么要求二步 TOTP。
type LoginResult struct {
	RequireTOTP bool
	TmpToken    string
	Token       string
	User        domain.User
}

func Login(ctx context.Context, db *gorm.DB, cfg *config.Config, username, password, clientIP string,
	signToken func(*domain.User, string) (string, error),
	signTmpToken func(*domain.User, string) (string, error),
) (LoginResult, int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if middleware.AccountLocked(username) {
		auditq.Security(username, clientIP, "security:account_locked", 429, nil)
		return LoginResult{}, http.StatusTooManyRequests, 1005, errors.New("账号暂时锁定，请稍后再试")
	}

	user, err := repo.GetUserByUsername(ctx, db, username)
	if err != nil {
		middleware.RecordAccountFailure(username)
		auditq.Security(username, clientIP, "security:login_failed", 401,
			map[string]any{"reason": "user_not_found"})
		return LoginResult{}, http.StatusUnauthorized, 1001, ErrInvalidCredentials
	}
	if !crypto.BcryptVerify(password, user.Password) {
		middleware.RecordAccountFailure(username)
		auditq.Security(username, clientIP, "security:login_failed", 401,
			map[string]any{"reason": "bad_password"})
		return LoginResult{}, http.StatusUnauthorized, 1001, ErrInvalidCredentials
	}

	middleware.RecordAccountSuccess(username)
	if user.MFAEnabled && user.MFASecret != "" {
		tmpToken, err := signTmpToken(&user, cfg.Security.JWTSecret)
		if err != nil {
			return LoginResult{}, http.StatusInternalServerError, 0, errors.New("生成 Token 失败")
		}
		return LoginResult{RequireTOTP: true, TmpToken: tmpToken}, 0, 0, nil
	}

	token, err := signToken(&user, cfg.Security.JWTSecret)
	if err != nil {
		return LoginResult{}, http.StatusInternalServerError, 0, errors.New("生成 Token 失败")
	}
	now := time.Now()
	if err := repo.UpdateUserLoginMeta(ctx, db, user.ID, &now, clientIP); err == nil {
		user.LastLogin = &now
		user.LastIP = clientIP
	}
	return LoginResult{Token: token, User: user}, 0, 0, nil
}

func GetCurrentUser(ctx context.Context, db *gorm.DB, userID uint) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return repo.GetUserByID(ctx, db, userID)
}

func UpdateProfile(ctx context.Context, db *gorm.DB, userID uint, oldPassword, newUsername, newPassword string) (domain.User, int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	user, err := repo.GetUserByID(ctx, db, userID)
	if err != nil {
		return domain.User{}, http.StatusNotFound, 0, ErrUserNotFound
	}
	if !crypto.BcryptVerify(oldPassword, user.Password) {
		return domain.User{}, http.StatusUnprocessableEntity, 1002, ErrOldPasswordWrong
	}

	updates := map[string]any{}
	if newUsername != "" && newUsername != user.Username {
		count, err := repo.CountUsersByUsernameExcludingID(ctx, db, newUsername, user.ID)
		if err != nil {
			return domain.User{}, http.StatusInternalServerError, 0, err
		}
		if count > 0 {
			return domain.User{}, http.StatusConflict, 1003, ErrUsernameExists
		}
		updates["username"] = newUsername
	}
	if newPassword != "" {
		hash, err := crypto.BcryptHash(newPassword)
		if err != nil {
			return domain.User{}, http.StatusInternalServerError, 0, errors.New("密码加密失败")
		}
		updates["password"] = hash
	}
	if len(updates) > 0 {
		if err := repo.UpdateUserFields(ctx, db, user.ID, updates); err != nil {
			return domain.User{}, http.StatusInternalServerError, 0, errors.New("更新失败")
		}
	}
	updated, err := repo.GetUserByID(ctx, db, userID)
	if err != nil {
		return domain.User{}, http.StatusNotFound, 0, ErrUserNotFound
	}
	return updated, 0, 0, nil
}

func ConfirmTOTP(ctx context.Context, db *gorm.DB, cfg *config.Config, userID uint, secret, code string) (int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if !totp.Verify(secret, code) {
		return http.StatusUnprocessableEntity, 1010, errors.New("验证码格式错误")
	}
	encSecret, err := crypto.Encrypt(secret, cfg.Security.AESKey)
	if err != nil {
		return http.StatusInternalServerError, 0, errors.New("加密失败")
	}
	if err := repo.UpdateUserMFAMeta(ctx, db, userID, encSecret, true); err != nil {
		return http.StatusInternalServerError, 0, err
	}
	return 0, 0, nil
}

func DisableTOTP(ctx context.Context, db *gorm.DB, userID uint) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return repo.ClearUserMFAMeta(ctx, db, userID)
}

func LoginWithTOTP(ctx context.Context, db *gorm.DB, cfg *config.Config, tmpToken, code, clientIP string,
	parseToken func(string, string) (*middleware.Claims, error),
	signToken func(*domain.User, string) (string, error),
) (LoginResult, int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	claims, err := parseToken(tmpToken, cfg.Security.JWTSecret)
	if err != nil || claims.Role != "tmp_totp" {
		auditq.Security("", clientIP, "security:mfa_token_invalid", 401, nil)
		return LoginResult{}, http.StatusUnauthorized, 1001, errors.New("Token 无效")
	}

	user, err := repo.GetUserByID(ctx, db, claims.UserID)
	if err != nil {
		auditq.Security(claims.Username, clientIP, "security:mfa_token_invalid", 401,
			map[string]any{"reason": "user_not_found"})
		return LoginResult{}, http.StatusUnauthorized, 1001, ErrUserNotFound
	}
	secret, err := crypto.Decrypt(user.MFASecret, cfg.Security.AESKey)
	if err != nil {
		auditq.Security(user.Username, clientIP, "security:totp_failed", 401,
			map[string]any{"reason": "secret_decrypt_failed"})
		return LoginResult{}, http.StatusUnauthorized, 1011, ErrMFASecretInvalid
	}
	step, ok := totp.VerifyAt(secret, code, time.Now())
	if !ok {
		auditq.Security(user.Username, clientIP, "security:totp_failed", 401,
			map[string]any{"reason": "bad_code"})
		return LoginResult{}, http.StatusUnauthorized, 1011, ErrTOTPCodeInvalid
	}
	rows, err := repo.AdvanceUserLastTOTPStep(ctx, db, user.ID, step)
	if err != nil || rows == 0 {
		auditq.Security(user.Username, clientIP, "security:totp_replay", 401,
			map[string]any{"step": step})
		return LoginResult{}, http.StatusUnauthorized, 1011, ErrTOTPCodeReplayed
	}
	user.LastTOTPStep = step
	token, err := signToken(&user, cfg.Security.JWTSecret)
	if err != nil {
		return LoginResult{}, http.StatusInternalServerError, 0, errors.New("生成 Token 失败")
	}
	now := time.Now()
	if err := repo.UpdateUserLoginMeta(ctx, db, user.ID, &now, clientIP); err == nil {
		user.LastLogin = &now
		user.LastIP = clientIP
	}
	return LoginResult{Token: token, User: user}, 0, 0, nil
}

func SetupStatus(ctx context.Context, db *gorm.DB) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	count, err := repo.CountUsers(ctx, db)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func CreateFirstAdmin(ctx context.Context, db *gorm.DB, username, password, clientIP string) (domain.User, int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	username = strings.TrimSpace(username)
	if len(username) < 3 || len(password) < 6 {
		return domain.User{}, http.StatusBadRequest, 0, errors.New("用户名至少 3 字符，密码至少 6 字符")
	}
	count, err := repo.CountUsers(ctx, db)
	if err != nil {
		return domain.User{}, http.StatusInternalServerError, 0, err
	}
	if count > 0 {
		auditq.Security(username, clientIP, "security:setup_admin_blocked", 409, nil)
		return domain.User{}, http.StatusConflict, 1003, ErrAdminAlreadySetup
	}
	phash, err := crypto.BcryptHash(password)
	if err != nil {
		return domain.User{}, http.StatusInternalServerError, 0, errors.New("密码加密失败")
	}
	now := time.Now()
	user := domain.User{
		Username:  username,
		Password:  phash,
		Role:      "admin",
		LastLogin: &now,
	}
	if err := repo.CreateUser(ctx, db, &user); err != nil {
		return domain.User{}, http.StatusInternalServerError, 0, errors.New("创建管理员失败: " + err.Error())
	}
	auditq.Security(username, clientIP, "security:setup_admin_created", 200, nil)
	return user, 0, 0, nil
}
