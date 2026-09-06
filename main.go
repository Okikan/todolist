package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "待办清单",
		Width:     960,
		Height:    720,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 245, B: 251, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.onBeforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "todolist-9f3c2b1a-4e8d-4a55-b2c1-5f6a7b8c9d01",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
