//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon -framework ApplicationServices

#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <ApplicationServices/ApplicationServices.h>

// Must match BZZ_USERDATA in replacer_darwin.go — stamped on events WE synthesize.
#define BZZ_USERDATA 0x627a7a

// goKeyCallback returns 1 to suppress the event, 0 to pass through.
// isSynthetic is 1 when the event carries our user-data stamp (our own
// backspace/retype/replay), 0 for a real user keystroke.
extern int goKeyCallback(int64_t keycode, UniChar character, int64_t flags, int isSynthetic);

static CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        CGEventTapEnable((CFMachPortRef)refcon, true);
        return event;
    }

    if (type != kCGEventKeyDown) return event;

    CGKeyCode keycode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    CGEventFlags flags = CGEventGetFlags(event);
    int isSynthetic = (CGEventGetIntegerValueField(event, kCGEventSourceUserData) == BZZ_USERDATA) ? 1 : 0;

    UniChar chars[4];
    UniCharCount len = 0;
    CGEventKeyboardGetUnicodeString(event, 4, &len, chars);
    UniChar ch = (len > 0) ? chars[0] : 0;

    int suppress = goKeyCallback((int64_t)keycode, ch, (int64_t)flags, isSynthetic);
    if (suppress) return NULL;
    return event;
}

static bool checkAccessibility(void) {
    return AXIsProcessTrusted();
}

static void promptAccessibility(void) {
    NSDictionary *opts = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts);
}

static CFMachPortRef createTap(void) {
    CGEventMask mask = (1 << kCGEventKeyDown);
    CFMachPortRef tap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,
        mask,
        eventCallback,
        NULL
    );
    return tap;
}

static int isTapNull(CFMachPortRef p) { return p == NULL; }
*/
import "C"

import (
	"errors"
	"sync/atomic"
	"time"
)

const (
	kCGEventFlagMaskCommand = 1 << 20
)

// onKeyEvent is set by main() — called synchronously from CGEventTap callback.
// Returns true to suppress the event.
var onKeyEvent func(keycode uint16, char rune, flags int64) bool

//export goKeyCallback
func goKeyCallback(keycode C.int64_t, character C.UniChar, flags C.int64_t, isSynthetic C.int) C.int {
	// Our own synthetic events (backspace/retype/replay) always pass through and
	// are never fed to the buffer — they carry the BZZ_USERDATA stamp.
	if isSynthetic != 0 {
		return 0
	}

	// A real keystroke arriving while a replace is in flight: suppress it now and
	// queue it for replay once the replace finishes. This stops the user's fast
	// typing from interleaving with our synthetic backspace+retype.
	if atomic.LoadInt32(&replacing) == 1 {
		captureReplay(uint16(keycode), rune(character), int64(flags))
		return 1
	}

	if onKeyEvent != nil {
		if onKeyEvent(uint16(keycode), rune(character), int64(flags)) {
			return 1 // suppress
		}
	}
	return 0
}

func startHook() error {
	if !bool(C.checkAccessibility()) {
		C.promptAccessibility()
		return errors.New("grant Accessibility in System Settings > Privacy & Security > Accessibility, then restart")
	}

	tap := C.createTap()
	if C.isTapNull(tap) != 0 {
		return errors.New("failed to create event tap")
	}

	src := C.CFMachPortCreateRunLoopSource(C.kCFAllocatorDefault, tap, 0)

	go func() {
		rl := C.CFRunLoopGetCurrent()
		C.CFRunLoopAddSource(rl, src, C.kCFRunLoopCommonModes)
		C.CGEventTapEnable(tap, C.bool(true))
		C.CFRunLoopRun()
	}()

	// Give the run loop a moment to start
	time.Sleep(50 * time.Millisecond)
	return nil
}
