package main

import (
	"testing"

	"github.com/takeshy/skk-popup/internal/clipboard"
	"github.com/takeshy/skk-popup/internal/config"
)

type focusRecorder struct {
	remembered int
}

func (*focusRecorder) Name() string           { return "test" }
func (f *focusRecorder) RememberFocus()       { f.remembered++ }
func (*focusRecorder) RestoreFocus()          {}
func (*focusRecorder) PasteKeys(string) error { return nil }

func TestShowPopupDoesNotRecaptureFocusWhenAlreadyVisible(t *testing.T) {
	desktop := &focusRecorder{}
	app := &App{ready: true, visible: true, desktop: desktop}

	app.ShowPopup()

	if desktop.remembered != 0 {
		t.Fatalf("RememberFocus called %d times for an already visible popup", desktop.remembered)
	}
}

func TestCopyFailureDoesNotArmAutoPaste(t *testing.T) {
	cfg := config.Default()
	cfg.Clipboard.AutoPaste = true
	app := &App{
		cfg:            cfg,
		copier:         clipboard.New("wails", func(string) bool { return false }),
		pasteAfterHide: true,
	}

	if err := app.CopyToClipboard("new text"); err == nil {
		t.Fatal("expected clipboard copy to fail")
	}
	if app.pasteAfterHide {
		t.Fatal("auto-paste remained armed after clipboard copy failed")
	}
}

func TestSuccessfulCopyArmsAutoPaste(t *testing.T) {
	cfg := config.Default()
	cfg.Clipboard.AutoPaste = true
	app := &App{
		cfg:    cfg,
		copier: clipboard.New("wails", func(string) bool { return true }),
	}

	if err := app.CopyToClipboard("new text"); err != nil {
		t.Fatalf("CopyToClipboard: %v", err)
	}
	if !app.pasteAfterHide {
		t.Fatal("auto-paste was not armed after clipboard copy succeeded")
	}
}
