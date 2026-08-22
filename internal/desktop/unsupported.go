//go:build !linux && !windows && !darwin

package desktop

import "errors"

// Unsupported keeps skk-popup buildable on other platforms; paste and focus
// restoration are no-ops that fail explicitly.
type Unsupported struct{}

var ErrUnsupported = errors.New("skk-popup: this platform has no paste/focus support")

func newPlatform() Platform { return Unsupported{} }

func (Unsupported) Name() string { return "unsupported" }

func (Unsupported) RememberFocus() {}

func (Unsupported) RestoreFocus() {}

func (Unsupported) PasteKeys(shortcut string) error { return ErrUnsupported }
