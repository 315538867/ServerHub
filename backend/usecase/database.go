// Package usecase: database.go 收口数据库连接子域业务逻辑。
//
// 包含 DBConn CRUD、getConnCtx（Conn/Server/Runner 解析）、
// MySQL/Redis 远端命令构建。handler 只负责 DTO 解析 / 鉴权 / 调 usecase / 回响应。
//
// TODO R7: 切 ports interface，移除 db *gorm.DB 入参。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/repo"
	"gorm.io/gorm"
)

// ConnContext 封装远端数据库操作所需的全部上下文（连接记录、解密密码、SSH/本地 runner）。
type ConnContext struct {
	Conn   domain.DBConn
	Pass   string // 已解密的明文密码
	Runner runner.Runner
}

// MySQLArgs 构造 mysql CLI 的连接参数（host/port/user），不含密码。
func (cx *ConnContext) MySQLArgs() string {
	host := strings.ToLower(strings.TrimSpace(cx.Conn.Host))
	if host == "" || host == "localhost" {
		return fmt.Sprintf("-u%s", ShellQuote(cx.Conn.Username))
	}
	return fmt.Sprintf("-u%s -h%s -P%d",
		ShellQuote(cx.Conn.Username), ShellQuote(cx.Conn.Host), cx.Conn.Port)
}

// MySQLEnv 构造 MYSQL_PWD 环境变量前缀；无密码时返回空串。
func (cx *ConnContext) MySQLEnv() string {
	if cx.Pass == "" {
		return ""
	}
	return "MYSQL_PWD=" + ShellQuote(cx.Pass) + " "
}

// MySQLCmd 构造一条完整的 mysql 执行命令（含密码环境变量 / 连接参数 / SQL）。
func (cx *ConnContext) MySQLCmd(sql string) string {
	db := ""
	if cx.Conn.Database != "" {
		db = ShellQuote(cx.Conn.Database) + " "
	}
	return fmt.Sprintf("%smysql %s %s--batch --skip-column-names -e %s 2>&1",
		cx.MySQLEnv(), cx.MySQLArgs(), db, ShellQuote(sql))
}

// RedisCli 构造一条 redis-cli 命令；args 为 INFO / KEYS / GET ... 等子命令。
func (cx *ConnContext) RedisCli(args string) string {
	auth := ""
	if cx.Pass != "" {
		auth = "-a " + ShellQuote(cx.Pass) + " --no-auth-warning "
	}
	return fmt.Sprintf("redis-cli -h %s -p %d %s%s 2>&1",
		ShellQuote(cx.Conn.Host), cx.Conn.Port, auth, args)
}

// ShellQuote 用单引号包裹字符串，内部引号用 '\'' 转义。
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ── ConnContext 工厂 ──────────────────────────────────────────────────────────

// GetDBConnContext 根据连接 ID 解析出 ConnContext：查 DBConn → 解密密码 → 查 Server → 创建 runner。
func GetDBConnContext(ctx context.Context, db *gorm.DB, cfg *config.Config, connID uint) (*ConnContext, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	conn, err := repo.GetDBConnByID(ctx, db, connID)
	if err != nil {
		return nil, fmt.Errorf("数据库连接不存在: %w", err)
	}

	var pass string
	if conn.Password != "" {
		pass, err = crypto.Decrypt(conn.Password, cfg.Security.AESKey)
		if err != nil {
			return nil, fmt.Errorf("解密失败: %w", err)
		}
	}

	srv, err := repo.GetServerByID(ctx, db, conn.ServerID)
	if err != nil {
		return nil, fmt.Errorf("服务器不存在: %w", err)
	}

	r, err := runner.For(&srv, cfg)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	return &ConnContext{Conn: conn, Pass: pass, Runner: r}, nil
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// CreateDBConnInput 创建数据库连接的入参。
type CreateDBConnInput struct {
	ServerID uint
	Name     string
	Type     string
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

// CreateDBConn 创建一条数据库连接记录，密码经 AES 加密后落库。
func CreateDBConn(ctx context.Context, db *gorm.DB, cfg *config.Config, input CreateDBConnInput) (domain.DBConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	host := input.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := input.Port
	if port == 0 {
		if input.Type == "redis" {
			port = 6379
		} else {
			port = 3306
		}
	}
	encPass := ""
	if input.Password != "" {
		var err error
		encPass, err = crypto.Encrypt(input.Password, cfg.Security.AESKey)
		if err != nil {
			return domain.DBConn{}, fmt.Errorf("加密失败: %w", err)
		}
	}
	conn := domain.DBConn{
		ServerID: input.ServerID, Name: input.Name, Type: input.Type,
		Host: host, Port: port, Username: input.Username,
		Password: encPass, Database: input.Database,
	}
	if err := repo.CreateDBConn(ctx, db, &conn); err != nil {
		return domain.DBConn{}, fmt.Errorf("创建失败: %w", err)
	}
	return conn, nil
}

// GetDBConn 按 ID 查询数据库连接。
func GetDBConn(ctx context.Context, db *gorm.DB, id uint) (domain.DBConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.GetDBConnByID(ctx, db, id)
}

// UpdateDBConnInput 更新数据库连接的入参（仅非零值会被写入，空串=保留原值，空密码=不改）。
type UpdateDBConnInput struct {
	Name     string
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

// UpdateDBConn 部分更新数据库连接，新密码会重新加密。
func UpdateDBConn(ctx context.Context, db *gorm.DB, cfg *config.Config, id uint, input UpdateDBConnInput) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	conn, err := repo.GetDBConnByID(ctx, db, id)
	if err != nil {
		return err
	}
	if input.Name != "" {
		conn.Name = input.Name
	}
	if input.Host != "" {
		conn.Host = input.Host
	}
	if input.Port != 0 {
		conn.Port = input.Port
	}
	if input.Username != "" {
		conn.Username = input.Username
	}
	if input.Database != "" {
		conn.Database = input.Database
	}
	if input.Password != "" {
		enc, err := crypto.Encrypt(input.Password, cfg.Security.AESKey)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		conn.Password = enc
	}
	return repo.SaveDBConn(ctx, db, &conn)
}

// DeleteDBConn 按 ID 删除数据库连接。
func DeleteDBConn(ctx context.Context, db *gorm.DB, id uint) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.DeleteDBConn(ctx, db, id)
}

// ListDBConns 按可选条件查询数据库连接；serverID=0 时不限制，appID=nil 时不限制。
func ListDBConns(ctx context.Context, db *gorm.DB, serverID uint, appID *uint) ([]domain.DBConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return repo.ListDBConns(ctx, db, serverID, appID)
}
