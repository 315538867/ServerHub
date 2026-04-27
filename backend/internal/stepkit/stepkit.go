// Package stepkit 是 source adapter 的共享 step 引擎,从 v1 pkg/takeover/steps.go
// 平移而来,签名改为 ctx + infra.Runner 以匹配 v2 端口契约。
//
// adapter takeover 通过 RunSteps 执行有 Undo 的 step 链,任一 Do 失败即逆序回滚已完成
// step 的 Undo。Log 是 tee 累积器,所有 helper 命令的 stdout/stderr 都流入同一 buffer。
package stepkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/infra"
	"github.com/serverhub/serverhub/pkg/safeshell"
)

// AppsBase 是所有接管目标的根目录(与 v1 pkg/takeover 保持一致)。
const AppsBase = "/opt/serverhub/apps"

// BackupsBase 是接管前系统配置文件的备份根目录。永不自动删除,运维确认。
const BackupsBase = "/opt/serverhub/backups"

// Step 是一个原子 takeover 单元。Do 执行远端副作用; Undo 反做。
// RunSteps 仅对 Do 已成功返回 nil 的 step 调用 Undo。
type Step struct {
	Name string
	Do   func() error
	Undo func() error // 可为 nil(纯校验/探活步骤)
}

// Log 是 step 链的进度累积 buffer,直接返回给 UI 展示。
type Log struct{ b strings.Builder }

func (l *Log) Write(p []byte) (int, error) { return l.b.Write(p) }
func (l *Log) Printf(f string, a ...any)   { fmt.Fprintf(&l.b, f, a...) }
func (l *Log) Println(s string)            { l.b.WriteString(s); l.b.WriteByte('\n') }
func (l *Log) String() string              { return l.b.String() }

// RunSteps 顺序执行 plan;首次 Do 失败立即逆序调 Undo,返回错误带 step 名上下文。
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
			log.Printf("  ! undo 失败: %v\n", err)
		}
	}
	log.Println("────── 回滚结束 ──────")
}

// TargetDir 是 deploy 标准目录。caller 必须先用 safeshell.ValidName 校验 name。
func TargetDir(name string) string { return AppsBase + "/" + name }

// BackupDir 返回 kind-scoped 备份根。caller 应在写之前 mkdir。
func BackupDir(kind string) string { return BackupsBase + "/" + kind }

// Timestamp 是 takeover 一次运行内所有 dated artifact 的统一后缀,UTC 保证多操作员一致。
func Timestamp() string { return time.Now().UTC().Format("20060102-150405") }

// MustRun 在远端跑 cmd 并把日志写 log。stderr 拼到 stdout 后(对齐 v1 pkg/runner
// 的"combined output"语义),非 0 退出返回带输出的 error。
func MustRun(ctx context.Context, r infra.Runner, log *Log, cmd string) (string, error) {
	log.Printf("$ %s\n", oneLine(cmd))
	stdout, stderr, err := r.Run(ctx, cmd)
	out := stdout
	if stderr != "" {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += stderr
	}
	if out != "" {
		log.Println(strings.TrimRight(out, "\n"))
	}
	if err != nil {
		return out, fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(out))
	}
	return out, nil
}

// RunQuiet 不写 log,仅返回 stdout(stderr 丢弃)。给"探测/查询"类调用用,避免 log
// 被 noise 淹没。失败返回空串 + nil(语义同 v1 rn.Run("test -f X") 模式)。
func RunQuiet(ctx context.Context, r infra.Runner, cmd string) (string, error) {
	stdout, _, err := r.Run(ctx, cmd)
	return stdout, err
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// EnsureAbsent 校验 path 不存在;存在返回 error。
func EnsureAbsent(ctx context.Context, r infra.Runner, path string) error {
	if err := safeshell.AbsPath(path); err != nil {
		return fmt.Errorf("路径不合法 %q: %w", path, err)
	}
	out, _ := RunQuiet(ctx, r, "test -e "+safeshell.Quote(path)+" && echo exists || echo absent")
	if strings.Contains(out, "exists") {
		return fmt.Errorf("目标已存在: %s", path)
	}
	return nil
}

// EnsureReadable 校验 path 可读。
func EnsureReadable(ctx context.Context, r infra.Runner, path string) error {
	out, _ := RunQuiet(ctx, r, "test -r "+safeshell.Quote(path)+" && echo ok || echo no")
	if !strings.Contains(out, "ok") {
		return fmt.Errorf("不可读: %s", path)
	}
	return nil
}

// ProbeHTTP 通过 curl --resolve 在远端本机回环探活,2xx/3xx 视为通过。
func ProbeHTTP(ctx context.Context, r infra.Runner, log *Log, hostName string, port int) error {
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
	out, err := MustRun(ctx, r, log, cmd)
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

// WriteRemoteFile 包装 safeshell.WriteRemoteFile + MustRun,step 闭包内常用。
func WriteRemoteFile(ctx context.Context, r infra.Runner, log *Log, path, content string) error {
	_, err := MustRun(ctx, r, log, safeshell.WriteRemoteFile(path, content, true))
	return err
}
