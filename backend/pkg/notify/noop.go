//go:build !desktop

package notify

// Send is a no-op in server mode.
func Send(_, _ string) {}
