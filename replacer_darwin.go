//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon -framework AppKit

#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <AppKit/AppKit.h>
#include <dispatch/dispatch.h>

// readClipboardString returns a strdup'd UTF-8 string from the clipboard, or NULL.
// Caller must free the returned pointer.
static const char* readClipboardString(void) {
    NSPasteboard* pb = [NSPasteboard generalPasteboard];
    NSString* s = [pb stringForType:NSPasteboardTypeString];
    if (s == nil) return NULL;
    return strdup([s UTF8String]);
}

// writeClipboardString sets the clipboard to the given UTF-8 string.
static void writeClipboardString(const char* utf8) {
    NSPasteboard* pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSString* s = [NSString stringWithUTF8String:utf8];
    [pb setString:s forType:NSPasteboardTypeString];
}

// clearModifiers posts key-up events for every modifier so no synthetic
// modifier stays logically "stuck". Without this, the Cmd from the triggering
// Cmd+Shift+X hotkey (or from our own Cmd+C/Cmd+V) can linger, turning the
// user's next Space into Cmd+Space (Spotlight). Posting an up for a key that
// is not down is harmless.
static void clearModifiers(void) {
    // L/R command, L/R shift, L/R control, L/R option
    CGKeyCode mods[] = {0x37, 0x36, 0x38, 0x3C, 0x3B, 0x3E, 0x3A, 0x3D};
    for (int i = 0; i < 8; i++) {
        CGEventRef up = CGEventCreateKeyboardEvent(NULL, mods[i], false);
        CGEventSetFlags(up, 0);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(up);
    }
}

// sendCmdC sends Cmd+C (copy)
static void sendCmdC(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0x08, true);
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0x08, false);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

// sendCmdV sends Cmd+V (paste)
static void sendCmdV(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0x09, true);
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0x09, false);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

void sendBackspace(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0x33, true);
    CGEventRef up   = CGEventCreateKeyboardEvent(NULL, 0x33, false);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

// physicalKeycode returns the macOS virtual keycode for common ASCII chars.
// For Cyrillic and other chars we use 0 (overridden by the unicode string).
// Using the correct keycode for ASCII punctuation prevents apps from
// re-interpreting the event using the current keyboard layout.
static CGKeyCode physicalKeycode(UniChar ch) {
    switch (ch) {
        case ' ':  return 0x31; // kVK_Space
        case ',':  return 0x2B; // kVK_ANSI_Comma
        case '.':  return 0x2F; // kVK_ANSI_Period
        case ';':  return 0x29; // kVK_ANSI_Semicolon
        case '\'': return 0x27; // kVK_ANSI_Quote
        case '[':  return 0x21; // kVK_ANSI_LeftBracket
        case ']':  return 0x1E; // kVK_ANSI_RightBracket
        case '`':  return 0x32; // kVK_ANSI_Grave
        default:   return 0;
    }
}

void sendEnter(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0x24, true);
    CGEventRef up   = CGEventCreateKeyboardEvent(NULL, 0x24, false);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

void sendUnichar(UniChar ch) {
    CGKeyCode kc = physicalKeycode(ch);
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, kc, true);
    CGEventRef up   = CGEventCreateKeyboardEvent(NULL, kc, false);
    UniChar c[1] = {ch};
    CGEventKeyboardSetUnicodeString(down, 1, c);
    CGEventKeyboardSetUnicodeString(up, 1, c);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

// Switch layout using TIS API directly (called from any thread)
void switchLayout(void) {
    CFDictionaryRef filter = CFDictionaryCreate(
        kCFAllocatorDefault,
        (const void *[]){kTISPropertyInputSourceCategory},
        (const void *[]){kTISCategoryKeyboardInputSource},
        1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks
    );
    CFArrayRef sources = TISCreateInputSourceList(filter, false);
    CFRelease(filter);
    if (!sources) return;

    TISInputSourceRef current = TISCopyCurrentKeyboardInputSource();
    if (!current) { CFRelease(sources); return; }

    CFStringRef currentID = TISGetInputSourceProperty(current, kTISPropertyInputSourceID);
    CFIndex count = CFArrayGetCount(sources);

    // Collect only selectable keyboard sources
    TISInputSourceRef selectable[16];
    int nsel = 0;
    for (CFIndex i = 0; i < count && nsel < 16; i++) {
        TISInputSourceRef s = (TISInputSourceRef)CFArrayGetValueAtIndex(sources, i);
        CFBooleanRef canSelect = TISGetInputSourceProperty(s, kTISPropertyInputSourceIsSelectCapable);
        if (canSelect && CFBooleanGetValue(canSelect)) {
            selectable[nsel++] = s;
        }
    }

    // Find current index and pick next
    int curIdx = -1;
    for (int i = 0; i < nsel; i++) {
        CFStringRef sid = TISGetInputSourceProperty(selectable[i], kTISPropertyInputSourceID);
        if (sid && currentID && CFStringCompare(sid, currentID, 0) == kCFCompareEqualTo) {
            curIdx = i;
            break;
        }
    }

    if (curIdx >= 0 && nsel > 1) {
        int nextIdx = (curIdx + 1) % nsel;
        OSStatus err = TISSelectInputSource(selectable[nextIdx]);
        (void)err;
    }

    CFRelease(current);
    CFRelease(sources);
}
// selectLayout jumps straight to a specific keyboard input source:
//   wantRussian != 0 -> first selectable source whose ID contains "Russian"
//   wantRussian == 0 -> first selectable ASCII-capable, non-Russian source (ABC / U.S.)
// Unlike switchLayout() — which cycles to the *next* source and lands on the
// wrong one when there are >2 sources (e.g. ABC + Russian + Character Viewer) —
// this targets the correct layout directly.
void selectLayout(int wantRussian) {
    CFDictionaryRef filter = CFDictionaryCreate(
        kCFAllocatorDefault,
        (const void *[]){kTISPropertyInputSourceCategory},
        (const void *[]){kTISCategoryKeyboardInputSource},
        1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks
    );
    CFArrayRef sources = TISCreateInputSourceList(filter, false);
    CFRelease(filter);
    if (!sources) return;

    CFIndex count = CFArrayGetCount(sources);
    for (CFIndex i = 0; i < count; i++) {
        TISInputSourceRef s = (TISInputSourceRef)CFArrayGetValueAtIndex(sources, i);
        CFBooleanRef canSelect = TISGetInputSourceProperty(s, kTISPropertyInputSourceIsSelectCapable);
        if (!canSelect || !CFBooleanGetValue(canSelect)) continue;

        CFStringRef sid = TISGetInputSourceProperty(s, kTISPropertyInputSourceID);
        if (!sid) continue;
        int isRussian = (CFStringFind(sid, CFSTR("Russian"), kCFCompareCaseInsensitive).location != kCFNotFound) ? 1 : 0;

        if (wantRussian) {
            if (isRussian) { TISSelectInputSource(s); break; }
        } else {
            if (isRussian) continue;
            CFBooleanRef ascii = TISGetInputSourceProperty(s, kTISPropertyInputSourceIsASCIICapable);
            if (ascii && CFBooleanGetValue(ascii)) { TISSelectInputSource(s); break; }
        }
    }
    CFRelease(sources);
}
// Returns 1 if current input source contains "Russian", 0 otherwise
int isCurrentLayoutRussian(void) {
    TISInputSourceRef current = TISCopyCurrentKeyboardInputSource();
    if (!current) return 0;

    CFStringRef sourceID = TISGetInputSourceProperty(current, kTISPropertyInputSourceID);
    int result = 0;
    if (sourceID) {
        // Russian layouts typically have "Russian" in their ID
        CFRange range = CFStringFind(sourceID, CFSTR("Russian"), kCFCompareCaseInsensitive);
        if (range.location != kCFNotFound) result = 1;
    }
    CFRelease(current);
    return result;
}
*/
import "C"

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// replacing guards against hook feedback loop
var replacing int32

// IsRussianLayout returns true if macOS is currently on Russian input source
func IsRussianLayout() bool {
	return C.isCurrentLayoutRussian() == 1
}

// Go wrappers for C functions — used by main.go for Enter handling
func sendBackspaceKey() { C.sendBackspace() }
func sendChar(ch rune)  { C.sendUnichar(C.UniChar(ch)) }
func switchLang()       { C.switchLayout() }

// selectLayout targets a specific layout (Russian or ASCII/English) directly,
// instead of cycling to the next source like switchLang().
func selectLayout(russian bool) {
	if russian {
		C.selectLayout(1)
	} else {
		C.selectLayout(0)
	}
}
func sendEnter()        { C.sendEnter() }

// Clipboard + paste helpers for the manual-conversion hotkey (Cmd+Shift+X)
func readClipboard() string {
	cstr := C.readClipboardString()
	if cstr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

func writeClipboard(s string) {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	C.writeClipboardString(cs)
}

func sendCopy()  { C.sendCmdC() }
func sendPaste() { C.sendCmdV() }

// clearModifiers releases any stuck synthetic modifiers (Cmd/Shift/Ctrl/Opt)
// so the triggering hotkey or our copy/paste can't leak into the next keypress.
func clearModifiers() { C.clearModifiers() }

func replaceText(buf *Buffer, deleteChars int, newText string) {
	if !atomic.CompareAndSwapInt32(&replacing, 0, 1) {
		vlog("REPLACE SKIPPED (already replacing): %q", newText)
		return
	}
	vlog("REPLACE START: delete=%d text=%q", deleteChars, newText)
	buf.Clear()

	// Delete old text (word + boundary char)
	for i := 0; i < deleteChars; i++ {
		C.sendBackspace()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)

	// Type corrected text + space
	for _, ch := range newText {
		C.sendUnichar(C.UniChar(ch))
		time.Sleep(5 * time.Millisecond)
	}
	vlog("REPLACE TYPED: %q", newText)

	// Deliberately do NOT touch the system input source on auto-correction.
	// Punto-style behavior: keep the user in their single typing layout and just
	// fix each wrong-layout word in place. Switching the layout mid-stream
	// changes what subsequent physical keys produce, which makes a mixed
	// sentence (RU-in-latin + real English) inconsistent — some words land
	// already-correct, others don't. The manual hotkey still targets the layout,
	// since there the user explicitly converts one word and continues in it.
	time.Sleep(10 * time.Millisecond)

	atomic.StoreInt32(&replacing, 0)
	vlog("REPLACE DONE")
}
