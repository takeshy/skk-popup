package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/takeshy/skk-popup/internal/clipboard"
	"github.com/takeshy/skk-popup/internal/config"
	"github.com/takeshy/skk-popup/internal/dict"
	"github.com/takeshy/skk-popup/internal/hotkey"
)

// Bindings behind the ⋮ menu: version/help info and the Settings dialog
// (ported from omarchy-skk-popup's Settings/Help overlays).

// AppInfo is the read-only information shown in the menu and Settings.
type AppInfo struct {
	Version          string `json:"version"`
	OS               string `json:"os"`
	ConfigPath       string `json:"configPath"`
	DataDir          string `json:"dataDir"`
	DictionarySource string `json:"dictionarySource"`
}

// SaveConfigResult reports what SaveConfig applied immediately.
type SaveConfigResult struct {
	Path            string `json:"path"`
	RestartRequired bool   `json:"restartRequired"`
	Warning         string `json:"warning"`
}

// Minimum window size while the Settings/Help overlay is open (mirrors
// omarchy growing the card so the overlay content fits).
const (
	overlayMinWidth  = 600
	overlayMinHeight = 540
)

// GetAppInfo returns version and path information for the menu.
func (a *App) GetAppInfo() AppInfo {
	info := AppInfo{
		Version:          appVersion(),
		OS:               runtime.GOOS,
		ConfigPath:       config.Path(),
		DataDir:          dict.DataDir(),
		DictionarySource: "embedded",
	}
	if p := a.cfg.Dictionary.ExternalPath; p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			info.DictionarySource = p
		} else {
			info.DictionarySource = "embedded (external_path が見つかりません: " + p + ")"
		}
	}
	return info
}

// LoadConfig returns the configuration currently in effect.
func (a *App) LoadConfig() config.View {
	a.mu.Lock()
	defer a.mu.Unlock()
	return config.ToView(a.cfg)
}

// SaveConfig validates, writes config.toml, and applies what can change at
// runtime: window size, clipboard behaviour, and (on Windows) the hotkey.
// The dictionary path takes effect after a restart.
func (a *App) SaveConfig(view config.View) (SaveConfigResult, error) {
	next, err := config.FromView(view)
	if err != nil {
		return SaveConfigResult{}, err
	}
	path := config.Path()
	if err := config.Save(path, next); err != nil {
		return SaveConfigResult{}, fmt.Errorf("設定ファイルを書き込めません: %w", err)
	}

	a.actionMu.Lock()
	defer a.actionMu.Unlock()

	result := SaveConfigResult{Path: path}
	a.mu.Lock()
	prev := *a.cfg
	*a.cfg = *next
	overlay := a.overlayOpen
	a.mu.Unlock()

	if prev.Clipboard.Backend != next.Clipboard.Backend && a.application != nil {
		a.copier = clipboard.New(next.Clipboard.Backend, a.application.Clipboard.SetText)
	}
	if prev.Hotkey != next.Hotkey {
		if a.hotkeyMgr != nil {
			a.hotkeyMgr.Stop()
			a.hotkeyMgr = nil
		}
		if next.Hotkey.Enabled {
			mgr, err := hotkey.Start(next.Hotkey.Accelerator, a.ShowPopup)
			if err != nil {
				if errors.Is(err, hotkey.ErrUnsupported) {
					result.Warning = "アプリ内ホットキーは Windows 専用です。この OS では設定は保存されるだけで無視されます。"
				} else {
					result.Warning = "ホットキーを登録できません: " + err.Error()
				}
				log.Printf("hotkey: %v", err)
			} else {
				a.hotkeyMgr = mgr
			}
		}
	}
	if prev.Dictionary.ExternalPath != next.Dictionary.ExternalPath {
		result.RestartRequired = true
	}
	if a.window != nil && !overlay && (prev.Window.Width != next.Window.Width || prev.Window.Height != next.Window.Height) {
		a.window.SetSize(next.Window.Width, next.Window.Height)
	}
	return result, nil
}

// SetOverlayOpen grows the window while a Settings/Help overlay is shown
// and restores the configured size when it closes.
func (a *App) SetOverlayOpen(open bool) {
	a.actionMu.Lock()
	defer a.actionMu.Unlock()
	a.mu.Lock()
	a.overlayOpen = open
	width, height := a.cfg.Window.Width, a.cfg.Window.Height
	a.mu.Unlock()
	if a.window == nil {
		return
	}
	if open {
		a.window.SetSize(max(width, overlayMinWidth), max(height, overlayMinHeight))
	} else {
		a.window.SetSize(width, height)
	}
}
