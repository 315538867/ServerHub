package terminal

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client (e.g. curl, native app)
		}
		// Allow same host (panel origin)
		host := r.Host
		return origin == "http://"+host || origin == "https://"+host ||
			origin == "http://localhost:5173" // Vite dev server
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	// JWT is validated inside the handler via ?token= query param.
	// This route must NOT use the Auth middleware (WS handshake can't set headers).
	r.GET("/:id/terminal", handler(db, cfg))
}

func handler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── auth ──────────────────────────────────────────────────
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "missing token"})
			return
		}
		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.Security.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token 无效"})
			return
		}

		// ── find server ───────────────────────────────────────────
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "ID 格式错误"})
			return
		}
		var s model.Server
		if err := db.First(&s, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "服务器不存在"})
			return
		}

		// ── decrypt credentials ───────────────────────────────────
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
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "解密失败"})
			return
		}

		// ── SSH client (reuse pool connection) ────────────────────
		client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "ssh: " + err.Error()})
			return
		}

		// ── upgrade to WebSocket ──────────────────────────────────
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		// ── open SSH session with PTY ─────────────────────────────
		session, err := client.NewSession()
		if err != nil {
			writeText(ws, "error: session: "+err.Error())
			return
		}
		defer session.Close()

		modes := gossh.TerminalModes{
			gossh.ECHO:          1,
			gossh.TTY_OP_ISPEED: 38400,
			gossh.TTY_OP_OSPEED: 38400,
		}
		if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
			writeText(ws, "error: pty: "+err.Error())
			return
		}

		stdin, _ := session.StdinPipe()
		stdout, _ := session.StdoutPipe()
		stderr, _ := session.StderrPipe()

		if err := session.Shell(); err != nil {
			writeText(ws, "error: shell: "+err.Error())
			return
		}

		// ── pipe SSH output → WS (stdout + stderr, concurrent) ───
		var wsMu sync.Mutex
		writeBin := func(p []byte) {
			wsMu.Lock()
			defer wsMu.Unlock()
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			ws.WriteMessage(websocket.BinaryMessage, p) //nolint:errcheck
		}

		var wg sync.WaitGroup
		pipe := func(r io.Reader) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf := make([]byte, 4096)
				for {
					n, err := r.Read(buf)
					if n > 0 {
						cp := make([]byte, n)
						copy(cp, buf[:n])
						writeBin(cp)
					}
					if err != nil {
						return
					}
				}
			}()
		}
		pipe(stdout)
		pipe(stderr)

		// ── WS input → SSH stdin / resize ─────────────────────────
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if msgType == websocket.TextMessage {
				var msg resizeMsg
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
					session.WindowChange(int(msg.Rows), int(msg.Cols))
				}
			} else {
				if _, err := stdin.Write(data); err != nil {
					break
				}
			}
		}

		stdin.Close()
		session.Wait()
		wg.Wait()
	}
}

func writeText(ws *websocket.Conn, msg string) {
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	ws.WriteMessage(websocket.TextMessage, []byte(msg)) //nolint:errcheck
}
