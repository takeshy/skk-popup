package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/takeshy/skk-popup/internal/assetserver"
	"github.com/takeshy/skk-popup/internal/ipc"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsassetserver "github.com/wailsapp/wails/v2/pkg/options/assetserver"
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
	if !bindingsMode {
		// Wails' `-tags bindings` sub-build runs this binary just to have
		// wails.Run() dump the bound methods and exit; it never reaches
		// OnStartup, so the multi-instance guard would otherwise reject it
		// whenever a real daemon is already running and break bindings
		// generation without actually regenerating anything.
		socketPath := ipc.SocketPath()
		if ipc.IsDaemonRunning(socketPath) {
			fmt.Fprintln(os.Stderr, "skk-popup: daemon is already running")
			os.Exit(1)
		}
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "skk-popup",
		Width:             app.cfg.Window.Width,
		Height:            app.cfg.Window.Height,
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       true,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &wailsassetserver.Options{
			Assets:     assets,
			Middleware: assetserver.DictionaryMiddleware(app.cfg.Dictionary.ExternalPath),
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "skk-popup:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`skk-popup - SKK popup input window

Usage:
  skk-popup            start the daemon (resident mode)
  skk-popup toggle     show/hide the popup window
  skk-popup show       show the popup window
  skk-popup hide       hide the popup window
  skk-popup quit       stop the daemon`)
}
