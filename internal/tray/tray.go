package tray

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/getlantern/systray"
)

// Callbacks allows the main app to wire up tray actions.
type Callbacks struct {
	OnReady                func()
	OnQuit                 func()
	OnPause                func()
	OnResume               func()
	SetInactivityThreshold func(seconds uint64)
	OnPrometheusToggle     func(enable bool)
	OnSetDBPath            func(path string)
	// GetMCPConfig returns the MCP JSON config string.
	GetMCPConfig           func() string
}

// Run starts the system tray icon. It blocks until quit.
func Run(dbpath string, cb Callbacks) {
	systray.Run(func() {
		systray.SetIcon(IconActive())
		systray.SetTitle("Timewarp")
		systray.SetTooltip("Timewarp — Tracking Time")

		mStatus := systray.AddMenuItem("Timewarp is running", "")
		mStatus.Disable()

		systray.AddSeparator()

		mPause := systray.AddMenuItem("Stop Tracking", "Pause focus tracking")
		var paused atomic.Bool

		systray.AddSeparator()

		mInactivity := systray.AddMenuItem("Inactivity Threshold", "")
		mInactivity.Disable()
		thresholds := []struct {
			label   string
			seconds uint64
		}{
			{"30 seconds", 30},
			{"1 minute", 60},
			{"2 minutes", 120},
			{"5 minutes", 300},
			{"10 minutes", 600},
		}
		var thresholdItems []*systray.MenuItem
		for _, t := range thresholds {
			item := systray.AddMenuItem("  "+t.label, fmt.Sprintf("Set inactivity threshold to %s", t.label))
			thresholdItems = append(thresholdItems, item)
		}
		thresholdItems[1].Check()

		systray.AddSeparator()

		mProm := systray.AddMenuItem("Enable Prometheus Endpoint", "Expose metrics on HTTP (off by default)")
		var promEnabled atomic.Bool

		systray.AddSeparator()

		mOpenDB := systray.AddMenuItem("Open DB Folder", "Open the database folder in Explorer")
		mSetDB := systray.AddMenuItem("Set DB Folder...", "Choose a folder for database files")
		systray.AddSeparator()
		mCopyMCP := systray.AddMenuItem("Copy MCP Config", "Copy MCP server config JSON to clipboard")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop Timewarp")

		go func() {
			for {
				select {
				case <-mPause.ClickedCh:
					if paused.Load() {
						paused.Store(false)
						mPause.SetTitle("Stop Tracking")
						mStatus.SetTitle("Timewarp is running")
						systray.SetIcon(IconActive())
						systray.SetTooltip("Timewarp — Tracking Time")
						if cb.OnResume != nil {
							cb.OnResume()
						}
					} else {
						paused.Store(true)
						mPause.SetTitle("Start Tracking")
						mStatus.SetTitle("Timewarp is paused")
						systray.SetIcon(IconPaused())
						systray.SetTooltip("Timewarp — Paused")
						if cb.OnPause != nil {
							cb.OnPause()
						}
					}

				case <-thresholdItems[0].ClickedCh:
					selectThreshold(thresholdItems, 0, thresholds[0].seconds, cb.SetInactivityThreshold)
				case <-thresholdItems[1].ClickedCh:
					selectThreshold(thresholdItems, 1, thresholds[1].seconds, cb.SetInactivityThreshold)
				case <-thresholdItems[2].ClickedCh:
					selectThreshold(thresholdItems, 2, thresholds[2].seconds, cb.SetInactivityThreshold)
				case <-thresholdItems[3].ClickedCh:
					selectThreshold(thresholdItems, 3, thresholds[3].seconds, cb.SetInactivityThreshold)
				case <-thresholdItems[4].ClickedCh:
					selectThreshold(thresholdItems, 4, thresholds[4].seconds, cb.SetInactivityThreshold)

				case <-mProm.ClickedCh:
					if promEnabled.Load() {
						promEnabled.Store(false)
						mProm.SetTitle("Enable Prometheus Endpoint")
						mProm.Uncheck()
						if cb.OnPrometheusToggle != nil {
							cb.OnPrometheusToggle(false)
						}
					} else {
						promEnabled.Store(true)
						mProm.SetTitle("Disable Prometheus Endpoint")
						mProm.Check()
						if cb.OnPrometheusToggle != nil {
							cb.OnPrometheusToggle(true)
						}
					}

				case <-mCopyMCP.ClickedCh:
					if cb.GetMCPConfig != nil {
						cfg := cb.GetMCPConfig()
						if copyToClipboard(cfg) {
							mCopyMCP.SetTitle("Copied!")
							go func() {
								time.Sleep(2 * time.Second)
								mCopyMCP.SetTitle("Copy MCP Config")
							}()
						}
					}
				case <-mOpenDB.ClickedCh:
					openFolder(dbpath)
				case <-mSetDB.ClickedCh:
					if picked := pickFolder(); picked != "" {
						dbpath = picked
						if cb.OnSetDBPath != nil {
							cb.OnSetDBPath(picked)
						}
					}
				case <-mQuit.ClickedCh:
					if cb.OnQuit != nil {
						cb.OnQuit()
					}
					systray.Quit()
					return
				}
			}
		}()

		if cb.OnReady != nil {
			cb.OnReady()
		}
	}, func() {})
}

func selectThreshold(items []*systray.MenuItem, selected int, seconds uint64, setter func(uint64)) {
	for i, item := range items {
		if i == selected {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
	if setter != nil {
		setter(seconds)
	}
}

func copyToClipboard(text string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	cmd := exec.Command("clip.exe")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

func pickFolder() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`$d = New-Object System.Windows.Forms.FolderBrowserDialog; `+
			`$d.Description = 'Select Timewarp DB folder'; `+
			`$d.ShowNewFolderButton = $true; `+
			`if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	result := strings.TrimSpace(string(out))
	return result
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
