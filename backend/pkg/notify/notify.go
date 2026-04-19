//go:build desktop

package notify

import "github.com/gen2brain/beeep"

// Send shows a native OS desktop notification.
func Send(title, body string) {
	_ = beeep.Notify(title, body, "")
}
