// Package ipc implements the Unix domain socket protocol used to control
// the running skk-popup daemon from short-lived CLI invocations.
//
// The protocol is line-based plain text: a client connects, sends one of
// "toggle" / "show" / "hide" / "quit", and receives either "ok" or
// "error: <message>".
package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// SocketPath returns the IPC socket path: $XDG_RUNTIME_DIR/skk-popup.sock on
// Linux, /tmp/skk-popup-$UID.sock as a Linux fallback, and a temp-directory
// path on Windows (Go's unix socket support works on Windows 10+).
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "skk-popup.sock")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "skk-popup.sock")
	}
	return fmt.Sprintf("/tmp/skk-popup-%s.sock", strconv.Itoa(os.Getuid()))
}

// IsValidCommand reports whether command is one accepted by the daemon.
func IsValidCommand(command string) bool {
	switch command {
	case "toggle", "show", "hide", "quit":
		return true
	default:
		return false
	}
}
