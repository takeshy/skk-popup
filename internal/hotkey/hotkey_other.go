//go:build !windows

package hotkey

import "errors"

// ErrUnsupported is returned on platforms where skk-popup does not register
// its own global hotkey.
var ErrUnsupported = errors.New("hotkey: not supported on this platform")

// Manager is a no-op placeholder outside Windows.
type Manager struct{}

// Start always fails outside Windows; callers should fall back to the
// platform's own shortcut facilities.
func Start(string, func()) (*Manager, error) { return nil, ErrUnsupported }

// Stop is a no-op outside Windows.
func (*Manager) Stop() {}
