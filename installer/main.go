package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var assets embed.FS

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = "0.1.0"

func main() {
	mode := "install"
	for _, a := range os.Args[1:] {
		switch a {
		case "--uninstall":
			mode = "uninstall"
		case "--modify":
			mode = "modify"
		}
	}
	app := NewApp(mode)

	err := wails.Run(&options.App{
		Title:     "PC Optimizer Setup",
		Width:     880,
		Height:    620,
		MinWidth:  880,
		MinHeight: 620,
		MaxWidth:  880,
		MaxHeight: 620,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 248, B: 241, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Windows: &windows.Options{
			WebviewUserDataPath: os.TempDir() + `\pcoptimizer-setup`,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
