package inactivity

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getLastInputInfo = user32.NewProc("GetLastInputInfo")
	getTickCount64   = kernel32.NewProc("GetTickCount64")
)

type LASTINPUTINFO struct {
	cbSize uint32
	dwTime uint32
}

func GetInactivityTime(inactivityThreshold uint64, inactivityCounter time.Duration) time.Duration {
	lastInputTime := getLastInputTime()
	currentTime := getTickCount()
	elapsed := currentTime - lastInputTime

	fmt.Println("Elapsed Time (ms):", elapsed)

	if elapsed > inactivityThreshold {
		inactivityCounter += time.Second // Increment if inactive
	}

	return inactivityCounter
}

// getLastInputTime retrieves the last input time in milliseconds
func getLastInputTime() uint64 {
	var lastInputInfo LASTINPUTINFO
	lastInputInfo.cbSize = uint32(unsafe.Sizeof(lastInputInfo))

	ret, _, _ := getLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInputInfo)))
	if ret == 0 { // Function call failed
		fmt.Println("GetLastInputInfo failed")
		return 0
	}

	fmt.Println("Raw dwTime:", lastInputInfo.dwTime)
	fmt.Println("Converted time (ms):", uint64(lastInputInfo.dwTime))

	return uint64(lastInputInfo.dwTime) // Return time in milliseconds
}

func getTickCount() uint64 {
	ret, _, _ := getTickCount64.Call()
	return uint64(ret)
}
