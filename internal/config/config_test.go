package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaultsWhenMissing(t *testing.T) {
	cfg := LoadFrom(filepath.Join(t.TempDir(), "absent.toml"))
	if cfg.Window.Width != 600 || cfg.Window.Height != 240 {
		t.Fatalf("unexpected default window size: %+v", cfg.Window)
	}
	if !cfg.Window.RestoreFocus {
		t.Fatal("restore_focus should default to true")
	}
	if cfg.Clipboard.Backend != "wl-copy" || !cfg.Clipboard.AutoPaste || cfg.Clipboard.PasteKey != "ctrl+shift+v" {
		t.Fatalf("unexpected clipboard defaults: %+v", cfg.Clipboard)
	}
}

func TestLoadOverrides(t *testing.T) {
	content := `
# comment
[window]
width = 480
height = 160
restore_focus = false

[clipboard]
backend = "wails"
auto_paste = true
auto_paste_delay_ms = 120
paste_key = "ctrl+v"

[hotkey]
enabled = false
accelerator = "Ctrl+Alt+J"

[dictionary]
external_path = "/tmp/SKK-JISYO.L.json"
`
	cfg := LoadFrom(write(t, content))

	if cfg.Window.Width != 480 || cfg.Window.Height != 160 || cfg.Window.RestoreFocus {
		t.Fatalf("window config not applied: %+v", cfg.Window)
	}
	if cfg.Clipboard.Backend != "wails" || !cfg.Clipboard.AutoPaste || cfg.Clipboard.AutoPasteDelayMs != 120 || cfg.Clipboard.PasteKey != "ctrl+v" {
		t.Fatalf("clipboard config not applied: %+v", cfg.Clipboard)
	}
	if cfg.Hotkey.Enabled || cfg.Hotkey.Accelerator != "Ctrl+Alt+J" {
		t.Fatalf("hotkey config not applied: %+v", cfg.Hotkey)
	}
	if cfg.Dictionary.ExternalPath != "/tmp/SKK-JISYO.L.json" {
		t.Fatalf("dictionary config not applied: %+v", cfg.Dictionary)
	}
}
