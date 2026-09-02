package config

import (
	"encoding/json"
	"testing"
)

func TestViewRoundTripAndValidation(t *testing.T) {
	in := `{"window":{"width":720,"height":300,"restoreFocus":false},"clipboard":{"backend":"wails","autoPaste":true,"autoPasteDelayMs":90,"pasteKey":"ctrl+v"},"hotkey":{"enabled":true,"accelerator":"Ctrl + Alt + J"},"dictionary":{"externalPath":""}}`
	var v View
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatal(err)
	}
	cfg, err := FromView(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hotkey.Accelerator != "Ctrl+Alt+J" || cfg.Window.Width != 720 || cfg.Clipboard.PasteKey != "ctrl+v" || cfg.Clipboard.AutoPasteDelayMs != 90 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	got := ToView(cfg)
	v.Hotkey.Accelerator = "Ctrl+Alt+J" // normalised form
	if got != v {
		t.Fatalf("view round trip mismatch:\n got %+v\nwant %+v", got, v)
	}

	bad := v
	bad.Clipboard.Backend = "xclip"
	if _, err := FromView(bad); err == nil {
		t.Fatal("invalid backend accepted")
	}
	bad = v
	bad.Clipboard.PasteKey = "cmd+v"
	if _, err := FromView(bad); err == nil {
		t.Fatal("invalid paste key accepted")
	}
	bad = v
	bad.Window.Width = 10
	if _, err := FromView(bad); err == nil {
		t.Fatal("tiny window accepted")
	}
	bad = v
	bad.Hotkey.Accelerator = "Ctrl+Shift+Ö"
	if _, err := FromView(bad); err == nil {
		t.Fatal("invalid accelerator accepted")
	}
	bad = v
	bad.Dictionary.ExternalPath = "/definitely/missing/file"
	if _, err := FromView(bad); err == nil {
		t.Fatal("missing dictionary accepted")
	}
}
