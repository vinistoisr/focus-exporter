package tray

import (
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

// Run starts the system tray icon. onReady is called once the tray is initialized
// (start your exporter loop in a goroutine from there). The tray blocks the calling
// goroutine until quit.
func Run(dbpath string, onReady func(), onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(generateIcon())
		systray.SetTitle("Timewarp")
		systray.SetTooltip("Timewarp — Focus Tracking")

		mStatus := systray.AddMenuItem("Timewarp is running", "")
		mStatus.Disable()

		systray.AddSeparator()

		mOpenDB := systray.AddMenuItem("Open DB Folder", "Open the database folder in Explorer")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop Timewarp")

		go func() {
			for {
				select {
				case <-mOpenDB.ClickedCh:
					openFolder(dbpath)
				case <-mQuit.ClickedCh:
					if onQuit != nil {
						onQuit()
					}
					systray.Quit()
					return
				}
			}
		}()

		if onReady != nil {
			onReady()
		}
	}, func() {})
}

func openFolder(path string) {
	if path == "" || runtime.GOOS != "windows" {
		return
	}
	cmd := exec.Command("explorer.exe", path)
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}
