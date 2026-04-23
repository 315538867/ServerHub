package database

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	g := r.Group("/dbconns")
	g.GET("", listConnsHandler(db))
	g.POST("", createConnHandler(db, cfg))
	g.GET("/:id", getConnHandler(db))
	g.PUT("/:id", updateConnHandler(db, cfg))
	g.DELETE("/:id", deleteConnHandler(db))

	g.POST("/:id/test", testConnHandler(db, cfg))

	// MySQL routes
	g.GET("/:id/mysql/databases", mysqlListDatabases(db, cfg))
	g.POST("/:id/mysql/databases", mysqlCreateDatabase(db, cfg))
	g.DELETE("/:id/mysql/databases/:dbname", mysqlDropDatabase(db, cfg))
	g.GET("/:id/mysql/users", mysqlListUsers(db, cfg))
	g.POST("/:id/mysql/users", mysqlCreateUser(db, cfg))
	g.POST("/:id/mysql/query", mysqlQuery(db, cfg))
	g.GET("/:id/mysql/export/:dbname", mysqlExport(db, cfg))
	g.GET("/:id/mysql/status", mysqlStatus(db, cfg))

	// Redis routes
	g.GET("/:id/redis/info", redisInfo(db, cfg))
	g.GET("/:id/redis/keys", redisKeys(db, cfg))
	g.GET("/:id/redis/keys/*key", redisGetKey(db, cfg))
	g.DELETE("/:id/redis/keys/*key", redisDelKey(db, cfg))
	g.POST("/:id/redis/flushdb", redisFlushDB(db, cfg))
}

// ── conn helpers ──────────────────────────────────────────────────────────────

type connCtx struct {
	conn   model.DBConn
	pass   string
	client *gossh.Client
}

func getConnCtx(c *gin.Context, db *gorm.DB, cfg *config.Config) (*connCtx, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return nil, false
	}
	var conn model.DBConn
	if err := db.First(&conn, id).Error; err != nil {
		resp.NotFound(c, "数据库连接不存在")
		return nil, false
	}
	var pass string
	if conn.Password != "" {
		pass, err = crypto.Decrypt(conn.Password, cfg.Security.AESKey)
		if err != nil {
			resp.InternalError(c, "解密失败")
			return nil, false
		}
	}
	var srv model.Server
	if err := db.First(&srv, conn.ServerID).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, false
	}
	var cred string
	switch srv.AuthType {
	case "key":
		if srv.PrivateKey != "" {
			cred, err = crypto.Decrypt(srv.PrivateKey, cfg.Security.AESKey)
		}
	default:
		if srv.Password != "" {
			cred, err = crypto.Decrypt(srv.Password, cfg.Security.AESKey)
		}
	}
	if err != nil {
		resp.InternalError(c, "解密服务器凭据失败")
		return nil, false
	}
	client, err := sshpool.Connect(srv.ID, srv.Host, srv.Port, srv.Username, srv.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, false
	}
	return &connCtx{conn: conn, pass: pass, client: client}, true
}

func (cx *connCtx) mysqlArgs() string {
	// Host == "localhost" (or empty) → let mysql client pick the default unix
	// socket. That's the only way to satisfy `'user'@'localhost'` grants, which
	// are *not* matched by TCP to 127.0.0.1. Passing `-h localhost -P 3306`
	// would force a TCP attempt and break this case.
	host := strings.ToLower(strings.TrimSpace(cx.conn.Host))
	if host == "" || host == "localhost" {
		return fmt.Sprintf("-u%s", shellQuote(cx.conn.Username))
	}
	return fmt.Sprintf("-u%s -h%s -P%d",
		shellQuote(cx.conn.Username), shellQuote(cx.conn.Host), cx.conn.Port)
}

func (cx *connCtx) mysqlEnv() string {
	if cx.pass == "" {
		return ""
	}
	return "MYSQL_PWD=" + shellQuote(cx.pass) + " "
}

func (cx *connCtx) mysqlCmd(sql string) string {
	db := ""
	if cx.conn.Database != "" {
		db = shellQuote(cx.conn.Database) + " "
	}
	return fmt.Sprintf("%smysql %s %s--batch --skip-column-names -e %s 2>&1",
		cx.mysqlEnv(), cx.mysqlArgs(), db, shellQuote(sql))
}

func (cx *connCtx) redisCli(args string) string {
	auth := ""
	if cx.pass != "" {
		auth = "-a " + shellQuote(cx.pass) + " --no-auth-warning "
	}
	return fmt.Sprintf("redis-cli -h %s -p %d %s%s 2>&1",
		shellQuote(cx.conn.Host), cx.conn.Port, auth, args)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func listConnsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID := c.Query("server_id")
		appID := c.Query("application_id")
		var conns []model.DBConn
		q := db.Model(&model.DBConn{})
		if serverID != "" {
			q = q.Where("server_id = ?", serverID)
		}
		if appID != "" {
			q = q.Where("application_id = ?", appID)
		}
		q.Find(&conns)
		type item struct {
			ID            uint   `json:"id"`
			ServerID      uint   `json:"server_id"`
			ApplicationID *uint  `json:"application_id"`
			Name          string `json:"name"`
			Type          string `json:"type"`
			Host          string `json:"host"`
			Port          int    `json:"port"`
			Username      string `json:"username"`
			Database      string `json:"database"`
		}
		result := make([]item, len(conns))
		for i, c := range conns {
			result[i] = item{ID: c.ID, ServerID: c.ServerID, ApplicationID: c.ApplicationID,
				Name: c.Name, Type: c.Type,
				Host: c.Host, Port: c.Port, Username: c.Username, Database: c.Database}
		}
		resp.OK(c, result)
	}
}

func createConnHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ServerID uint   `json:"server_id" binding:"required"`
			Name     string `json:"name"      binding:"required"`
			Type     string `json:"type"      binding:"required"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Database string `json:"database"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "server_id、名称和类型不能为空")
			return
		}
		if body.Host == "" {
			body.Host = "127.0.0.1"
		}
		if body.Port == 0 {
			if body.Type == "redis" {
				body.Port = 6379
			} else {
				body.Port = 3306
			}
		}
		encPass := ""
		if body.Password != "" {
			var err error
			encPass, err = crypto.Encrypt(body.Password, cfg.Security.AESKey)
			if err != nil {
				resp.InternalError(c, "加密失败")
				return
			}
		}
		conn := model.DBConn{
			ServerID: body.ServerID, Name: body.Name, Type: body.Type,
			Host: body.Host, Port: body.Port, Username: body.Username,
			Password: encPass, Database: body.Database,
		}
		if err := db.Create(&conn).Error; err != nil {
			resp.InternalError(c, "创建失败")
			return
		}
		resp.OK(c, gin.H{"id": conn.ID})
	}
}

func getConnHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		var conn model.DBConn
		if err := db.First(&conn, id).Error; err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		resp.OK(c, gin.H{
			"id": conn.ID, "server_id": conn.ServerID, "name": conn.Name, "type": conn.Type,
			"host": conn.Host, "port": conn.Port, "username": conn.Username, "database": conn.Database,
		})
	}
}

func updateConnHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		var conn model.DBConn
		if err := db.First(&conn, id).Error; err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		var body struct {
			Name     string `json:"name"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Database string `json:"database"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if body.Name != "" {
			conn.Name = body.Name
		}
		if body.Host != "" {
			conn.Host = body.Host
		}
		if body.Port != 0 {
			conn.Port = body.Port
		}
		if body.Username != "" {
			conn.Username = body.Username
		}
		if body.Database != "" {
			conn.Database = body.Database
		}
		if body.Password != "" {
			enc, err := crypto.Encrypt(body.Password, cfg.Security.AESKey)
			if err != nil {
				resp.InternalError(c, "加密失败")
				return
			}
			conn.Password = enc
		}
		db.Save(&conn)
		resp.OK(c, nil)
	}
}

func deleteConnHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		db.Delete(&model.DBConn{}, id)
		resp.OK(c, nil)
	}
}

func testConnHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var cmd string
		if cx.conn.Type == "redis" {
			cmd = cx.redisCli("PING")
		} else {
			cmd = cx.mysqlCmd("SELECT 1")
		}
		out, err := sshpool.Run(cx.client, cmd)
		if err != nil {
			resp.Fail(c, 200, 5010, "连接失败: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

// ── MySQL ─────────────────────────────────────────────────────────────────────

func parseMySQLTable(out string) (columns []string, rows [][]string) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return
	}
	columns = strings.Split(lines[0], "\t")
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	return
}

func mysqlRun(cx *connCtx, sql string) (string, error) {
	return sshpool.Run(cx.client, cx.mysqlCmd(sql))
}

func mysqlListDatabases(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		out, err := mysqlRun(cx, "SHOW DATABASES")
		if err != nil {
			resp.InternalError(c, out)
			return
		}
		var dbs []string
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				dbs = append(dbs, line)
			}
		}
		resp.OK(c, dbs)
	}
}

func mysqlCreateDatabase(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Name    string `json:"name" binding:"required"`
			Charset string `json:"charset"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "名称不能为空")
			return
		}
		charset := body.Charset
		if charset == "" {
			charset = "utf8mb4"
		}
		sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s_unicode_ci",
			strings.ReplaceAll(body.Name, "`", ""), charset, charset)
		out, err := mysqlRun(cx, sql)
		if err != nil || strings.Contains(out, "ERROR") {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func mysqlDropDatabase(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		dbName := c.Param("dbname")
		sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", strings.ReplaceAll(dbName, "`", ""))
		out, err := mysqlRun(cx, sql)
		if err != nil || strings.Contains(out, "ERROR") {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func mysqlListUsers(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		out, err := mysqlRun(cx, "SELECT user, host FROM mysql.user ORDER BY user")
		if err != nil {
			resp.InternalError(c, out)
			return
		}
		_, rows := parseMySQLTable(out)
		type userItem struct {
			User string `json:"user"`
			Host string `json:"host"`
		}
		result := make([]userItem, 0, len(rows))
		for _, r := range rows {
			if len(r) >= 2 {
				result = append(result, userItem{User: r[0], Host: r[1]})
			}
		}
		resp.OK(c, result)
	}
}

func mysqlCreateUser(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			User     string `json:"user"     binding:"required"`
			Host     string `json:"host"`
			Password string `json:"password" binding:"required"`
			Database string `json:"database"`
			Grant    string `json:"grant"` // ALL / SELECT / etc
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "用户名和密码不能为空")
			return
		}
		if body.Host == "" {
			body.Host = "%"
		}
		grant := body.Grant
		if grant == "" {
			grant = "ALL PRIVILEGES"
		}
		dbTarget := "*.*"
		if body.Database != "" {
			dbTarget = fmt.Sprintf("`%s`.*", strings.ReplaceAll(body.Database, "`", ""))
		}
		userEsc := strings.ReplaceAll(body.User, "'", "''")
		hostEsc := strings.ReplaceAll(body.Host, "'", "''")
		passEsc := strings.ReplaceAll(body.Password, "'", "''")
		sql := fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'; GRANT %s ON %s TO '%s'@'%s'; FLUSH PRIVILEGES",
			userEsc, hostEsc, passEsc, grant, dbTarget, userEsc, hostEsc,
		)
		out, err := mysqlRun(cx, sql)
		if err != nil || strings.Contains(out, "ERROR") {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func mysqlQuery(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			SQL      string `json:"sql"      binding:"required"`
			Database string `json:"database"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "SQL 语句不能为空")
			return
		}
		useDB := cx.conn.Database
		if body.Database != "" {
			useDB = body.Database
		}
		dbPrefix := ""
		if useDB != "" {
			dbPrefix = fmt.Sprintf("USE `%s`; ", strings.ReplaceAll(useDB, "`", ""))
		}
		cmd := fmt.Sprintf("%smysql %s --batch -e %s 2>&1",
			cx.mysqlEnv(), cx.mysqlArgs(), shellQuote(dbPrefix+body.SQL))
		out, err := sshpool.Run(cx.client, cmd)
		if err != nil || strings.Contains(out, "ERROR") {
			resp.Fail(c, 200, 5020, strings.TrimSpace(out))
			return
		}
		cols, rows := parseMySQLTable(out)
		resp.OK(c, gin.H{"columns": cols, "rows": rows})
	}
}

func mysqlExport(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		dbName := c.Param("dbname")
		cmd := fmt.Sprintf("%smysqldump -h%s -P%d -u%s %s 2>/dev/null",
			cx.mysqlEnv(), shellQuote(cx.conn.Host), cx.conn.Port,
			shellQuote(cx.conn.Username), shellQuote(dbName))
		out, err := sshpool.Run(cx.client, cmd)
		if err != nil {
			resp.InternalError(c, "数据库导出失败")
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.sql"`, dbName))
		c.Data(http.StatusOK, "application/sql", []byte(out))
	}
}

func mysqlStatus(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		out, err := mysqlRun(cx, "SHOW GLOBAL STATUS")
		if err != nil {
			resp.InternalError(c, out)
			return
		}
		cols, rows := parseMySQLTable(out)
		resp.OK(c, gin.H{"columns": cols, "rows": rows})
	}
}

// ── Redis ─────────────────────────────────────────────────────────────────────

func redisRun(cx *connCtx, args string) (string, error) {
	return sshpool.Run(cx.client, cx.redisCli(args))
}

func redisInfo(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		out, err := redisRun(cx, "INFO")
		if err != nil {
			resp.InternalError(c, out)
			return
		}
		info := make(map[string]string)
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info[parts[0]] = parts[1]
			}
		}
		resp.OK(c, info)
	}
}

func redisKeys(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		pattern := c.DefaultQuery("pattern", "*")
		out, err := redisRun(cx, fmt.Sprintf("--scan --pattern %s --count 100", shellQuote(pattern)))
		if err != nil {
			resp.InternalError(c, out)
			return
		}
		var keys []string
		for _, k := range strings.Split(strings.TrimSpace(out), "\n") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
				if len(keys) >= 200 {
					break
				}
			}
		}
		resp.OK(c, keys)
	}
}

func redisGetKey(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		key := strings.TrimPrefix(c.Param("key"), "/")
		typeOut, _ := redisRun(cx, fmt.Sprintf("TYPE %s", shellQuote(key)))
		keyType := strings.TrimSpace(typeOut)
		var val string
		switch keyType {
		case "string":
			val, _ = redisRun(cx, fmt.Sprintf("GET %s", shellQuote(key)))
		case "hash":
			val, _ = redisRun(cx, fmt.Sprintf("HGETALL %s", shellQuote(key)))
		case "list":
			val, _ = redisRun(cx, fmt.Sprintf("LRANGE %s 0 99", shellQuote(key)))
		case "set":
			val, _ = redisRun(cx, fmt.Sprintf("SMEMBERS %s", shellQuote(key)))
		case "zset":
			val, _ = redisRun(cx, fmt.Sprintf("ZRANGE %s 0 99 WITHSCORES", shellQuote(key)))
		}
		ttl, _ := redisRun(cx, fmt.Sprintf("TTL %s", shellQuote(key)))
		resp.OK(c, gin.H{"type": keyType, "value": strings.TrimSpace(val), "ttl": strings.TrimSpace(ttl)})
	}
}

func redisDelKey(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		key := strings.TrimPrefix(c.Param("key"), "/")
		redisRun(cx, fmt.Sprintf("DEL %s", shellQuote(key))) //nolint:errcheck
		resp.OK(c, nil)
	}
}

func redisFlushDB(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Confirm string `json:"confirm"`
		}
		c.ShouldBindJSON(&body) //nolint:errcheck
		if body.Confirm != "FLUSHDB" {
			resp.BadRequest(c, `请输入 "FLUSHDB" 确认清空`)
			return
		}
		redisRun(cx, "FLUSHDB") //nolint:errcheck
		resp.OK(c, nil)
	}
}
