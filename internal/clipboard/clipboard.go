// Package clipboard copies text to the system clipboard, preferring
// wl-clipboard over the Wails runtime binding. Paste keystroke synthesis
// and focus restoration live in internal/desktop.
package clipboard

import (
	"context"
	"os/exec"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Copier struct {
	mu      sync.Mutex
	backend string
	ctx     context.Context
	lastCmd *exec.Cmd
}

// New creates a copier. ctx is the Wails app context; it may be nil when
// only the wl-copy backend is used.
func New(backend string, ctx context.Context) *Copier {
	if backend != "wails" {
		backend = "wl-copy"
	}
	return &Copier{backend: backend, ctx: ctx}
}

// Copy places text on the clipboard.
//
// With the wl-copy backend a previous wl-copy child is killed first so
// repeated copies do not accumulate background processes; wl-copy itself
// forks immediately and keeps serving the selection from the new process.
// The immediate child still needs to be reaped once it exits (whether it
// forked away on its own or was killed here), so each one is waited on in
// its own goroutine instead of being left as a zombie for the life of the
// daemon.
func (c *Copier) Copy(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.backend == "wl-copy" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command("wl-copy")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Start(); err != nil {
				return err
			}
			go cmd.Wait()
			if c.lastCmd != nil && c.lastCmd.Process != nil {
				_ = c.lastCmd.Process.Kill()
			}
			c.lastCmd = cmd
			return nil
		}
	}
	if c.ctx == nil {
		return errNoBackend{}
	}
	return wailsruntime.ClipboardSetText(c.ctx, text)
}

type errNoBackend struct{}

func (errNoBackend) Error() string { return "no clipboard backend available" }
