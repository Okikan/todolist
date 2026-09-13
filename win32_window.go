//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// 直接走 Win32 API 管理窗口显隐：ShowWindowAsync 是异步投递、不依赖
// Wails 主线程 RPC，长时间隐藏到托盘后也不会被节流卡死。
var (
	user32Win               = syscall.NewLazyDLL("user32.dll")
	procFindWindowW         = user32Win.NewProc("FindWindowW")
	procIsWindowVisible     = user32Win.NewProc("IsWindowVisible")
	procIsIconic            = user32Win.NewProc("IsIconic")
	procShowWindowAsync     = user32Win.NewProc("ShowWindowAsync")
	procSetForegroundWindow = user32Win.NewProc("SetForegroundWindow")
)

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9
)

// findMainWindow 通过窗口标题查找主窗口句柄（找不到返回 0）
func findMainWindow() uintptr {
	title, _ := syscall.UTF16PtrFromString("待办清单")
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return hwnd
}

func winVisible(hwnd uintptr) bool {
	v, _, _ := procIsWindowVisible.Call(hwnd)
	return v != 0
}

func winMinimised(hwnd uintptr) bool {
	v, _, _ := procIsIconic.Call(hwnd)
	return v != 0
}

func winShowAsync(hwnd uintptr, cmd uintptr) {
	procShowWindowAsync.Call(hwnd, cmd)
}

func winForeground(hwnd uintptr) {
	procSetForegroundWindow.Call(hwnd)
}
