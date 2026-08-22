//go:build !linux && !windows && !darwin

package desktop

import "errors"

// Unsupported keeps wl-skk buildable on other platforms; paste and focus
// restoration are no-ops that fail explicitly.
type Unsupported struct{}

var ErrUnsupported = errors.New("wl-skk: this platform has no paste/focus support")

func newPlatform() Platform { return Unsupported{} }

func (Unsupported) Name() string { return "unsupported" }

func (Unsupported) RememberFocus() {}

func (Unsupported) RestoreFocus() {}

func (Unsupported) PasteKeys(shortcut string) error { return ErrUnsupported }
