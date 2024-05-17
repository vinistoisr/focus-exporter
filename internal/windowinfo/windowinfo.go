package windowinfo

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	openProcess = kernel32.NewProc("OpenProcess")
	closeHandle = kernel32.NewProc("CloseHandle")
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

func getForegroundWindow() windows.HWND {
	getForegroundWindow := user32.NewProc("GetForegroundWindow")
	ret, _, _ := getForegroundWindow.Call()
	return windows.HWND(ret)
}

func getWindowText(hwnd windows.HWND) string {
	getWindowTextW := user32.NewProc("GetWindowTextW")

	const maxChars = 256
	text := make([]uint16, maxChars)
	ret, _, _ := getWindowTextW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&text[0])),
		uintptr(maxChars),
	)
	if ret == 0 {
		return "" // Empty string if no title
	}
	return windows.UTF16ToString(text)
}

func getProcessName(processID uint32) string {
	// PROCESS_QUERY_INFORMATION | PROCESS_VM_READ
	desiredAccess := uint32(0x0400 | 0x0010)
	handle, _, _ := openProcess.Call(uintptr(desiredAccess), 0, uintptr(processID))
	if handle == 0 {
		return ""
	}
	defer closeHandle.Call(handle)

	psapi := syscall.NewLazyDLL("psapi.dll")
	getModuleBaseNameW := psapi.NewProc("GetModuleBaseNameW")

	const maxPath = 260
	var processName [maxPath]uint16
	ret, _, _ := getModuleBaseNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&processName[0])),
		uintptr(maxPath),
	)
	if ret == 0 {
		return ""
	}
	return windows.UTF16ToString(processName[:])
}
