package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend
var frontend embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	application := NewApp()
	if err := wails.Run(&options.App{
		Title:         "Password Admin",
		Width:         1080,
		Height:        720,
		MinWidth:      760,
		MinHeight:     560,
		AssetServer:   &assetserver.Options{Assets: assets},
		Linux:         &linux.Options{Icon: appIcon},
		OnStartup:     application.startup,
		OnBeforeClose: application.beforeClose,
		OnShutdown:    application.shutdown,
		Bind:          []interface{}{application},
	}); err != nil {
		log.Fatal(err)
	}
}
