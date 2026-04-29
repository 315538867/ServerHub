package database

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/usecase"
	"github.com/serverhub/serverhub/repo"
)

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
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

// ── DB conn context helper ──────────────────────────────────────────────────

func getConnCtx(c *gin.Context, db repo.DB, cfg *config.Config) (*usecase.ConnContext, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "ID 格式错误")
		return nil, false
	}
	cx, err := usecase.GetDBConnContext(c.Request.Context(), db, cfg, uint(id))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			resp.NotFound(c, err.Error())
		} else {
			resp.Fail(c, http.StatusServiceUnavailable, 5003, err.Error())
		}
		return nil, false
	}
	return cx, true
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func listConnsHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID, _ := strconv.Atoi(c.Query("server_id"))
		var appID *uint
		if s := c.Query("application_id"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				u := uint(v)
				appID = &u
			}
		}
		conns, err := usecase.ListDBConns(c.Request.Context(), db, uint(serverID), appID)
		if err != nil {
			resp.InternalError(c, "查询失败")
			return
		}
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

func createConnHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
		conn, err := usecase.CreateDBConn(c.Request.Context(), db, cfg, usecase.CreateDBConnInput{
			ServerID: body.ServerID, Name: body.Name, Type: body.Type,
			Host: body.Host, Port: body.Port, Username: body.Username,
			Password: body.Password, Database: body.Database,
		})
		if err != nil {
			resp.InternalError(c, err.Error())
			return
		}
		resp.OK(c, gin.H{"id": conn.ID})
	}
}

func getConnHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		conn, err := usecase.GetDBConn(c.Request.Context(), db, uint(id))
		if err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		resp.OK(c, gin.H{
			"id": conn.ID, "server_id": conn.ServerID, "name": conn.Name, "type": conn.Type,
			"host": conn.Host, "port": conn.Port, "username": conn.Username, "database": conn.Database,
		})
	}
}

func updateConnHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
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
		err := usecase.UpdateDBConn(c.Request.Context(), db, cfg, uint(id), usecase.UpdateDBConnInput{
			Name: body.Name, Host: body.Host, Port: body.Port,
			Username: body.Username, Password: body.Password, Database: body.Database,
		})
		if err != nil {
			resp.NotFound(c, "资源不存在")
			return
		}
		resp.OK(c, nil)
	}
}

func deleteConnHandler(db repo.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		_ = usecase.DeleteDBConn(c.Request.Context(), db, uint(id))
		resp.OK(c, nil)
	}
}

func testConnHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		var cmd string
		if cx.Conn.Type == "redis" {
			cmd = cx.RedisCli("PING")
		} else {
			cmd = cx.MySQLCmd("SELECT 1")
		}
		out, err := cx.Runner.Run(cmd)
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

func mysqlRun(cx *usecase.ConnContext, sql string) (string, error) {
	return cx.Runner.Run(cx.MySQLCmd(sql))
}

func mysqlListDatabases(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func mysqlCreateDatabase(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func mysqlDropDatabase(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func mysqlListUsers(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func mysqlCreateUser(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func mysqlQuery(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
		useDB := cx.Conn.Database
		if body.Database != "" {
			useDB = body.Database
		}
		dbPrefix := ""
		if useDB != "" {
			dbPrefix = fmt.Sprintf("USE `%s`; ", strings.ReplaceAll(useDB, "`", ""))
		}
		cmd := fmt.Sprintf("%smysql %s --batch -e %s 2>&1",
			cx.MySQLEnv(), cx.MySQLArgs(), usecase.ShellQuote(dbPrefix+body.SQL))
		out, err := cx.Runner.Run(cmd)
		if err != nil || strings.Contains(out, "ERROR") {
			resp.Fail(c, 200, 5020, strings.TrimSpace(out))
			return
		}
		cols, rows := parseMySQLTable(out)
		resp.OK(c, gin.H{"columns": cols, "rows": rows})
	}
}

func mysqlExport(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		dbName := c.Param("dbname")
		cmd := fmt.Sprintf("%smysqldump -h%s -P%d -u%s %s 2>/dev/null",
			cx.MySQLEnv(), usecase.ShellQuote(cx.Conn.Host), cx.Conn.Port,
			usecase.ShellQuote(cx.Conn.Username), usecase.ShellQuote(dbName))
		out, err := cx.Runner.Run(cmd)
		if err != nil {
			resp.InternalError(c, "数据库导出失败")
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.sql"`, dbName))
		c.Data(http.StatusOK, "application/sql", []byte(out))
	}
}

func mysqlStatus(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func redisRun(cx *usecase.ConnContext, args string) (string, error) {
	return cx.Runner.Run(cx.RedisCli(args))
}

func redisInfo(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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

func redisKeys(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		pattern := c.DefaultQuery("pattern", "*")
		out, err := redisRun(cx, fmt.Sprintf("--scan --pattern %s --count 100", usecase.ShellQuote(pattern)))
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

func redisGetKey(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		key := strings.TrimPrefix(c.Param("key"), "/")
		typeOut, _ := redisRun(cx, fmt.Sprintf("TYPE %s", usecase.ShellQuote(key)))
		keyType := strings.TrimSpace(typeOut)
		var val string
		switch keyType {
		case "string":
			val, _ = redisRun(cx, fmt.Sprintf("GET %s", usecase.ShellQuote(key)))
		case "hash":
			val, _ = redisRun(cx, fmt.Sprintf("HGETALL %s", usecase.ShellQuote(key)))
		case "list":
			val, _ = redisRun(cx, fmt.Sprintf("LRANGE %s 0 99", usecase.ShellQuote(key)))
		case "set":
			val, _ = redisRun(cx, fmt.Sprintf("SMEMBERS %s", usecase.ShellQuote(key)))
		case "zset":
			val, _ = redisRun(cx, fmt.Sprintf("ZRANGE %s 0 99 WITHSCORES", usecase.ShellQuote(key)))
		}
		ttl, _ := redisRun(cx, fmt.Sprintf("TTL %s", usecase.ShellQuote(key)))
		resp.OK(c, gin.H{"type": keyType, "value": strings.TrimSpace(val), "ttl": strings.TrimSpace(ttl)})
	}
}

func redisDelKey(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cx, ok := getConnCtx(c, db, cfg)
		if !ok {
			return
		}
		key := strings.TrimPrefix(c.Param("key"), "/")
		redisRun(cx, fmt.Sprintf("DEL %s", usecase.ShellQuote(key))) //nolint:errcheck
		resp.OK(c, nil)
	}
}

func redisFlushDB(db repo.DB, cfg *config.Config) gin.HandlerFunc {
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
