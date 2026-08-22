package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/takeshy/wl-skk-popup/internal/assetserver"
	"github.com/takeshy/wl-skk-popup/internal/ipc"
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
			fmt.Fprintln(os.Stderr, "wl-skk:", err)
			os.Exit(1)
		}
		return
	}
	runDaemon()
}

// runDaemon starts the resident process. A second daemon exits without
// touching the running one.
func runDaemon() {
	socketPath := ipc.SocketPath()
	if ipc.IsDaemonRunning(socketPath) {
		fmt.Fprintln(os.Stderr, "wl-skk: daemon is already running")
		os.Exit(1)
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "wl-skk",
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
		fmt.Fprintln(os.Stderr, "wl-skk:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`wl-skk - SKK popup input window for Wayland

Usage:
  wl-skk            start the daemon (resident mode)
  wl-skk toggle     show/hide the popup window
  wl-skk show       show the popup window
  wl-skk hide       hide the popup window
  wl-skk quit       stop the daemon`)
}
