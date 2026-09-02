package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/takeshy/skk-popup/internal/hotkey"
)

// View is the JSON shape exchanged with the Settings dialog.

// View mirrors Config with JSON names for the frontend.
type View struct {
	Window     WindowView     `json:"window"`
	Clipboard  ClipboardView  `json:"clipboard"`
	Hotkey     HotkeyView     `json:"hotkey"`
	Dictionary DictionaryView `json:"dictionary"`
}

type WindowView struct {
	Width        int  `json:"width"`
	Height       int  `json:"height"`
	RestoreFocus bool `json:"restoreFocus"`
}

type ClipboardView struct {
	Backend          string `json:"backend"`
	AutoPaste        bool   `json:"autoPaste"`
	AutoPasteDelayMs int    `json:"autoPasteDelayMs"`
	PasteKey         string `json:"pasteKey"`
}

type HotkeyView struct {
	Enabled     bool   `json:"enabled"`
	Accelerator string `json:"accelerator"`
}

type DictionaryView struct {
	ExternalPath string `json:"externalPath"`
}

// ToView converts the effective configuration for the frontend.
func ToView(c *Config) View {
	var v View
	v.Window.Width = c.Window.Width
	v.Window.Height = c.Window.Height
	v.Window.RestoreFocus = c.Window.RestoreFocus
	v.Clipboard.Backend = c.Clipboard.Backend
	v.Clipboard.AutoPaste = c.Clipboard.AutoPaste
	v.Clipboard.AutoPasteDelayMs = c.Clipboard.AutoPasteDelayMs
	v.Clipboard.PasteKey = c.Clipboard.PasteKey
	v.Hotkey.Enabled = c.Hotkey.Enabled
	v.Hotkey.Accelerator = c.Hotkey.Accelerator
	v.Dictionary.ExternalPath = c.Dictionary.ExternalPath
	return v
}

// FromView validates a submitted view and returns the config it
// describes. Values outside the accepted set are rejected rather than
// silently replaced so the user sees what was wrong.
func FromView(v View) (*Config, error) {
	c := Default()
	if v.Window.Width < 200 || v.Window.Width > 4000 || v.Window.Height < 120 || v.Window.Height > 4000 {
		return nil, errors.New("ウィンドウサイズは 幅 200〜4000 / 高さ 120〜4000 の範囲で指定してください")
	}
	c.Window.Width = v.Window.Width
	c.Window.Height = v.Window.Height
	c.Window.RestoreFocus = v.Window.RestoreFocus

	switch v.Clipboard.Backend {
	case "wl-copy", "wails":
		c.Clipboard.Backend = v.Clipboard.Backend
	default:
		return nil, fmt.Errorf("clipboard backend %q は wl-copy または wails を指定してください", v.Clipboard.Backend)
	}
	c.Clipboard.AutoPaste = v.Clipboard.AutoPaste
	if v.Clipboard.AutoPasteDelayMs < 0 || v.Clipboard.AutoPasteDelayMs > 10000 {
		return nil, errors.New("auto_paste_delay_ms は 0〜10000 の範囲で指定してください")
	}
	c.Clipboard.AutoPasteDelayMs = v.Clipboard.AutoPasteDelayMs
	switch v.Clipboard.PasteKey {
	case "ctrl+v", "ctrl+shift+v":
		c.Clipboard.PasteKey = v.Clipboard.PasteKey
	default:
		return nil, fmt.Errorf("paste_key %q は ctrl+v または ctrl+shift+v を指定してください", v.Clipboard.PasteKey)
	}

	c.Hotkey.Enabled = v.Hotkey.Enabled
	accelerator := strings.Join(strings.FieldsFunc(strings.TrimSpace(v.Hotkey.Accelerator), func(r rune) bool {
		return r == '+' || r == ' '
	}), "+")
	if accelerator == "" {
		return nil, errors.New("ホットキーを入力してください (例: Ctrl+Shift+K)")
	}
	if _, _, err := hotkey.ParseAccelerator(accelerator); err != nil {
		return nil, fmt.Errorf("ホットキー %q を解釈できません: A-Z, 0-9, F1-F24 と Ctrl/Shift/Alt/Win を + で繋いでください", v.Hotkey.Accelerator)
	}
	c.Hotkey.Accelerator = accelerator

	path := strings.TrimSpace(v.Dictionary.ExternalPath)
	if path != "" {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("外部辞書 %q が見つかりません", path)
		}
	}
	c.Dictionary.ExternalPath = path
	return c, nil
}
