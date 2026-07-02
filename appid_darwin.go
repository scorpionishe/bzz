//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <stdatomic.h>

// Cached frontmost bundle ID — updated via NSWorkspace notifications on main thread.
// We use a C atomic pointer so readers from any thread get a consistent value.
static _Atomic(const char*) cachedBundleID = NULL;

static void updateCached(NSString* bid) {
    const char* newStr = NULL;
    if (bid != nil) {
        newStr = strdup([bid UTF8String]);
    }
    const char* old = atomic_exchange(&cachedBundleID, newStr);
    if (old != NULL) {
        free((void*)old);
    }
}

// Called on main thread from NSWorkspace notification. Ignores activations of our
// own bundle so FrontmostAppID() keeps reporting the user's real app (matters for
// the tray "exclude current app" action, and harmless otherwise).
static void onAppActivated(NSNotification* note) {
    NSRunningApplication* app = note.userInfo[NSWorkspaceApplicationKey];
    NSString* bid = app ? app.bundleIdentifier : nil;
    NSString* self = [[NSBundle mainBundle] bundleIdentifier];
    if (bid != nil && self != nil && [bid isEqualToString:self]) {
        return;
    }
    updateCached(bid);
}

// Registers NSWorkspace observer and seeds initial value. Call on main thread.
static void bzzInstallFrontmostObserver(void) {
    @autoreleasepool {
        // Seed initial value
        NSRunningApplication* app = [[NSWorkspace sharedWorkspace] frontmostApplication];
        updateCached(app ? app.bundleIdentifier : nil);

        // Register for activation notifications
        [[[NSWorkspace sharedWorkspace] notificationCenter]
            addObserverForName:NSWorkspaceDidActivateApplicationNotification
                        object:nil
                         queue:[NSOperationQueue mainQueue]
                    usingBlock:^(NSNotification* note) {
                        onAppActivated(note);
                    }];
    }
}

// Thread-safe read: returns cached bundle ID (callers must strdup if they need to persist).
static const char* bzzFrontmostBundleID(void) {
    const char* s = atomic_load(&cachedBundleID);
    if (s == NULL) return "";
    return s;
}
*/
import "C"

// installFrontmostObserver registers NSWorkspace observer on main thread.
// Must be called AFTER NSApplication is initialized (i.e. after ensureApp()).
func installFrontmostObserver() {
	C.bzzInstallFrontmostObserver()
}

// FrontmostAppID returns cached bundle identifier of the frontmost app.
// Safe to call from any thread. Returns "" if no app is frontmost or observer not installed.
func FrontmostAppID() string {
	return C.GoString(C.bzzFrontmostBundleID())
}
