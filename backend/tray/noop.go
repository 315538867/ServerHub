//go:build !desktop

package tray

// Run calls serve() directly and blocks — server mode has no tray.
func Run(serve func(), _ int) {
	serve()
}
