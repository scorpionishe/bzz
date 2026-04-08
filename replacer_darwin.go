//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon

#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <dispatch/dispatch.h>

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
	"log"
	"sync/atomic"
	"time"
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
func sendEnter()        { C.sendEnter() }

func replaceText(buf *Buffer, deleteChars int, newText string) {
	if !atomic.CompareAndSwapInt32(&replacing, 0, 1) {
		log.Printf("REPLACE SKIPPED (already replacing): %q", newText)
		return
	}
	log.Printf("REPLACE START: delete=%d text=%q", deleteChars, newText)
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
	log.Printf("REPLACE TYPED: %q", newText)

	// Switch system layout after typing
	C.switchLayout()
	time.Sleep(30 * time.Millisecond)

	atomic.StoreInt32(&replacing, 0)
	log.Printf("REPLACE DONE")
}
