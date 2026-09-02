// Package config loads the skk-popup configuration file.
//
// Only the small TOML subset used by skk-popup is supported: [section]
// headers and `key = value` pairs where value is a string ("..."),
// integer, or boolean. Unknown keys are ignored.
package config

import (
	"bufio"
	"errors"
	"fmt"
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
			value = unquote(value[1 : len(value)-1])
		}
		cfg.apply(section, key, value)
	}
	return cfg
}

// quote renders a TOML basic string, escaping only the backslash and the
// double quote so Windows paths stay readable.
func quote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

// unquote reverses quote; other escape sequences are kept verbatim.
func unquote(value string) string {
	if !strings.Contains(value, "\\") {
		return value
	}
	var b strings.Builder
	escaped := false
	for _, r := range value {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	return b.String()
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

// Marshal renders cfg as config.toml text covering every supported key,
// with the same comments as the README example so a file written by the
// Settings dialog stays readable and hand-editable.
func Marshal(c *Config) string {
	var b strings.Builder
	b.WriteString("[window]\n")
	fmt.Fprintf(&b, "width = %d\n", c.Window.Width)
	fmt.Fprintf(&b, "height = %d\n", c.Window.Height)
	b.WriteString("# 閉じたあとに直前のウィンドウへフォーカスを戻す\n")
	fmt.Fprintf(&b, "restore_focus = %t\n", c.Window.RestoreFocus)
	b.WriteString("\n[clipboard]\n")
	b.WriteString("# \"wl-copy\" | \"wails\" (既定: Linux=wl-copy, Windows/macOS=wails)\n")
	fmt.Fprintf(&b, "backend = %s\n", quote(c.Clipboard.Backend))
	b.WriteString("# コピー後に自動で貼り付けショートカットを送出\n")
	fmt.Fprintf(&b, "auto_paste = %t\n", c.Clipboard.AutoPaste)
	b.WriteString("# 自動貼り付け時、フォーカス復帰から送出までの待ち時間 (ミリ秒)\n")
	fmt.Fprintf(&b, "auto_paste_delay_ms = %d\n", c.Clipboard.AutoPasteDelayMs)
	b.WriteString("# \"ctrl+v\" | \"ctrl+shift+v\"\n")
	fmt.Fprintf(&b, "paste_key = %s\n", quote(c.Clipboard.PasteKey))
	b.WriteString("\n[hotkey]\n")
	b.WriteString("# Windows のみ有効。アプリ内でグローバルホットキーを登録する\n")
	fmt.Fprintf(&b, "enabled = %t\n", c.Hotkey.Enabled)
	fmt.Fprintf(&b, "accelerator = %s\n", quote(c.Hotkey.Accelerator))
	b.WriteString("\n[dictionary]\n")
	b.WriteString("# 外部辞書ファイルのパス。指定するとバイナリ埋め込みより優先される (省略可)\n")
	fmt.Fprintf(&b, "external_path = %s\n", quote(c.Dictionary.ExternalPath))
	return b.String()
}

// Save writes cfg to path (creating the directory), replacing the file
// atomically so a crash mid-write cannot leave a truncated config.
func Save(path string, c *Config) error {
	if path == "" {
		return errors.New("config: no configuration path for this platform")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(Marshal(c)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
