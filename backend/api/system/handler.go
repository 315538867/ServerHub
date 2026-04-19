package system

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/system/firewall/rules", firewallListHandler(db, cfg))
	r.POST("/:id/system/firewall/rules", firewallAddHandler(db, cfg))
	r.DELETE("/:id/system/firewall/rules", firewallDelHandler(db, cfg))

	r.GET("/:id/system/cron/jobs", cronListHandler(db, cfg))
	r.POST("/:id/system/cron/jobs", cronAddHandler(db, cfg))
	r.PUT("/:id/system/cron/jobs", cronUpdateHandler(db, cfg))
	r.DELETE("/:id/system/cron/jobs", cronDelHandler(db, cfg))

	r.GET("/:id/system/processes", processListHandler(db, cfg))
	r.DELETE("/:id/system/processes/:pid", processKillHandler(db, cfg))

	r.GET("/:id/system/services", serviceListHandler(db, cfg))
	r.POST("/:id/system/services/:name/action", serviceActionHandler(db, cfg))
	r.GET("/:id/system/services/:name/logs", serviceLogsHandler(db, cfg))
}

// ── common helpers ────────────────────────────────────────────────────────────

func getSSH(c *gin.Context, db *gorm.DB, cfg *config.Config) (*gossh.Client, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, false
	}
	var s model.Server
	if err := db.First(&s, id).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, false
	}
	var cred string
	switch s.AuthType {
	case "key":
		if s.PrivateKey != "" {
			cred, err = crypto.Decrypt(s.PrivateKey, cfg.Security.AESKey)
		}
	default:
		if s.Password != "" {
			cred, err = crypto.Decrypt(s.Password, cfg.Security.AESKey)
		}
	}
	if err != nil {
		resp.InternalError(c, "解密失败")
		return nil, false
	}
	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return nil, false
	}
	return client, true
}

func sq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func streamSSH(ws *websocket.Conn, client *gossh.Client, cmd string) {
	var mu sync.Mutex
	send := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		mu.Lock()
		ws.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
		ws.WriteMessage(websocket.TextMessage, b)              //nolint:errcheck
		mu.Unlock()
	}
	sess, err := client.NewSession()
	if err != nil {
		send(gin.H{"type": "error", "data": err.Error()})
		return
	}
	defer sess.Close()
	stdout, _ := sess.StdoutPipe()
	if err := sess.Start(cmd); err != nil {
		send(gin.H{"type": "error", "data": err.Error()})
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		send(gin.H{"type": "output", "data": scanner.Text()})
	}
	sess.Wait() //nolint:errcheck
	send(gin.H{"type": "done"})
}

func encodeJSON(v any) ([]byte, error) {
	import_json := func() []byte {
		return nil
	}
	_ = import_json
	// Use fmt as a workaround — real encoding via encoding/json below.
	return nil, nil
}

// ── firewall ─────────────────────────────────────────────────────────────────

type FirewallRule struct {
	Index  int    `json:"index"`
	Rule   string `json:"rule"`
	Action string `json:"action"`
	From   string `json:"from"`
	Type   string `json:"type"` // ufw | firewalld
}

func detectFirewall(client *gossh.Client) string {
	out, _ := sshpool.Run(client, "which ufw 2>/dev/null")
	if strings.TrimSpace(out) != "" {
		return "ufw"
	}
	out, _ = sshpool.Run(client, "which firewall-cmd 2>/dev/null")
	if strings.TrimSpace(out) != "" {
		return "firewalld"
	}
	return "unknown"
}

func firewallListHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		fw := detectFirewall(client)
		var rules []FirewallRule
		switch fw {
		case "ufw":
			out, _ := sshpool.Run(client, "ufw status numbered 2>/dev/null")
			idx := 0
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "[") {
					continue
				}
				idx++
				// e.g. "[ 1] 22/tcp                     ALLOW IN    Anywhere"
				parts := strings.SplitN(line, "]", 2)
				if len(parts) < 2 {
					continue
				}
				rest := strings.TrimSpace(parts[1])
				rules = append(rules, FirewallRule{
					Index: idx, Rule: rest, Type: "ufw",
				})
			}
		case "firewalld":
			out, _ := sshpool.Run(client, "firewall-cmd --list-all 2>/dev/null")
			idx := 0
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasSuffix(line, ":") || strings.HasSuffix(line, "(active)") {
					continue
				}
				idx++
				rules = append(rules, FirewallRule{Index: idx, Rule: line, Type: "firewalld"})
			}
		default:
			rules = []FirewallRule{}
		}
		if rules == nil {
			rules = []FirewallRule{}
		}
		resp.OK(c, gin.H{"type": fw, "rules": rules})
	}
}

func firewallAddHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Port   string `json:"port"`
			Proto  string `json:"proto"`
			Action string `json:"action"`
			From   string `json:"from"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		fw := detectFirewall(client)
		var cmd string
		switch fw {
		case "ufw":
			action := "allow"
			if body.Action == "deny" {
				action = "deny"
			}
			if body.From != "" && body.From != "Anywhere" {
				cmd = fmt.Sprintf("ufw %s from %s to any port %s proto %s",
					action, sq(body.From), sq(body.Port), sq(body.Proto))
			} else {
				cmd = fmt.Sprintf("ufw %s %s/%s", action, sq(body.Port), sq(body.Proto))
			}
		case "firewalld":
			cmd = fmt.Sprintf("firewall-cmd --permanent --add-port=%s/%s && firewall-cmd --reload",
				sq(body.Port), sq(body.Proto))
		default:
			resp.InternalError(c, "未检测到支持的防火墙")
			return
		}
		out, err := sshpool.Run(client, cmd)
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func firewallDelHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		rule := c.Query("rule")
		if rule == "" {
			resp.BadRequest(c, "规则不能为空")
			return
		}
		fw := detectFirewall(client)
		var cmd string
		switch fw {
		case "ufw":
			cmd = fmt.Sprintf("yes | ufw delete %s", sq(rule))
		case "firewalld":
			cmd = fmt.Sprintf("firewall-cmd --permanent --remove-port=%s && firewall-cmd --reload", sq(rule))
		default:
			resp.InternalError(c, "未检测到支持的防火墙")
			return
		}
		out, err := sshpool.Run(client, cmd)
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

// ── cron ─────────────────────────────────────────────────────────────────────

type CronJob struct {
	Index int    `json:"index"`
	Expr  string `json:"expr"`
	Cmd   string `json:"cmd"`
	Raw   string `json:"raw"`
}

func parseCrontab(out string) []CronJob {
	var jobs []CronJob
	idx := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		expr := strings.Join(fields[:5], " ")
		cmd := strings.Join(fields[5:], " ")
		jobs = append(jobs, CronJob{Index: idx, Expr: expr, Cmd: cmd, Raw: line})
		idx++
	}
	return jobs
}

func getCrontab(client *gossh.Client) string {
	out, _ := sshpool.Run(client, "crontab -l 2>/dev/null")
	return out
}

func writeCrontab(client *gossh.Client, content string) error {
	_, err := sshpool.Run(client, fmt.Sprintf("echo %s | crontab -", sq(content)))
	return err
}

func cronListHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		jobs := parseCrontab(getCrontab(client))
		if jobs == nil {
			jobs = []CronJob{}
		}
		resp.OK(c, jobs)
	}
}

func cronAddHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Expr string `json:"expr" binding:"required"`
			Cmd  string `json:"cmd"  binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "表达式和命令不能为空")
			return
		}
		current := strings.TrimRight(getCrontab(client), "\n")
		newLine := body.Expr + " " + body.Cmd
		if current != "" {
			current += "\n"
		}
		current += newLine + "\n"
		if err := writeCrontab(client, current); err != nil {
			resp.InternalError(c, "写入定时任务失败")
			return
		}
		resp.OK(c, nil)
	}
}

func cronUpdateHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Index int    `json:"index"`
			Expr  string `json:"expr" binding:"required"`
			Cmd   string `json:"cmd"  binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		jobs := parseCrontab(getCrontab(client))
		if body.Index < 0 || body.Index >= len(jobs) {
			resp.BadRequest(c, "索引无效")
			return
		}
		jobs[body.Index].Expr = body.Expr
		jobs[body.Index].Cmd = body.Cmd
		lines := buildCrontab(jobs)
		if err := writeCrontab(client, lines); err != nil {
			resp.InternalError(c, "写入定时任务失败")
			return
		}
		resp.OK(c, nil)
	}
}

func cronDelHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		idx, err := strconv.Atoi(c.Query("index"))
		if err != nil {
			resp.BadRequest(c, "索引无效")
			return
		}
		jobs := parseCrontab(getCrontab(client))
		if idx < 0 || idx >= len(jobs) {
			resp.BadRequest(c, "索引越界")
			return
		}
		jobs = append(jobs[:idx], jobs[idx+1:]...)
		if err := writeCrontab(client, buildCrontab(jobs)); err != nil {
			resp.InternalError(c, "写入定时任务失败")
			return
		}
		resp.OK(c, nil)
	}
}

func buildCrontab(jobs []CronJob) string {
	var sb strings.Builder
	for _, j := range jobs {
		sb.WriteString(j.Expr + " " + j.Cmd + "\n")
	}
	return sb.String()
}

// ── processes ─────────────────────────────────────────────────────────────────

type ProcessItem struct {
	User    string  `json:"user"`
	PID     string  `json:"pid"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Command string  `json:"command"`
}

func processListHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		out, _ := sshpool.Run(client, "ps aux --sort=-%cpu 2>/dev/null | head -21")
		var items []ProcessItem
		lines := strings.Split(out, "\n")
		for _, line := range lines[1:] { // skip header
			fields := strings.Fields(line)
			if len(fields) < 11 {
				continue
			}
			cpu, _ := strconv.ParseFloat(fields[2], 64)
			mem, _ := strconv.ParseFloat(fields[3], 64)
			items = append(items, ProcessItem{
				User:    fields[0],
				PID:     fields[1],
				CPU:     cpu,
				Mem:     mem,
				Command: strings.Join(fields[10:], " "),
			})
		}
		if items == nil {
			items = []ProcessItem{}
		}
		resp.OK(c, items)
	}
}

func processKillHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		pid := c.Param("pid")
		// Basic safety: pid must be a number
		if _, err := strconv.Atoi(pid); err != nil {
			resp.BadRequest(c, "进程 ID 无效")
			return
		}
		out, err := sshpool.Run(client, "kill -9 "+pid)
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

// ── services ─────────────────────────────────────────────────────────────────

type ServiceItem struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

func serviceListHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		out, _ := sshpool.Run(client, "systemctl list-units --type=service --no-pager --plain --all 2>/dev/null")
		var items []ServiceItem
		for i, line := range strings.Split(out, "\n") {
			if i == 0 {
				continue // skip header
			}
			// Remove leading ● if present
			line = strings.TrimLeft(strings.TrimSpace(line), "●• ")
			if line == "" || strings.HasPrefix(line, "UNIT") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			desc := ""
			if len(fields) > 4 {
				desc = strings.Join(fields[4:], " ")
			}
			items = append(items, ServiceItem{
				Unit:        fields[0],
				Load:        fields[1],
				Active:      fields[2],
				Sub:         fields[3],
				Description: desc,
			})
		}
		if items == nil {
			items = []ServiceItem{}
		}
		resp.OK(c, items)
	}
}

func serviceActionHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		name := c.Param("name")
		var body struct {
			Action string `json:"action"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "操作类型不能为空")
			return
		}
		allowed := map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}
		if !allowed[body.Action] {
			resp.BadRequest(c, "未知操作")
			return
		}
		out, err := sshpool.Run(client, fmt.Sprintf("systemctl %s %s", body.Action, sq(name)))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, gin.H{"output": strings.TrimSpace(out)})
	}
}

func serviceLogsHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, ok := getSSH(c, db, cfg)
		if !ok {
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		name := sq(c.Param("name"))
		go streamSSH(ws, client, fmt.Sprintf("journalctl -u %s -f --no-pager -n 100 2>&1", name))
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	}
}
