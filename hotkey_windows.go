//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	hotkeyID   = 1
	wmHotkey   = 0x0312
	modAlt     = 0x0001
	modControl = 0x0002
	vkSpace    = 0x20
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey = user32.NewProc("RegisterHotKey")
	procGetMessage     = user32.NewProc("GetMessageW")
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// registerGlobalHotkey 注册全局快捷键 Ctrl+Alt+Space：随时呼出/隐藏主窗口。
// 注册失败（快捷键被其他程序占用）时静默放弃，不影响应用其余功能。
func registerGlobalHotkey(app *App) {
	go func() {
		runtime.LockOSThread() // RegisterHotKey 与 GetMessage 必须在同一线程
		ret, _, _ := procRegisterHotKey.Call(0, hotkeyID, modControl|modAlt, vkSpace)
		if ret == 0 {
			return
		}
		var m winMsg
		for {
			ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if ret == 0 || ret == ^uintptr(0) { // WM_QUIT 或消息错误
				return
			}
			if m.message == wmHotkey && m.wParam == hotkeyID {
				app.toggleWindow()
			}
		}
	}()
}
