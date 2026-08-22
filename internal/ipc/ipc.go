// Package ipc implements the Unix domain socket protocol used to control
// the running wl-skk daemon from short-lived CLI invocations.
//
// The protocol is line-based plain text: a client connects, sends one of
// "toggle" / "show" / "hide" / "quit", and receives either "ok" or
// "error: <message>".
package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SocketPath returns $XDG_RUNTIME_DIR/wl-skk.sock, falling back to
// /tmp/wl-skk-$UID.sock when XDG_RUNTIME_DIR is not set.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "wl-skk.sock")
	}
	return fmt.Sprintf("/tmp/wl-skk-%s.sock", strconv.Itoa(os.Getuid()))
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
