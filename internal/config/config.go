// Package config loads the skk-popup configuration file.
//
// Only the small TOML subset used by skk-popup is supported: [section]
// headers and `key = value` pairs where value is a string ("..."),
// integer, or boolean. Unknown keys are ignored.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type WindowConfig struct {
	Width        int
	Height       int
	RestoreFocus bool
}

type ClipboardConfig struct {
	Backend          string // "wl-copy" | "wails"
	AutoPaste        bool
	AutoPasteDelayMs int
	PasteKey         string // "ctrl+v" | "ctrl+shift+v"
}

type DictionaryConfig struct {
	ExternalPath string
}

// HotkeyConfig registers an in-app global hotkey. This exists for
// Windows, which has no compositor bind equivalent; on Linux the binding
// belongs to Hyprland and on macOS to OS-level shortcut facilities
// (Shortcuts.app etc.), so the default is disabled there.
type HotkeyConfig struct {
	Enabled     bool
	Accelerator string // "Ctrl+Shift+K" style
}

type Config struct {
	Window     WindowConfig
	Clipboard  ClipboardConfig
	Dictionary DictionaryConfig
	Hotkey     HotkeyConfig
}

func Default() *Config {
	return &Config{
		Window: WindowConfig{
			Width:        600,
			Height:       240,
			RestoreFocus: true,
		},
		Clipboard: ClipboardConfig{
			Backend:          defaultClipboardBackend(),
			AutoPaste:        true,
			AutoPasteDelayMs: 80,
			PasteKey:         "ctrl+shift+v",
		},
		Hotkey: HotkeyConfig{
			Enabled:     runtime.GOOS == "windows",
			Accelerator: "Ctrl+Shift+K",
		},
	}
}

func defaultClipboardBackend() string {
	if runtime.GOOS == "linux" {
		return "wl-copy"
	}
	// wl-clipboard does not exist on Windows/macOS; use the Wails
	// runtime binding (Win32 clipboard / NSPasteboard).
	return "wails"
}

// Dir returns the per-user configuration directory:
// $XDG_CONFIG_HOME/skk-popup (or ~/.config/skk-popup) on Linux,
// %AppData%/skk-popup on Windows, and ~/Library/Application Support/skk-popup
// on macOS.
func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "skk-popup")
}

// Path returns the path to the configuration file.
func Path() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// Load reads the configuration file, falling back to defaults for
// missing values or when the file does not exist.
func Load() *Config {
	path := Path()
	if path == "" {
		return Default()
	}
	return LoadFrom(path)
}

// LoadFrom parses the TOML subset at path on top of the defaults. A
// missing or unreadable file yields the defaults.
func LoadFrom(path string) *Config {
	cfg := Default()
	file, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := stripInlineComment(strings.TrimSpace(line[eq+1:]))
		if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}
		cfg.apply(section, key, value)
	}
	return cfg
}

func stripInlineComment(value string) string {
	inString := false
	escaped := false
	for i, r := range value {
		if inString && r == '\\' && !escaped {
			escaped = true
			continue
		}
		if r == '"' && !escaped {
			inString = !inString
		} else if r == '#' && !inString {
			return strings.TrimSpace(value[:i])
		}
		escaped = false
	}
	return strings.TrimSpace(value)
}

func (c *Config) apply(section, key, value string) {
	switch section {
	case "window":
		switch key {
		case "width":
			c.Window.Width = atoiOr(value, c.Window.Width)
		case "height":
			c.Window.Height = atoiOr(value, c.Window.Height)
		case "restore_focus":
			c.Window.RestoreFocus = value == "true"
		}
	case "clipboard":
		switch key {
		case "backend":
			if value == "wl-copy" || value == "wails" {
				c.Clipboard.Backend = value
			}
		case "auto_paste":
			c.Clipboard.AutoPaste = value == "true"
		case "auto_paste_delay_ms":
			c.Clipboard.AutoPasteDelayMs = atoiOr(value, c.Clipboard.AutoPasteDelayMs)
		case "paste_key":
			if value == "ctrl+v" || value == "ctrl+shift+v" {
				c.Clipboard.PasteKey = value
			}
		}
	case "dictionary":
		switch key {
		case "external_path":
			c.Dictionary.ExternalPath = value
		}
	case "hotkey":
		switch key {
		case "enabled":
			c.Hotkey.Enabled = value == "true"
		case "accelerator", "shortcut":
			if value != "" {
				c.Hotkey.Accelerator = strings.Join(strings.FieldsFunc(value, func(r rune) bool {
					return r == '+' || r == ' '
				}), "+")
			}
		}
	}
}

func atoiOr(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
