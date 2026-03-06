package tray

import (
	"encoding/json"
	"fmt"
	"os"
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

// hiddenCmd returns a Cmd with the console window hidden.
func hiddenCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

// HasScheduledTask checks if the "Timewarp" scheduled task exists.
func HasScheduledTask() bool {
	cmd := hiddenCmd("schtasks", "/query", "/tn", "Timewarp")
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

	cmd := hiddenCmd("powershell", "-NoProfile", "-Command",
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

// OfferScheduledTask asks the user if they want Timewarp to start automatically.
// If yes, it creates the scheduled task via an elevated process and returns true.
func OfferScheduledTask(exePath, dbpath string) bool {
	ret := messageBox("Start Automatically?",
		"Would you like Timewarp to start automatically when you log in?\r\n\r\n"+
			"This will ask for administrator permission to create a scheduled task.\r\n\r\n"+
			"You can remove this later from the command line with:\r\n"+
			"  timewarp.exe -uninstall",
		mbYesNo|mbIconQuestion)
	if ret != idYes {
		return false
	}

	// Launch an elevated process with just -install (not the whole app)
	args := fmt.Sprintf(`-install -dbpath "%s"`, dbpath)
	if err := elevatedRun(exePath, args); err != nil {
		messageBox("Timewarp", fmt.Sprintf("Failed to create scheduled task:\n%v", err), mbOK|mbIconInfo)
		return false
	}
	return true
}

// elevatedRun launches the given exe with args via UAC "runas".
func elevatedRun(exe, args string) error {
	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	shell32 := syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW := shell32.NewProc("ShellExecuteW")

	ret, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verbPtr)), uintptr(unsafe.Pointer(exePtr)), uintptr(unsafe.Pointer(argsPtr)), 0, syscall.SW_HIDE)
	if ret <= 32 {
		return fmt.Errorf("ShellExecute returned %d", ret)
	}
	return nil
}

// claudeConfigPath returns the path to Claude Desktop's config file, or empty if not found.
func claudeConfigPath() string {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return ""
	}
	p := filepath.Join(appdata, "Claude", "claude_desktop_config.json")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// IsClaudeConnected checks if the Claude Desktop config already has a timewarp MCP entry.
func IsClaudeConnected() bool {
	p := claudeConfigPath()
	if p == "" {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	servers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return false
	}
	_, exists := servers["timewarp"]
	return exists
}

// ConnectClaude adds the timewarp MCP server entry to Claude Desktop's config.
// Returns a user-facing message describing the result.
func ConnectClaude(exePath, dbpath string) string {
	p := claudeConfigPath()
	if p == "" {
		return "Could not find Claude Desktop config.\n\nExpected location:\n%APPDATA%\\Claude\\claude_desktop_config.json\n\nMake sure Claude Desktop is installed."
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Sprintf("Could not read config file: %v", err)
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Sprintf("Could not parse config file: %v\n\nYou may need to fix the JSON manually.", err)
	}

	// Parse or create mcpServers
	servers := map[string]json.RawMessage{}
	if raw, ok := config["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return fmt.Sprintf("Could not parse mcpServers: %v\n\nYou may need to fix the JSON manually.", err)
		}
	}

	// Check if already connected
	if _, exists := servers["timewarp"]; exists {
		return "Timewarp is already connected to Claude Desktop."
	}

	// Build the timewarp entry
	entry := map[string]interface{}{
		"command": exePath,
		"args":    []string{"-mcp", "-dbpath", dbpath},
	}
	entryJSON, _ := json.Marshal(entry)
	servers["timewarp"] = json.RawMessage(entryJSON)

	serversJSON, _ := json.Marshal(servers)
	config["mcpServers"] = json.RawMessage(serversJSON)

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Sprintf("Could not serialize config: %v", err)
	}

	if err := os.WriteFile(p, out, 0644); err != nil {
		return fmt.Sprintf("Could not write config file: %v", err)
	}

	return "Connected! Restart Claude Desktop to activate the Timewarp MCP server."
}
