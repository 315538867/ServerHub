// Package wsstream pipes a remote SSH command's stdout into a WebSocket,
// with optional include/exclude line filtering, large line tolerance, and
// proper goroutine cleanup on either side hanging up.
package wsstream

import (
	"bufio"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

const (
	maxLineBytes  = 4 * 1024 * 1024
	chanBuffer    = 256
	writeTimeout  = 10 * time.Second
	pingInterval  = 30 * time.Second
	pongDeadline  = 90 * time.Second
)

// Opts carries optional line-level filters.
type Opts struct {
	Include       string
	Exclude       string
	Regex         bool
	CaseSensitive bool
}

type msg struct {
	Type  string `json:"type"`
	Data  string `json:"data,omitempty"`
	Count int64  `json:"count,omitempty"`
}

// Stream runs cmd over an existing SSH client and forwards stdout/stderr to ws.
// It blocks until the session ends or either peer disconnects.
func Stream(ws *websocket.Conn, client *gossh.Client, cmd string, opts Opts) {
	var writeMu sync.Mutex
	send := func(m msg) error {
		b, _ := json.Marshal(m)
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = ws.SetWriteDeadline(time.Now().Add(writeTimeout))
		return ws.WriteMessage(websocket.TextMessage, b)
	}

	matcher, err := compileMatcher(opts)
	if err != nil {
		_ = send(msg{Type: "error", Data: "filter: " + err.Error()})
		return
	}

	sess, err := client.NewSession()
	if err != nil {
		_ = send(msg{Type: "error", Data: err.Error()})
		return
	}
	defer sess.Close()

	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = send(msg{Type: "error", Data: err.Error()})
		return
	}
	sess.Stderr = nil

	if err := sess.Start(cmd); err != nil {
		_ = send(msg{Type: "error", Data: err.Error()})
		return
	}

	lines := make(chan string, chanBuffer)
	var dropped int64
	done := make(chan struct{})

	// reader: scan SSH stdout, filter, push to channel (drop oldest on full)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
		for scanner.Scan() {
			line := scanner.Text()
			if matcher != nil && !matcher(line) {
				continue
			}
			select {
			case lines <- line:
			default:
				dropped++
				select {
				case <-lines:
				default:
				}
				select {
				case lines <- line:
				default:
				}
			}
		}
	}()

	// pinger: WS ping + pong handler refreshes read deadline
	_ = ws.SetReadDeadline(time.Now().Add(pongDeadline))
	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(pongDeadline))
		return nil
	})
	pingStop := make(chan struct{})
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-pingStop:
				return
			case <-t.C:
				writeMu.Lock()
				_ = ws.SetWriteDeadline(time.Now().Add(writeTimeout))
				err := ws.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// reader pump: detect peer close
	go func() {
		defer close(done)
		for {
			if _, _, err := ws.NextReader(); err != nil {
				return
			}
		}
	}()

	defer close(pingStop)
	lastDropped := int64(0)
	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				_ = sess.Wait()
				_ = send(msg{Type: "done"})
				return
			}
			if err := send(msg{Type: "output", Data: line}); err != nil {
				_ = sess.Signal(gossh.SIGTERM)
				return
			}
		case <-flushTicker.C:
			if dropped > lastDropped {
				_ = send(msg{Type: "dropped", Count: dropped - lastDropped})
				lastDropped = dropped
			}
		case <-done:
			_ = sess.Signal(gossh.SIGTERM)
			return
		}
	}
}

func compileMatcher(o Opts) (func(string) bool, error) {
	if o.Include == "" && o.Exclude == "" {
		return nil, nil
	}
	build := func(pat string) (func(string) bool, error) {
		if pat == "" {
			return nil, nil
		}
		if o.Regex {
			expr := pat
			if !o.CaseSensitive {
				expr = "(?i)" + expr
			}
			re, err := regexp.Compile(expr)
			if err != nil {
				return nil, err
			}
			return re.MatchString, nil
		}
		needle := pat
		if !o.CaseSensitive {
			needle = strings.ToLower(needle)
			return func(s string) bool { return strings.Contains(strings.ToLower(s), needle) }, nil
		}
		return func(s string) bool { return strings.Contains(s, needle) }, nil
	}
	inc, err := build(o.Include)
	if err != nil {
		return nil, err
	}
	exc, err := build(o.Exclude)
	if err != nil {
		return nil, err
	}
	return func(line string) bool {
		if inc != nil && !inc(line) {
			return false
		}
		if exc != nil && exc(line) {
			return false
		}
		return true
	}, nil
}

// OptsFromQuery parses include/exclude/regex/case from a gin-style query getter.
func OptsFromQuery(q func(string) string) Opts {
	return Opts{
		Include:       q("include"),
		Exclude:       q("exclude"),
		Regex:         q("regex") == "1" || q("regex") == "true",
		CaseSensitive: q("case") == "1" || q("case") == "true",
	}
}
