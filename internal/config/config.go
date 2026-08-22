// Package config loads the wl-skk configuration file.
//
// Only the small TOML subset used by wl-skk is supported: [section]
// headers and `key = value` pairs where value is a string ("..."),
// integer, or boolean. Unknown keys are ignored.
package config

import (
	"bufio"
	"os"
	"path/filepath"
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

type Config struct {
	Window     WindowConfig
	Clipboard  ClipboardConfig
	Dictionary DictionaryConfig
}

func Default() *Config {
	return &Config{
		Window: WindowConfig{
			Width:        600,
			Height:       240,
			RestoreFocus: true,
		},
		Clipboard: ClipboardConfig{
			Backend:          "wl-copy",
			AutoPaste:        false,
			AutoPasteDelayMs: 80,
			PasteKey:         "ctrl+shift+v",
		},
	}
}

// Dir returns $XDG_CONFIG_HOME/wl-skk (or ~/.config/wl-skk).
func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "wl-skk")
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
		value := strings.TrimSpace(line[eq+1:])
		if i := strings.Index(value, "#"); i >= 0 && !strings.HasPrefix(value, "\"") {
			value = strings.TrimSpace(value[:i])
		}
		if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}
		cfg.apply(section, key, value)
	}
	return cfg
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
	}
}

func atoiOr(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
