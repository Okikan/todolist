package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"fyne.io/systray"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// startTray 启动系统托盘（独立 goroutine，随进程存活）
func startTray(app *App) {
	go systray.Run(func() { app.onTrayReady() }, func() {})
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("待办清单")

	mToggle := systray.AddMenuItem("显示 / 隐藏窗口", "显示或隐藏主窗口")
	mNew := systray.AddMenuItem("新建任务", "打开主窗口并新建任务")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出待办清单")

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				a.toggleWindow()
			case <-mNew.ClickedCh:
				a.showWindow()
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "open-new-task")
				}
			case <-mQuit.ClickedCh:
				a.quitApp()
				return
			}
		}
	}()
}

// quitApp 清理托盘图标后退出整个应用
func (a *App) quitApp() {
	a.quitting = true
	systray.Quit()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}
