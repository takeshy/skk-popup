package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/takeshy/skk-popup/internal/clipboard"
	"github.com/takeshy/skk-popup/internal/config"
	"github.com/takeshy/skk-popup/internal/desktop"
	"github.com/takeshy/skk-popup/internal/dict"
	"github.com/takeshy/skk-popup/internal/hotkey"
	"github.com/takeshy/skk-popup/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the Wails application: a permanently resident popup window that
// is shown/hidden on demand. Exported methods are exposed to the
// frontend as bindings.
type App struct {
	application *application.App
	window      *application.WebviewWindow
	cfg         *config.Config
	store       *dict.Store
	copier      *clipboard.Copier
	desktop     desktop.Platform
	hotkeyMgr   *hotkey.Manager

	mu             sync.Mutex
	visible        bool
	ready          bool
	pendingShow    bool
	pasteAfterHide bool
	ipcServer      *ipc.Server
}

func NewApp(wailsApp *application.App, cfg *config.Config) *App {
	return &App{application: wailsApp, cfg: cfg}
}

func (a *App) SetWindow(window *application.WebviewWindow) { a.window = window }

// ServiceStartup is called by Wails once all services are registered.
func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.copier = clipboard.New(a.cfg.Clipboard.Backend, a.application.Clipboard.SetText)
	a.desktop = desktop.New()

	if a.cfg.Hotkey.Enabled {
		mgr, err := hotkey.Start(a.cfg.Hotkey.Accelerator, a.TogglePopup)
		if err != nil {
			// Not fatal: the popup stays reachable via `skk-popup toggle`.
			log.Printf("hotkey: %v", err)
		} else {
			a.hotkeyMgr = mgr
		}
	}

	store, err := dict.NewStore()
	if err != nil {
		log.Printf("dict store: %v", err)
	} else {
		a.store = store
	}

	endpoint := ipc.PrepareEndpoint(ipc.Endpoint())
	server, err := ipc.NewServer(endpoint)
	if err != nil {
		// Another daemon won the race for the endpoint. Staying resident
		// without IPC would leave an orphaned process nothing could ever
		// show/hide/toggle/quit, so back out and let the winner run.
		log.Printf("ipc server: %v", err)
		a.application.Quit()
		return nil
	}
	a.ipcServer = server
	server.SetHandler(a.handleCommand)
	server.SetQuitCallback(func() {
		a.application.Quit()
	})
	go server.Serve()
	return nil
}

// ServiceShutdown flushes pending writes and closes the socket.
func (a *App) ServiceShutdown() error {
	if a.hotkeyMgr != nil {
		a.hotkeyMgr.Stop()
	}
	if a.ipcServer != nil {
		a.ipcServer.Close()
	}
	if a.store != nil {
		a.store.Flush()
	}
	return nil
}

func (a *App) handleCommand(command string) error {
	switch command {
	case "show":
		a.ShowPopup()
	case "hide":
		a.HidePopup()
	case "toggle":
		a.TogglePopup()
	}
	return nil
}

// TogglePopup flips window visibility.
func (a *App) TogglePopup() {
	if a.IsVisible() {
		a.HidePopup()
		return
	}
	a.ShowPopup()
}

// IsVisible reports whether the popup window is currently shown.
func (a *App) IsVisible() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.visible
}

// ShowPopup shows the window and tells the frontend to restore its session
// and focus the input. Requests received before NotifyReady are queued.
func (a *App) ShowPopup() {
	a.mu.Lock()
	if !a.ready {
		a.pendingShow = true
		a.mu.Unlock()
		return
	}
	a.visible = true
	a.mu.Unlock()

	// Capture who had focus before our window takes it (no-op on Linux,
	// where the compositor tracks this for us).
	if a.desktop != nil {
		a.desktop.RememberFocus()
	}
	a.window.Show()
	a.application.Event.Emit("popup:shown")
}

// HidePopup hides the window (the daemon keeps running), flushing any
// staged dictionary writes first. When auto-paste is enabled and text was
// just copied, focus is restored to the previous window and the configured
// paste shortcut (clipboard.paste_key) is sent after the configured delay.
func (a *App) HidePopup() {
	a.mu.Lock()
	wasVisible := a.visible
	a.visible = false
	doPaste := a.pasteAfterHide
	a.pasteAfterHide = false
	a.mu.Unlock()

	if !wasVisible && !doPaste {
		return
	}
	if a.store != nil {
		a.store.Flush()
	}
	a.window.Hide()
	if a.desktop == nil {
		return
	}
	if a.cfg.Window.RestoreFocus || doPaste {
		a.desktop.RestoreFocus()
	}
	if doPaste {
		time.Sleep(time.Duration(a.cfg.Clipboard.AutoPasteDelayMs) * time.Millisecond)
		if err := a.desktop.PasteKeys(a.cfg.Clipboard.PasteKey); err != nil {
			log.Printf("paste: %v", err)
		}
	}
}

// NotifyReady is called by the frontend once the dictionary and user data
// finished loading; a queued show request is served now.
func (a *App) NotifyReady() {
	a.mu.Lock()
	a.ready = true
	pending := a.pendingShow
	a.pendingShow = false
	a.mu.Unlock()

	if pending {
		a.ShowPopup()
	}
}

// LoadUserDict returns the persisted user dictionary JSON.
func (a *App) LoadUserDict() (string, error) {
	if a.store == nil {
		return "{}", nil
	}
	return a.store.LoadUserDict(), nil
}

// SaveUserDict persists the user dictionary JSON (debounced).
func (a *App) SaveUserDict(data string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SaveUserDict(data)
}

// LoadHistory returns the persisted candidate history JSON.
func (a *App) LoadHistory() string {
	if a.store == nil {
		return "{}"
	}
	return a.store.LoadHistory()
}

// SaveHistory persists the candidate history JSON (debounced).
func (a *App) SaveHistory(data string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SaveHistory(data)
}

// LoadInputHistory returns the persisted clipboard input history JSON.
func (a *App) LoadInputHistory() string {
	if a.store == nil {
		return "[]"
	}
	return a.store.LoadInputHistory()
}

// SaveInputHistory persists the clipboard input history JSON (debounced).
func (a *App) SaveInputHistory(data string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SaveInputHistory(data)
}

// ReadClipboard returns the current plain-text clipboard content.
func (a *App) ReadClipboard() string {
	if a.application == nil {
		return ""
	}
	text, ok := a.application.Clipboard.Text()
	if !ok {
		return ""
	}
	return text
}

// CopyToClipboard places the confirmed text on the Wayland clipboard. It
// arms the auto-paste step performed by HidePopup when enabled.
func (a *App) CopyToClipboard(text string) error {
	if text == "" {
		return nil
	}
	a.mu.Lock()
	a.pasteAfterHide = a.cfg.Clipboard.AutoPaste
	a.mu.Unlock()

	if a.copier == nil {
		return errClipboardUnavailable{}
	}
	return a.copier.Copy(text)
}

type errClipboardUnavailable struct{}

func (errClipboardUnavailable) Error() string { return "clipboard is not ready" }
