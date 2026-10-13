//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32Mod              = syscall.NewLazyDLL("user32.dll")
	procFindWindowW        = user32Mod.NewProc("FindWindowW")
	procShowWindow         = user32Mod.NewProc("ShowWindow")
	procGetWindowPlacement = user32Mod.NewProc("GetWindowPlacement")
	procGetSystemMetrics   = user32Mod.NewProc("GetSystemMetrics")
)

const (
	swMaximize      = 3
	swShowMaximized = 3
	smCxScreen      = 0
	smCyScreen      = 1
)

type winPoint struct{ X, Y int32 }
type winRect struct{ Left, Top, Right, Bottom int32 }

type windowPlacement struct {
	Length, Flags, ShowCmd       uint32
	PtMinPosition, PtMaxPosition winPoint
	RcNormalPosition             winRect
}

func findWindow(title string) (syscall.Handle, bool) {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0, false
	}
	r, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
	if r == 0 {
		return 0, false
	}
	return syscall.Handle(r), true
}

func isMaximized(title string) bool {
	h, ok := findWindow(title)
	if !ok {
		return false
	}
	var wp windowPlacement
	wp.Length = uint32(unsafe.Sizeof(wp))
	r, _, _ := procGetWindowPlacement.Call(uintptr(h), uintptr(unsafe.Pointer(&wp)))
	if r == 0 {
		return false
	}
	return wp.ShowCmd == swShowMaximized
}

// maximizeWhenReady maximizes the window once it exists (the Fyne window is
// created asynchronously during Show). Runs in its own goroutine; ShowWindow
// is thread-safe.
func maximizeWhenReady(title string) {
	for i := 0; i < 40; i++ {
		if h, ok := findWindow(title); ok {
			procShowWindow.Call(uintptr(h), uintptr(swMaximize))
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// screenSize returns the primary monitor size in physical pixels.
func screenSize() (w, h int32) {
	rw, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
	rh, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
	return int32(rw), int32(rh)
}
