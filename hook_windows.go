//go:build windows

package main

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHook  = user32.NewProc("SetWindowsHookExW")
	procCallNextHook    = user32.NewProc("CallNextHookEx")
	procGetMessage      = user32.NewProc("GetMessageW")
	procUnhookWindows   = user32.NewProc("UnhookWindowsHookEx")
	procToUnicodeEx     = user32.NewProc("ToUnicodeEx")
	procGetKeyboardState = user32.NewProc("GetKeyboardState")
	procGetKeyboardLayout = user32.NewProc("GetKeyboardLayout")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	VK_BACK        = 0x08
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

var (
	hookHandle uintptr
	hookEvents chan KeyEvent
	hookMu     sync.Mutex
)

func startHook() (<-chan KeyEvent, error) {
	events := make(chan KeyEvent, 256)
	hookMu.Lock()
	hookEvents = events
	hookMu.Unlock()

	go func() {
		callback := syscall.NewCallback(func(nCode int, wParam uintptr, lParam uintptr) uintptr {
			if nCode >= 0 && wParam == WM_KEYDOWN {
				if atomic.LoadInt32(&replacing) == 1 {
					ret, _, _ := procCallNextHook.Call(hookHandle, uintptr(nCode), wParam, lParam)
					return ret
				}

				kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
				ch := vkToUnicode(kb.VkCode, kb.ScanCode)

				evt := KeyEvent{
					KeyCode: uint16(kb.VkCode),
					Char:    ch,
					Time:    time.Now(),
				}

				hookMu.Lock()
				evCh := hookEvents
				hookMu.Unlock()

				if evCh != nil {
					select {
					case evCh <- evt:
					default:
					}
				}
			}
			ret, _, _ := procCallNextHook.Call(hookHandle, uintptr(nCode), wParam, lParam)
			return ret
		})

		h, _, _ := procSetWindowsHook.Call(WH_KEYBOARD_LL, callback, 0, 0)
		hookHandle = h

		// Message pump (required for hooks)
		var msg MSG
		for {
			procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		}
	}()

	// Give the hook time to install
	time.Sleep(100 * time.Millisecond)
	return events, nil
}

func vkToUnicode(vkCode, scanCode uint32) rune {
	var keyState [256]byte
	procGetKeyboardState.Call(uintptr(unsafe.Pointer(&keyState[0])))

	hwnd, _, _ := procGetForegroundWindow.Call()
	tid, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
	hkl, _, _ := procGetKeyboardLayout.Call(tid)

	var buf [4]uint16
	ret, _, _ := procToUnicodeEx.Call(
		uintptr(vkCode),
		uintptr(scanCode),
		uintptr(unsafe.Pointer(&keyState[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		4, 0,
		hkl,
	)
	if int32(ret) > 0 {
		return rune(buf[0])
	}
	return 0
}
