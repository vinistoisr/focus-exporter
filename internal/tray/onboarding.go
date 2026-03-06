package tray

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOK             = 0x00000000
	mbYesNo          = 0x00000004
	mbIconQuestion   = 0x00000020
	mbIconInfo       = 0x00000040
	idYes            = 6
)

func messageBox(title, text string, flags uintptr) int {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	ret, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
	return int(ret)
}

// HasExistingDB checks if any timewarp-*.db files exist in the given directory.
func HasExistingDB(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "timewarp-*.db"))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// HasScheduledTask checks if the "Timewarp" scheduled task exists.
func HasScheduledTask() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", "Timewarp")
	err := cmd.Run()
	return err == nil
}

// RunOnboarding shows a welcome dialog and folder picker if this is a first run.
// Returns the chosen DB path, or empty string if the user cancelled.
func RunOnboarding() string {
	messageBox("Welcome to Timewarp",
		"Timewarp tracks your active window to help you fill out timesheets.\r\n\r\n"+
			"First, choose a folder to store your data.\r\n\r\n"+
			"Tip: Pick a folder inside OneDrive if you use multiple computers — "+
			"each PC saves its own file, so there are no sync conflicts and "+
			"you can see all your activity in one place.\r\n\r\n"+
			"Click OK to choose a folder.",
		mbOK|mbIconInfo)

	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`$d = New-Object System.Windows.Forms.FolderBrowserDialog; `+
			`$d.Description = 'Select or create a folder for Timewarp data'; `+
			`$d.ShowNewFolderButton = $true; `+
			`if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// OfferScheduledTask asks the user if they want Timewarp to start automatically,
// and returns true if they said yes.
func OfferScheduledTask() bool {
	ret := messageBox("Start Automatically?",
		"Would you like Timewarp to start automatically when you log in?\r\n\r\n"+
			"This will ask for administrator permission to create a scheduled task.\r\n\r\n"+
			"You can remove this later from the command line with:\r\n"+
			"  timewarp.exe -uninstall",
		mbYesNo|mbIconQuestion)
	return ret == idYes
}
