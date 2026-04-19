//go:build desktop

package tray

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"fyne.io/systray"
	"github.com/serverhub/serverhub/pkg/notify"
)

//go:embed icon.png
var iconPNG []byte

// Run starts the HTTP server in a goroutine, then runs the system tray on the
// main OS thread (required on macOS/Darwin). Blocks until the user quits.
func Run(serve func(), port int) {
	go serve()
	systray.Run(func() { onReady(port) }, func() {})
}

func onReady(port int) {
	systray.SetIcon(iconPNG)
	systray.SetTitle("ServerHub")
	systray.SetTooltip(fmt.Sprintf("ServerHub — http://localhost:%d/panel/", port))

	mOpen := systray.AddMenuItem("打开面板", "在默认浏览器中打开")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem("关于 ServerHub", "SSH-native · Zero-agent · Desktop-first")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出 ServerHub", "停止服务并退出")

	notify.Send("ServerHub 已启动", fmt.Sprintf("访问 http://localhost:%d/panel/", port))

	for {
		select {
		case <-mOpen.ClickedCh:
			openBrowser(fmt.Sprintf("http://localhost:%d/panel/", port))
		case <-mAbout.ClickedCh:
			notify.Send("ServerHub", "SSH-native · Zero-agent · Desktop-first\ngithub.com/serverhub/serverhub")
		case <-mQuit.ClickedCh:
			systray.Quit()
			os.Exit(0)
		}
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
