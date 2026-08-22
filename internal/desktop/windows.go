//go:build windows

// Windows implements paste via SendInput and focus restoration by
// remembering the foreground window handle captured right before the
// popup window is shown.
package desktop

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSendInput           = user32.NewProc("SendInput")
)

const (
	inputKeyboard  = 1
	keyeventfKeyup = 0x0002
	vkShift        = 0x10
	vkControl      = 0x11
	vkV            = 0x56
	swRestore      = 9
)

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// winInput mirrors C INPUT: DWORD type; union { MOUSEINPUT mi;
// KEYBDINPUT ki; ... }. The union starts at offset 8 (pointer alignment)
// and MOUSEINPUT, the largest member, is 32 bytes on 64-bit Windows.
type winInput struct {
	msgType uint32
	_       uint32
	ki      keybdInput
	_       [8]byte
}

func sendKey(vk uint16, flags uint32) error {
	var in winInput
	in.msgType = inputKeyboard
	in.ki.wVk = vk
	in.ki.dwFlags = flags
	ret, _, callErr := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if ret == 0 {
		return fmt.Errorf("SendInput failed: %v", callErr)
	}
	return nil
}

// Windows implements Platform for Microsoft Windows.
type Windows struct {
	mu       sync.Mutex
	prevHwnd uintptr
}

func newPlatform() Platform { return &Windows{} }

func (*Windows) Name() string { return "windows" }

// RememberFocus snapshots the currently focused window. Called just
// before the popup takes focus.
func (d *Windows) RememberFocus() {
	hwnd, _, _ := procGetForegroundWindow.Call()
	d.mu.Lock()
	d.prevHwnd = hwnd
	d.mu.Unlock()
}

// RestoreFocus returns focus to the remembered window. Because our own
// window held focus immediately before this call, Windows permits the
// switch in practice; if it is denied the popup simply stays hidden with
// focus on whatever the compositor picked.
func (d *Windows) RestoreFocus() {
	d.mu.Lock()
	hwnd := d.prevHwnd
	d.prevHwnd = 0
	d.mu.Unlock()
	if hwnd == 0 {
		return
	}
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
	procSetForegroundWindow.Call(hwnd)
}

// PasteKeys sends the configured paste shortcut to whatever window has
// keyboard focus.
func (d *Windows) PasteKeys(shortcut string) error {
	type keyStep struct {
		vk    uint16
		flags uint32
	}
	modifiers := []uint16{vkControl}
	if shortcut == PasteCtrlShiftV {
		// Windows terminals paste on Ctrl+Shift+V.
		modifiers = append(modifiers, vkShift)
	}

	steps := make([]keyStep, 0, len(modifiers)*2+2)
	for _, vk := range modifiers {
		steps = append(steps, keyStep{vk: vk})
	}
	steps = append(steps, keyStep{vk: vkV}, keyStep{vk: vkV, flags: keyeventfKeyup})
	for i := len(modifiers) - 1; i >= 0; i-- {
		steps = append(steps, keyStep{vk: modifiers[i], flags: keyeventfKeyup})
	}

	for _, step := range steps {
		if err := sendKey(step.vk, step.flags); err != nil {
			return err
		}
	}
	return nil
}
