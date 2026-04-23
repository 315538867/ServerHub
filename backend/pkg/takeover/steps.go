// Package takeover migrates a discovered service into ServerHub's standard
// directory layout (/opt/serverhub/apps/<name>/) so the platform owns its
// lifecycle going forward. Each kind (static / docker / compose / systemd)
// has its own runner; this file holds the shared infrastructure: a step
// engine that executes a forward action with a paired undo, automatically
// rolling back the already-completed steps if a later one fails.
package takeover

import (
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// AppsBase is where all takeover targets land. Hard-coded because the deploy
// runner already assumes apps live under a known prefix; making it configurable
// would mean threading a config through the rest of the deploy machinery for
// no real benefit.
const AppsBase = "/opt/serverhub/apps"

// BackupsBase is where pre-takeover copies of system config files live. We
// never delete from here — the user is the only one who decides when an old
// nginx vhost or systemd unit is safe to discard.
const BackupsBase = "/opt/serverhub/backups"

// Step is one unit of work in a takeover plan. Do performs a side-effect on
// the target host; Undo reverses it. RunSteps invokes Undo only for steps
// whose Do already returned nil — a step that fails its own Do is responsible
// for cleaning up whatever it half-did before returning the error.
type Step struct {
	Name string
	Do   func() error
	Undo func() error // may be nil for steps that need no rollback (probes, validations)
}

// Log is a tee-friendly accumulator. Every Step's progress lines flow through
// Write, and the same buffer is returned to the caller so the operator sees
// exactly what happened on the host.
type Log struct {
	b strings.Builder
}

func (l *Log) Write(p []byte) (int, error)    { return l.b.Write(p) }
func (l *Log) Printf(f string, a ...any)      { fmt.Fprintf(&l.b, f, a...) }
func (l *Log) Println(s string)               { l.b.WriteString(s); l.b.WriteByte('\n') }
func (l *Log) String() string                 { return l.b.String() }

// RunSteps executes the plan in order. On the first Do error, it walks the
// already-completed steps in reverse calling each non-nil Undo, then returns
// the original error annotated with which step failed. Undo errors are logged
// but do not mask the root cause.
func RunSteps(log *Log, steps []Step) error {
	completed := make([]Step, 0, len(steps))
	for _, s := range steps {
		log.Printf("\n▶ %s\n", s.Name)
		if err := s.Do(); err != nil {
			log.Printf("✗ %s: %v\n", s.Name, err)
			rollback(log, completed)
			return fmt.Errorf("%s: %w", s.Name, err)
		}
		log.Printf("✓ %s\n", s.Name)
		completed = append(completed, s)
	}
	return nil
}

func rollback(log *Log, done []Step) {
	if len(done) == 0 {
		return
	}
	log.Println("\n────── 回滚开始 ──────")
	for i := len(done) - 1; i >= 0; i-- {
		s := done[i]
		if s.Undo == nil {
			continue
		}
		log.Printf("↩ undo: %s\n", s.Name)
		if err := s.Undo(); err != nil {
			// Surface but don't propagate — we want every undo attempted.
			log.Printf("  ! undo 失败: %v\n", err)
		}
	}
	log.Println("────── 回滚结束 ──────")
}

// TargetDir returns the standard apps directory for a given deploy name.
// Caller must already have validated name with safeshell.ValidName.
func TargetDir(name string) string { return AppsBase + "/" + name }

// BackupDir returns the kind-scoped backup root, creating callers should mkdir
// before writing. e.g. BackupDir("nginx") = "/opt/serverhub/backups/nginx".
func BackupDir(kind string) string { return BackupsBase + "/" + kind }

// Timestamp produces the suffix used everywhere a takeover writes a dated
// artifact (release dir, backup file, renamed-original directory). Always
// in UTC so two operators in different timezones see consistent names.
func Timestamp() string { return time.Now().UTC().Format("20060102-150405") }

// MustRun runs cmd on the host and returns its combined output. On non-zero
// exit it returns an error whose message includes both the command and the
// captured output — far more useful for an operator than the bare exit code.
func MustRun(rn runner.Runner, log *Log, cmd string) (string, error) {
	log.Printf("$ %s\n", oneLine(cmd))
	out, err := rn.Run(cmd)
	if out != "" {
		log.Println(strings.TrimRight(out, "\n"))
	}
	if err != nil {
		return out, fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(out))
	}
	return out, nil
}

// oneLine collapses multi-line shell snippets so the log preview stays compact
// while the full output is still captured below.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// EnsureAbsent is a precheck helper: returns an error if path exists on the
// host. Used everywhere we refuse to clobber a pre-existing target.
func EnsureAbsent(rn runner.Runner, path string) error {
	if err := safeshell.AbsPath(path); err != nil {
		return fmt.Errorf("路径不合法 %q: %w", path, err)
	}
	out, _ := rn.Run("test -e " + safeshell.Quote(path) + " && echo exists || echo absent")
	if strings.Contains(out, "exists") {
		return fmt.Errorf("目标已存在: %s", path)
	}
	return nil
}

// EnsureReadable returns an error if path is not readable.
func EnsureReadable(rn runner.Runner, path string) error {
	out, _ := rn.Run("test -r " + safeshell.Quote(path) + " && echo ok || echo no")
	if !strings.Contains(out, "ok") {
		return fmt.Errorf("不可读: %s", path)
	}
	return nil
}

// ProbeHTTP issues a single curl against the host on the given Host header
// and returns nil iff the response status is 2xx or 3xx. Used as a takeover
// post-flight check to confirm the migrated service still answers requests.
//
// We hit 127.0.0.1 (resolved via --resolve) so the request stays on-box and
// works whether or not external DNS for hostName points at this server. port
// can be 80 or 443; for 443 we also pass -k since the local socket may not
// have a matching cert chain.
func ProbeHTTP(rn runner.Runner, log *Log, hostName string, port int) error {
	if hostName == "" || hostName == "_" {
		hostName = "localhost"
	}
	scheme := "http"
	extra := ""
	if port == 443 {
		scheme = "https"
		extra = " -k"
	}
	url := fmt.Sprintf("%s://%s:%d/", scheme, hostName, port)
	resolve := fmt.Sprintf("%s:%d:127.0.0.1", hostName, port)
	cmd := fmt.Sprintf(
		`curl -sS -o /dev/null -w '%%{http_code}' --max-time 5 --resolve %s%s %s`,
		safeshell.Quote(resolve), extra, safeshell.Quote(url))
	out, err := MustRun(rn, log, cmd)
	if err != nil {
		return fmt.Errorf("HTTP 探活失败: %w", err)
	}
	code := strings.TrimSpace(out)
	if len(code) != 3 || (code[0] != '2' && code[0] != '3') {
		return fmt.Errorf("HTTP 状态非 2xx/3xx: %s", code)
	}
	log.Printf("HTTP %s ✓\n", code)
	return nil
}
