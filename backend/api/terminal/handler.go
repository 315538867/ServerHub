package terminal

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/middleware"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/serverhub/serverhub/repo"
	gossh "golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
	upgrader.CheckOrigin = middleware.WSCheckOrigin(cfg)
	r.GET("/:id/terminal", handler(db, cfg))
}

func handler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── auth ──
		// Prefer Sec-WebSocket-Protocol "bearer, <token>" — JWT then doesn't
		// land in proxy access logs or browser history. Fallback to ?token=
		// for the duration of the frontend migration.
		tokenStr, viaSubproto := middleware.ExtractWSToken(c.Request)
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
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token 无效"})
			return
		}
		// Reject the temporary tmp_totp role — those tokens are only for
		// completing the second-factor exchange, not for opening a shell.
		if claims.Role == "tmp_totp" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token 不允许用于终端"})
			return
		}

		// ── find server ──
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "ID 格式错误"})
			return
		}
		s, err := repo.GetServerByID(c.Request.Context(), db, uint(id))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "服务器不存在"})
			return
		}

		// Echo the negotiated subprotocol so the browser handshake succeeds.
		var respHeader http.Header
		if viaSubproto {
			respHeader = http.Header{"Sec-WebSocket-Protocol": []string{"bearer"}}
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, respHeader)
		if err != nil {
			return
		}
		defer ws.Close()

		if s.Type == "local" {
			runLocal(ws)
			return
		}
		runSSH(ws, &s, cfg)
	}
}

// runLocal spawns an interactive shell on the host running ServerHub via PTY.
func runLocal(ws *websocket.Conn) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell, "-i")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		writeText(ws, "error: pty: "+err.Error())
		return
	}
	defer ptmx.Close()
	defer func() { _ = cmd.Process.Kill() }()

	pumpPTY(ws, ptmx, func(rows, cols uint16) {
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
	})
	_ = cmd.Wait()
}

// runSSH proxies the WS to a dedicated SSH PTY session against a remote server.
func runSSH(ws *websocket.Conn, s *domain.Server, cfg *config.Config) {
	var cred string
	var err error
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
		writeText(ws, "error: decrypt: "+err.Error())
		return
	}

	client, err := sshpool.Dial(s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		writeText(ws, "error: ssh: "+err.Error())
		return
	}
	defer client.Close()

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

// pumpPTY shuttles bytes between WS and a local PTY master.
func pumpPTY(ws *websocket.Conn, ptmx *os.File, resize func(rows, cols uint16)) {
	var wsMu sync.Mutex
	writeBin := func(p []byte) {
		wsMu.Lock()
		defer wsMu.Unlock()
		ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		ws.WriteMessage(websocket.BinaryMessage, p) //nolint:errcheck
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
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

	for {
		msgType, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if msgType == websocket.TextMessage {
			var msg resizeMsg
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
				resize(uint16(msg.Rows), uint16(msg.Cols))
			}
		} else {
			if _, err := ptmx.Write(data); err != nil {
				break
			}
		}
	}
	_ = ptmx.Close()
	<-done
}

func writeText(ws *websocket.Conn, msg string) {
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	ws.WriteMessage(websocket.TextMessage, []byte(msg)) //nolint:errcheck
}
