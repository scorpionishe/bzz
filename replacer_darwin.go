//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon -framework AppKit -framework ApplicationServices

#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <AppKit/AppKit.h>
#include <ApplicationServices/ApplicationServices.h>
#include <dispatch/dispatch.h>

// Magic stamped into kCGEventSourceUserData on every event WE synthesize, so the
// event-tap callback can tell our own backspaces/retypes apart from the user's
// real keystrokes. Real keystrokes typed during a replace are suppressed and
// replayed afterwards (see hook_darwin.go) to stop them interleaving with our
// synthetic backspace+retype — the cause of garbled fast typing in VSCode.
#define BZZ_USERDATA 0x627a7a // 'bzz'

static void bzzTag(CGEventRef e) {
    CGEventSetIntegerValueField(e, kCGEventSourceUserData, BZZ_USERDATA);
}

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

// axSelectedText returns the text currently selected in the focused UI element
// via the Accessibility API, or NULL if there is no selection (or the app does
// not expose it — e.g. some Electron apps). Unlike Cmd+C this never grabs the
// whole line on an empty selection and never races the clipboard. Caller frees.
static const char* axSelectedText(void) {
    AXUIElementRef sys = AXUIElementCreateSystemWide();
    if (sys == NULL) return NULL;
    CFTypeRef focused = NULL;
    AXError err = AXUIElementCopyAttributeValue(sys, kAXFocusedUIElementAttribute, &focused);
    CFRelease(sys);
    if (err != kAXErrorSuccess || focused == NULL) return NULL;

    CFTypeRef sel = NULL;
    err = AXUIElementCopyAttributeValue((AXUIElementRef)focused, kAXSelectedTextAttribute, &sel);
    CFRelease(focused);
    if (err != kAXErrorSuccess || sel == NULL) return NULL;
    if (CFGetTypeID(sel) != CFStringGetTypeID()) { CFRelease(sel); return NULL; }

    CFStringRef s = (CFStringRef)sel;
    CFIndex len = CFStringGetLength(s);
    CFIndex maxSize = CFStringGetMaximumSizeForEncoding(len, kCFStringEncodingUTF8) + 1;
    char* buf = (char*)malloc(maxSize);
    if (buf == NULL) { CFRelease(sel); return NULL; }
    if (!CFStringGetCString(s, buf, maxSize, kCFStringEncodingUTF8)) {
        free(buf); CFRelease(sel); return NULL;
    }
    CFRelease(sel);
    return buf;
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
        bzzTag(up);
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
    bzzTag(down);
    bzzTag(up);
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
    bzzTag(down);
    bzzTag(up);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

void sendBackspace(void) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0x33, true);
    CGEventRef up   = CGEventCreateKeyboardEvent(NULL, 0x33, false);
    bzzTag(down);
    bzzTag(up);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

// postPhysicalReplay re-injects a user keystroke that we suppressed during a
// replace. It is deliberately NOT tagged, so it flows back through the tap as a
// normal physical event — buffered and passed to the app exactly as if the user
// had typed it just now, but after our synthetic retype instead of interleaved
// with it. The unicode override makes the produced character layout-independent.
void postPhysicalReplay(uint16_t keycode, uint64_t flags, UniChar ch) {
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keycode, true);
    CGEventRef up   = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keycode, false);
    CGEventSetFlags(down, (CGEventFlags)flags);
    CGEventSetFlags(up, (CGEventFlags)flags);
    if (ch != 0) {
        UniChar c[1] = {ch};
        CGEventKeyboardSetUnicodeString(down, 1, c);
        CGEventKeyboardSetUnicodeString(up, 1, c);
    }
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
    bzzTag(down);
    bzzTag(up);
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
    bzzTag(down);
    bzzTag(up);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
}

// Switch layout using the TIS API. TIS/TSM is main-thread-only on modern
// macOS: its lazy cache-refresh paths carry dispatch_assert_queue(main), and
// a call from any other thread intermittently aborts the whole process
// ("BUG IN CLIENT OF LIBDISPATCH") with no crash report. Hop to the main
// queue; the switch is fire-and-forget so async is fine.
void switchLayout(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
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
    });
}
// selectLayout jumps straight to a specific keyboard input source:
//   wantRussian != 0 -> first selectable source whose ID contains "Russian"
//   wantRussian == 0 -> first selectable ASCII-capable, non-Russian source (ABC / U.S.)
// Unlike switchLayout() — which cycles to the *next* source and lands on the
// wrong one when there are >2 sources (e.g. ABC + Russian + Character Viewer) —
// this targets the correct layout directly.
// Main-queue hop for the same reason as switchLayout(): callers are replace
// goroutines, and TIS off the main thread intermittently kills the process.
void selectLayout(int wantRussian) {
    dispatch_async(dispatch_get_main_queue(), ^{
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
    });
}
// Returns 1 if current input source contains "Russian", 0 otherwise.
// TIS query — main thread only; Go callers go through refreshLayoutCache().
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
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// replacing guards against hook feedback loop
var replacing int32

// replayKey is a user keystroke that arrived while a replace was in progress.
// It is suppressed at the tap and re-injected after the replace completes, so
// fast typing during a synthetic backspace+retype no longer interleaves.
type replayKey struct {
	keycode uint16
	char    rune
	flags   int64
}

var (
	replayMu sync.Mutex
	replayQ  []replayKey
)

// captureReplay records a user keystroke suppressed during a replace. Called
// from the event-tap callback (goKeyCallback) when replacing == 1.
func captureReplay(keycode uint16, char rune, flags int64) {
	replayMu.Lock()
	replayQ = append(replayQ, replayKey{keycode, char, flags})
	replayMu.Unlock()
}

// resetReplay drops any pending replay items (called at the start of a replace).
func resetReplay() {
	replayMu.Lock()
	replayQ = replayQ[:0]
	replayMu.Unlock()
}

// flushReplay re-injects every captured keystroke as a normal (untagged)
// physical event, in order. Must be called AFTER replacing is back to 0 so the
// re-injected events take the normal path instead of being captured again.
func flushReplay() {
	replayMu.Lock()
	pending := replayQ
	replayQ = nil
	replayMu.Unlock()
	for _, k := range pending {
		// Only override the produced character for printable keys. For control
		// keys (backspace, arrows, …) leave it 0 so the keycode acts normally
		// instead of inserting a literal control char.
		ch := k.char
		if ch < 0x20 || ch == 0x7f {
			ch = 0
		}
		C.postPhysicalReplay(C.uint16_t(k.keycode), C.uint64_t(k.flags), C.UniChar(ch))
		time.Sleep(3 * time.Millisecond)
	}
}

// finishReplacing ends a replace: clear the in-progress flag first (so replayed
// events are not re-captured), then re-inject anything the user typed meanwhile.
func finishReplacing() {
	atomic.StoreInt32(&replacing, 0)
	flushReplay()
}

// ruLayout caches whether the active input source is Russian. The TIS query
// behind it is main-thread-only (see switchLayout above), but IsRussianLayout
// is called from the event-tap thread on every word boundary — so background
// paths read this cache and only the main thread refreshes it.
var ruLayout int32

// IsRussianLayout returns true if macOS is currently on Russian input source.
// Safe from any thread — reads the cache maintained by refreshLayoutCache.
func IsRussianLayout() bool {
	return atomic.LoadInt32(&ruLayout) == 1
}

// refreshLayoutCache re-queries TIS for the active input source. Must run on
// the main thread: called at startup and from the layout-change observer.
func refreshLayoutCache() {
	if C.isCurrentLayoutRussian() == 1 {
		atomic.StoreInt32(&ruLayout, 1)
	} else {
		atomic.StoreInt32(&ruLayout, 0)
	}
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

// maybeSwitchLayout moves the system input source to match text's majority
// script, but only when switch-mode is enabled (Config.SwitchLayout). In the
// default neutral mode it is a no-op, so Bzz just rewrites the text in place.
func maybeSwitchLayout(text string) {
	if atomic.LoadInt32(&switchLayoutEnabled) == 0 {
		return
	}
	latin, cyr := 0, 0
	for _, r := range text {
		switch {
		case r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё':
			cyr++
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			latin++
		}
	}
	if cyr == 0 && latin == 0 {
		return
	}
	selectLayout(cyr >= latin)
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

// axSelectedText reads the current selection via the Accessibility API. Returns
// "" when nothing is selected, or when the focused app doesn't expose selection
// over AX (e.g. some Electron apps). The empty result is a safe no-op signal —
// the f18 handler then does nothing instead of grabbing the whole line via Cmd+C.
func axSelectedText() string {
	cstr := C.axSelectedText()
	if cstr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
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
	resetReplay()
	buf.Clear()

	// Run the backspace+retype asynchronously so the event-tap callback that
	// triggered us returns immediately and stays responsive. While replacing == 1
	// the callback suppresses and queues the user's keystrokes (captureReplay);
	// finishReplacing re-injects them afterwards. If this ran synchronously the
	// tap would block for the whole ~150ms retype and the user's fast keystrokes
	// would interleave with our synthetic ones in the HID queue — the garbled
	// VSCode typing we set out to fix.
	go func() {
		// Let the triggering boundary char (the space/punct that fired this fix)
		// finish passing through to the app before we start backspacing, so our
		// delete count still lines up with what is on screen.
		time.Sleep(15 * time.Millisecond)
		vlog("REPLACE START: delete=%d text=%q", deleteChars, newText)

		for i := 0; i < deleteChars; i++ {
			C.sendBackspace()
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)

		for _, ch := range newText {
			C.sendUnichar(C.UniChar(ch))
			time.Sleep(5 * time.Millisecond)
		}
		vlog("REPLACE TYPED: %q", newText)

		// By default stay layout-neutral (Punto-style): keep the user in their
		// single typing layout and just fix each wrong-layout word in place.
		// In switch-mode (Config.SwitchLayout) also move the system input source
		// to match, so continued typing comes out in the corrected script.
		maybeSwitchLayout(newText)
		time.Sleep(10 * time.Millisecond)

		finishReplacing()
		vlog("REPLACE DONE")
	}()
}
