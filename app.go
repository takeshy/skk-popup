package main

import (
	"context"
	"sync"
	"time"

	"github.com/takeshy/wl-skk-popup/internal/clipboard"
	"github.com/takeshy/wl-skk-popup/internal/config"
	"github.com/takeshy/wl-skk-popup/internal/desktop"
	"github.com/takeshy/wl-skk-popup/internal/dict"
	"github.com/takeshy/wl-skk-popup/internal/hotkey"
	"github.com/takeshy/wl-skk-popup/internal/ipc"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application: a permanently resident popup window that
// is shown/hidden on demand. Exported methods are exposed to the
// frontend as bindings.
type App struct {
	ctx       context.Context
	cfg       *config.Config
	store     *dict.Store
	copier    *clipboard.Copier
	desktop   desktop.Platform
	hotkeyMgr *hotkey.Manager

	mu             sync.Mutex
	visible        bool
	ready          bool
	pendingShow    bool
	pasteAfterHide bool
	ipcServer      *ipc.Server
}

func NewApp() *App {
	return &App{cfg: config.Load()}
}

// startup is called by Wails once the runtime context is available.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.copier = clipboard.New(a.cfg.Clipboard.Backend, ctx)
	a.desktop = desktop.New()

	if a.cfg.Hotkey.Enabled {
		mgr, err := hotkey.Start(a.cfg.Hotkey.Accelerator, a.TogglePopup)
		if err != nil {
			// Not fatal: the popup stays reachable via `wl-skk toggle`.
			wailsruntime.LogError(ctx, "hotkey: "+err.Error())
		} else {
			a.hotkeyMgr = mgr
		}
	}

	store, err := dict.NewStore()
	if err != nil {
		wailsruntime.LogError(ctx, "dict store: "+err.Error())
	} else {
		a.store = store
	}

	socketPath := ipc.RemoveStaleSocket(ipc.SocketPath())
	server, err := ipc.NewServer(socketPath)
	if err != nil {
		// Another daemon won the race for the socket. Staying resident
		// without IPC would leave an orphaned process nothing could ever
		// show/hide/toggle/quit, so back out and let the winner run.
		wailsruntime.LogError(ctx, "ipc server: "+err.Error())
		wailsruntime.Quit(ctx)
		return
	}
	a.ipcServer = server
	server.SetHandler(a.handleCommand)
	server.SetQuitCallback(func() {
		wailsruntime.Quit(ctx)
	})
	go server.Serve()
}

// shutdown flushes pending writes and closes the socket.
func (a *App) shutdown(ctx context.Context) {
	if a.hotkeyMgr != nil {
		a.hotkeyMgr.Stop()
	}
	if a.ipcServer != nil {
		a.ipcServer.Close()
	}
	if a.store != nil {
		a.store.Flush()
	}
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

// ShowPopup shows the window and tells the frontend to reset its buffer
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
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.EventsEmit(a.ctx, "popup:shown")
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
	wailsruntime.WindowHide(a.ctx)
	if a.desktop == nil {
		return
	}
	if a.cfg.Window.RestoreFocus || doPaste {
		a.desktop.RestoreFocus()
	}
	if doPaste {
		time.Sleep(time.Duration(a.cfg.Clipboard.AutoPasteDelayMs) * time.Millisecond)
		if err := a.desktop.PasteKeys(a.cfg.Clipboard.PasteKey); err != nil {
			wailsruntime.LogError(a.ctx, "paste: "+err.Error())
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
