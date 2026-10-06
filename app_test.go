package main

import (
	"testing"

	"github.com/takeshy/skk-popup/internal/clipboard"
	"github.com/takeshy/skk-popup/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type focusRecorder struct {
	remembered int
	restored   int
}

func (*focusRecorder) Name() string           { return "test" }
func (f *focusRecorder) RememberFocus()       { f.remembered++ }
func (f *focusRecorder) RestoreFocus()        { f.restored++ }
func (*focusRecorder) PasteKeys(string) error { return nil }

type popupRecorder struct {
	*application.WebviewWindow
	hides int
}

func (w *popupRecorder) Hide() application.Window {
	w.hides++
	return nil
}

func TestHidePopupHidesWindowEvenWhenVisibilityFlagIsStale(t *testing.T) {
	for _, visible := range []bool{false, true} {
		window := &popupRecorder{}
		desktop := &focusRecorder{}
		app := &App{
			window: window, desktop: desktop, cfg: config.Default(),
			visible: visible, pendingShow: true,
		}
		app.HidePopup()
		app.HidePopup()
		if window.hides != 2 {
			t.Fatalf("visible=%v: native window hidden %d times, want 2", visible, window.hides)
		}
		if app.visible || app.pendingShow || app.pasteAfterHide {
			t.Fatalf("visible=%v: popup state was not cleared", visible)
		}
		wantRestores := 0
		if visible {
			wantRestores = 1
		}
		if desktop.restored != wantRestores {
			t.Fatalf("visible=%v: restored focus %d times, want %d", visible, desktop.restored, wantRestores)
		}
	}
}

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
