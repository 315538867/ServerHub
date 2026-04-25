// Package sse 提供极简 Server-Sent Events 写器，面向"一个 HTTP 请求里推多条事件
// 后关闭"的场景（如 AppReleaseSet Apply 流式进度）。
//
// 不做事件 ID / retry / 断点续传——断线的客户端应走 GET 接口查终态快照。
package sse

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Writer 封装 gin 上下文做 SSE 推送。非线程安全——调用方需自行串行化。
type Writer struct {
	c       *gin.Context
	flusher http.Flusher
}

// New 初始化 SSE 响应头并返回 Writer。若响应不可 Flush（例如反代吃掉 Transfer-Encoding）
// 返回 nil，调用方应退化为非流式返回。
func New(c *gin.Context) *Writer {
	f, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil
	}
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// 要求反代（nginx）立即吐字节，不要缓冲 SSE。与 docs/deployment.md 的
	// proxy_buffering off 配合。
	h.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	f.Flush()
	return &Writer{c: c, flusher: f}
}

// Event 发送一条命名事件。data 以 JSON 序列化；失败不阻断，静默丢弃。
func (w *Writer) Event(name string, data any) error {
	if w == nil {
		return nil
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w.c.Writer, "event: %s\ndata: %s\n\n", name, payload); err != nil {
		return err
	}
	w.flusher.Flush()
	return nil
}

// Done 写一个 done 终止事件，便于客户端明确结束。
func (w *Writer) Done() {
	_ = w.Event("done", map[string]any{})
}

// Closed 表示客户端是否已断开；调用方可据此放弃后续推送（但后端任务应继续）。
func (w *Writer) Closed() bool {
	if w == nil {
		return true
	}
	select {
	case <-w.c.Request.Context().Done():
		return true
	default:
		return false
	}
}
