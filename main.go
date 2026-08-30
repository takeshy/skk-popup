package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/takeshy/skk-popup/internal/assetserver"
	"github.com/takeshy/skk-popup/internal/config"
	"github.com/takeshy/skk-popup/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
		command := os.Args[1]
		if command == "-h" || command == "--help" || command == "help" {
			usage()
			return
		}
		if err := ipc.RunClientCommand(command); err != nil {
			fmt.Fprintln(os.Stderr, "skk-popup:", err)
			os.Exit(1)
		}
		return
	}
	runDaemon()
}

// runDaemon starts the resident process. A second daemon exits without
// touching the running one.
func runDaemon() {
	endpoint := ipc.Endpoint()
	if ipc.IsDaemonRunning(endpoint) {
		fmt.Fprintln(os.Stderr, "skk-popup: daemon is already running")
		os.Exit(1)
	}

	cfg := config.Load()
	assetHandler := assetserver.DictionaryMiddleware(cfg.Dictionary.ExternalPath)(
		application.AssetFileServerFS(assets),
	)
	wailsApp := application.New(application.Options{
		Name:        "skk-popup",
		Description: "SKK popup input window",
		Assets: application.AssetOptions{
			Handler: assetHandler,
		},
	})
	app := NewApp(wailsApp, cfg)
	wailsApp.RegisterService(application.NewService(app))

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "popup",
		Title:            "skk-popup",
		Width:            cfg.Window.Width,
		Height:           cfg.Window.Height,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
	})
	app.SetWindow(window)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Println(`skk-popup - SKK popup input window

Usage:
  skk-popup            start the daemon (resident mode)
  skk-popup toggle     show/hide the popup window
  skk-popup show       show or focus the popup window
  skk-popup hide       hide the popup window
  skk-popup quit       stop the daemon`)
}
