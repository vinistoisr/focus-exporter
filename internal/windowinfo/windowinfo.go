package windowinfo

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// ActiveWindowInfo struct to hold information about the active window
type ActiveWindowInfo struct {
	Title       string
	ProcessID   uint32
	ProcessName string
	Hostname    string
	Username    string
}

// GetActiveWindowInfo retrieves information about the active window
func GetActiveWindowInfo() (ActiveWindowInfo, error) {
	hwnd := getForegroundWindow()
	if hwnd == 0 {
		return ActiveWindowInfo{}, fmt.Errorf("could not get foreground window")
	}

	title := getWindowText(hwnd)

	var processID uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &processID); err != nil {
		return ActiveWindowInfo{}, fmt.Errorf("could not get process ID: %w", err)
	}

	processName := getProcessName(processID)

	hostname, err := os.Hostname()
	if err != nil {
		return ActiveWindowInfo{}, fmt.Errorf("could not get hostname: %w", err)
	}

	username := os.Getenv("USERNAME")

	return ActiveWindowInfo{
		Title:       title,
		ProcessID:   processID,
		ProcessName: processName,
		Hostname:    hostname,
		Username:    username,
	}, nil
}
