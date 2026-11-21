#include <Cocoa/Cocoa.h>
#include <Carbon/Carbon.h>

extern void goTrayToggle();
extern void goTrayQuit();
extern void goToggleSwitchLayout();
extern void goToggleContext();
extern void goToggleSpellcheck();
extern void goExcludeApp();
extern void goShowAbout();
extern void goMenuWillOpen();
extern void goLayoutChanged();

static NSStatusItem *statusItem = nil;
static NSMenu *statusMenu = nil;
static NSMenuItem *toggleItem = nil;
static NSMenuItem *switchItem = nil;
static NSMenuItem *contextItem = nil;
static NSMenuItem *spellItem = nil;
static NSMenuItem *excludeItem = nil;
static NSMenuItem *statsItem = nil;   // "time saved" line, disabled (display only)
static BOOL menuTracking = NO;

// WindowServer highlights a native status item before NSStatusBarButton gets
// mouseDown. Keep a separate, non-key input panel over the button at all times.
// The button itself remains the keyboard/Accessibility entry point.
@class TrayPointerArea;
@interface TrayPointerTarget : NSView
@property(nonatomic, weak) TrayPointerArea *owner;
@end

@interface TrayInputPanel : NSPanel
@end
@implementation TrayInputPanel
- (BOOL)canBecomeKeyWindow { return NO; }
- (BOOL)canBecomeMainWindow { return NO; }
@end

@interface TrayPointerArea : NSView {
    TrayInputPanel *surface;
    NSTimer *modifierTimer;
    id localModifiers;
    id globalModifiers;
}
@property(nonatomic, weak) id target;
@property(nonatomic) SEL action;
- (void)positionSurface;
- (void)updateModifiers;
- (void)watchModifiers;
- (void)stop;
- (void)press;
@end

@implementation TrayPointerArea
- (NSView *)hitTest:(NSPoint)point { return nil; }
- (void)stopWatchingModifiers {
    [modifierTimer invalidate];
    modifierTimer = nil;
}
- (void)stop {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    [self stopWatchingModifiers];
    if (localModifiers) [NSEvent removeMonitor:localModifiers];
    if (globalModifiers) [NSEvent removeMonitor:globalModifiers];
    localModifiers = globalModifiers = nil;
    if (surface) {
        [surface.parentWindow removeChildWindow:surface];
        [surface orderOut:nil];
        surface = nil;
    }
}
- (void)viewWillMoveToWindow:(NSWindow *)window {
    [self stop];
    [super viewWillMoveToWindow:window];
}
- (void)viewDidMoveToWindow {
    [super viewDidMoveToWindow];
    if (!self.window) return;
    for (NSNotificationName name in @[NSWindowDidMoveNotification, NSWindowDidResizeNotification,
                                      NSWindowDidChangeScreenNotification, NSWindowDidChangeOcclusionStateNotification]) {
        [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(positionSurface)
                                                     name:name object:self.window];
    }
    __weak TrayPointerArea *weakSelf = self;
    localModifiers = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskFlagsChanged handler:^NSEvent *(NSEvent *event) {
        [weakSelf updateModifiers];
        return event;
    }];
    globalModifiers = [NSEvent addGlobalMonitorForEventsMatchingMask:NSEventMaskFlagsChanged handler:^(NSEvent *event) {
        [weakSelf updateModifiers];
    }];
    [self positionSurface];
}
- (void)layout {
    [super layout];
    [self positionSurface];
}
- (void)positionSurface {
    NSWindow *anchor = self.window;
    NSRect rect = anchor ? [anchor convertRectToScreen:[self convertRect:self.bounds toView:nil]] : NSZeroRect;
    if (!anchor.isVisible || !anchor.screen || self.isHiddenOrHasHiddenAncestor ||
        NSIsEmptyRect(rect) || !NSContainsRect(anchor.screen.frame, rect) || ![NSMenu menuBarVisible]) {
        [surface orderOut:nil];
        [self stopWatchingModifiers];
        return;
    }
    if (!surface) {
        surface = [[TrayInputPanel alloc] initWithContentRect:rect
            styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
            backing:NSBackingStoreBuffered defer:NO];
        surface.opaque = NO;
        surface.hasShadow = NO;
        // A fully transparent window lets WindowServer send clicks underneath.
        surface.backgroundColor = [[NSColor whiteColor] colorWithAlphaComponent:0.002];
        surface.level = NSStatusWindowLevel + 1;
        surface.hidesOnDeactivate = NO;
        surface.excludedFromWindowsMenu = YES;
        surface.collectionBehavior = NSWindowCollectionBehaviorTransient |
            NSWindowCollectionBehaviorFullScreenAuxiliary | NSWindowCollectionBehaviorIgnoresCycle;
        [surface setAccessibilityElement:NO];
        TrayPointerTarget *input = [[TrayPointerTarget alloc] initWithFrame:NSMakeRect(0, 0, rect.size.width, rect.size.height)];
        input.owner = self;
        [input setAccessibilityElement:NO];
        surface.contentView = input;
        [anchor addChildWindow:surface ordered:NSWindowAbove];
    }
    if (!NSEqualRects(surface.frame, rect)) [surface setFrame:rect display:NO];
    if (!surface.isVisible) [surface orderFrontRegardless];
    [self updateModifiers];
    if (NSPointInRect(NSEvent.mouseLocation, rect)) [self watchModifiers];
}
- (void)watchModifiers {
    if (!modifierTimer) {
        modifierTimer = [NSTimer timerWithTimeInterval:0.035 target:self selector:@selector(updateModifiers)
                                             userInfo:nil repeats:YES];
        [[NSRunLoop mainRunLoop] addTimer:modifierTimer forMode:NSRunLoopCommonModes];
    }
    [self updateModifiers];
}
- (void)updateModifiers {
    // Keep the bypass while dragging outside the original bounds. Restoring
    // input on mouseExited would intercept the native drag before it can drop.
    surface.ignoresMouseEvents = (NSEvent.modifierFlags &
        (NSEventModifierFlagCommand | NSEventModifierFlagControl | NSEventModifierFlagOption | NSEventModifierFlagShift)) != 0;
    if (!surface.isVisible || !NSPointInRect(NSEvent.mouseLocation, surface.frame)) {
        [self stopWatchingModifiers];
        return;
    }
}
- (void)press { [NSApp sendAction:self.action to:self.target from:self]; }
- (void)dealloc {
    [self stop];
}
@end

@implementation TrayPointerTarget {
    NSTrackingArea *tracking;
    BOOL pressed;
}
- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    if (tracking) [self removeTrackingArea:tracking];
    tracking = [[NSTrackingArea alloc] initWithRect:NSZeroRect
        options:NSTrackingActiveAlways | NSTrackingInVisibleRect | NSTrackingMouseEnteredAndExited owner:self userInfo:nil];
    [self addTrackingArea:tracking];
}
- (void)mouseEntered:(NSEvent *)event { [self.owner watchModifiers]; }
- (void)mouseExited:(NSEvent *)event { [self.owner updateModifiers]; }
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
- (void)mouseDown:(NSEvent *)event { pressed = YES; }
- (void)mouseUp:(NSEvent *)event {
    BOOL activate = pressed && NSPointInRect([self convertPoint:event.locationInWindow fromView:nil], self.bounds);
    pressed = NO;
    if (activate) [self.owner press];
}
@end

static TrayPointerArea *pointerArea = nil;

@interface TrayDelegate : NSObject <NSMenuDelegate, NSStatusItemExpandedInterfaceDelegate>
@property(nonatomic, strong) NSRunningApplication *menuOwnerApp;
- (void)statusItem:(NSStatusItem *)item didBeginExpandedInterfaceSession:(NSStatusItemExpandedInterfaceSession *)session API_AVAILABLE(macos(27.0));
- (void)statusItemDidEndExpandedInterfaceSession:(NSStatusItem *)item animated:(BOOL)animated API_AVAILABLE(macos(27.0));
- (void)showMenu:(id)sender;
- (void)toggleAction:(id)sender;
- (void)quitAction:(id)sender;
- (void)switchLayoutAction:(id)sender;
- (void)contextAction:(id)sender;
- (void)spellAction:(id)sender;
- (void)excludeAction:(id)sender;
- (void)aboutAction:(id)sender;
@end

@implementation TrayDelegate
- (void)statusItem:(NSStatusItem *)item didBeginExpandedInterfaceSession:(NSStatusItemExpandedInterfaceSession *)session {
    [self showMenu:item];
}
- (void)statusItemDidEndExpandedInterfaceSession:(NSStatusItem *)item animated:(BOOL)animated {
    if (menuTracking) [statusMenu cancelTracking];
}
- (void)showMenu:(id)sender {
    if (menuTracking) { [statusMenu cancelTracking]; return; }
    NSStatusBarButton *button = statusItem.button;
    if (!button.window.isVisible) return;
    self.menuOwnerApp = NSWorkspace.sharedWorkspace.frontmostApplication;
    goMenuWillOpen();
    menuTracking = YES;
    [NSApp activateIgnoringOtherApps:YES];
    // Keep Bzz's existing NSMenu, including its native layout and keyboard
    // tracking. Do not attach it to the status item or anchor it to the button:
    // either would ask macOS to highlight that button again.
    dispatch_async(dispatch_get_main_queue(), ^{
        // Activation or a menu-bar manager can move the item before this block.
        NSStatusBarButton *anchor = statusItem.button;
        NSScreen *screen = anchor.window.screen;
        if (!anchor.window.isVisible || !screen) {
            [self menuDidClose:statusMenu];
            return;
        }
        NSRect rect = [anchor.window convertRectToScreen:[anchor convertRect:anchor.bounds toView:nil]];
        // The popup point anchors menu content, not its outer rounded frame.
        // Leave room for native chrome without constraining NSMenu's frame:
        // a custom confinement rect on macOS 27 clips the last item's highlight.
        CGFloat top = MIN(NSMinY(rect), NSMaxY(screen.visibleFrame)) - 16.0;
        [statusMenu popUpMenuPositioningItem:nil atLocation:NSMakePoint(NSMinX(rect), top) inView:nil];
    });
}
- (void)toggleAction:(id)sender { goTrayToggle(); }
- (void)quitAction:(id)sender { goTrayQuit(); }
- (void)switchLayoutAction:(id)sender { goToggleSwitchLayout(); }
- (void)contextAction:(id)sender { goToggleContext(); }
- (void)spellAction:(id)sender { goToggleSpellcheck(); }
- (void)excludeAction:(id)sender { goExcludeApp(); }
- (void)aboutAction:(id)sender { goShowAbout(); }
// Refresh checkmarks / exclude-app title from Go state just before the menu shows.
- (void)menuWillOpen:(NSMenu *)menu {
    if (!menuTracking) goMenuWillOpen();
}
- (void)menuDidClose:(NSMenu *)menu {
    menuTracking = NO;
    if (@available(macOS 27.0, *)) [statusItem.expandedInterfaceSession cancel];
    if (NSApp.isActive && self.menuOwnerApp.processIdentifier != NSProcessInfo.processInfo.processIdentifier) {
        [self.menuOwnerApp activateWithOptions:0];
    }
    self.menuOwnerApp = nil;
}
@end

static TrayDelegate *delegate = nil;

// buildMenu constructs the status menu with the settings submenu items. Called
// once from ensureApp on the main thread.
static void buildMenu(void) {
    statusMenu = [[NSMenu alloc] init];
    statusMenu.delegate = delegate;

    toggleItem = [[NSMenuItem alloc] initWithTitle:@"Приостановить"
                                            action:@selector(toggleAction:)
                                     keyEquivalent:@""];
    toggleItem.target = delegate;
    [statusMenu addItem:toggleItem];

    [statusMenu addItem:[NSMenuItem separatorItem]];

    switchItem = [[NSMenuItem alloc] initWithTitle:@"Менять раскладку"
                                            action:@selector(switchLayoutAction:)
                                     keyEquivalent:@""];
    switchItem.target = delegate;
    [statusMenu addItem:switchItem];

    contextItem = [[NSMenuItem alloc] initWithTitle:@"Учитывать контекст"
                                             action:@selector(contextAction:)
                                      keyEquivalent:@""];
    contextItem.target = delegate;
    [statusMenu addItem:contextItem];

    spellItem = [[NSMenuItem alloc] initWithTitle:@"Проверять орфографию"
                                           action:@selector(spellAction:)
                                    keyEquivalent:@""];
    spellItem.target = delegate;
    [statusMenu addItem:spellItem];

    excludeItem = [[NSMenuItem alloc] initWithTitle:@"Исключить приложение"
                                             action:@selector(excludeAction:)
                                      keyEquivalent:@""];
    excludeItem.target = delegate;
    [statusMenu addItem:excludeItem];

    [statusMenu addItem:[NSMenuItem separatorItem]];

    // Time-saved counter: a display-only line (no action → drawn disabled).
    // Hidden until Go supplies a summary via applyMenuState.
    statsItem = [[NSMenuItem alloc] initWithTitle:@"" action:nil keyEquivalent:@""];
    statsItem.hidden = YES;
    [statusMenu addItem:statsItem];

    NSMenuItem *aboutItem = [[NSMenuItem alloc] initWithTitle:@"О Bzz"
                                                      action:@selector(aboutAction:)
                                               keyEquivalent:@""];
    aboutItem.target = delegate;
    [statusMenu addItem:aboutItem];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Выйти"
                                                     action:@selector(quitAction:)
                                              keyEquivalent:@"q"];
    quitItem.target = delegate;
    [statusMenu addItem:quitItem];

}

// showAboutPanel brings up the standard macOS "About" window (app icon, name,
// version). Version strings come from the bundle's Info.plist automatically.
void showAboutPanel(const char *credits) {
    // The "time saved" line goes into the credits area under the version.
    NSString *creditsText = (credits && credits[0]) ? [NSString stringWithUTF8String:credits] : nil;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMutableDictionary *opts = [NSMutableDictionary dictionary];
        opts[NSAboutPanelOptionApplicationName] = @"bzz";
        if (creditsText) {
            NSMutableParagraphStyle *para = [[NSMutableParagraphStyle alloc] init];
            para.alignment = NSTextAlignmentCenter;
            opts[NSAboutPanelOptionCredits] = [[NSAttributedString alloc]
                initWithString:creditsText
                    attributes:@{NSFontAttributeName: [NSFont systemFontOfSize:[NSFont smallSystemFontSize]],
                                 NSForegroundColorAttributeName: [NSColor secondaryLabelColor],
                                 NSParagraphStyleAttributeName: para}];
        }
        // Hide the parenthetical build number — we don't track one, so the
        // default panel would show "Version 0.5.0 (0.5.0)".
        opts[NSAboutPanelOptionVersion] = @"";
        NSImage *icon = [NSApp applicationIconImage];
        if (icon) {
            opts[NSAboutPanelOptionApplicationIcon] = icon;
        }
        // Accessory apps are not active; bring the panel to the front.
        [NSApp activateIgnoringOtherApps:YES];
        [NSApp orderFrontStandardAboutPanelWithOptions:opts];
    });
}

// flagImage renders an emoji into an NSImage. The flag used to be the button's
// plain title, but on current macOS the status bar draws a plain title in its
// own fixed font — the flag came out tiny and sat below the baseline, and
// button.font was ignored. Drawing it ourselves fixes the size and alignment.
static NSImage *flagImage(NSString *emoji) {
    NSDictionary *attrs = @{NSFontAttributeName: [NSFont systemFontOfSize:15]};
    NSSize size = [emoji sizeWithAttributes:attrs];
    NSImage *img = [[NSImage alloc] initWithSize:size];
    [img lockFocus];
    [emoji drawAtPoint:NSZeroPoint withAttributes:attrs];
    [img unlockFocus];
    return img;
}

// updateTrayLayout sets the menu-bar glyph: a flag for the active layout, or the
// sleep glyph when paused.
void updateTrayLayout(int enabled, int russian) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!statusItem) return;
        if (!enabled) {
            statusItem.button.image = nil;
            statusItem.button.imagePosition = NSNoImage;
            statusItem.button.title = @"💤";
        } else {
            statusItem.button.title = @"";
            statusItem.button.image = flagImage(russian ? @"🇷🇺" : @"🇬🇧");
            statusItem.button.imagePosition = NSImageOnly;
        }
        if (toggleItem) {
            toggleItem.title = enabled ? @"Приостановить" : @"Включить";
        }
    });
}

// applyMenuState updates the settings checkmarks and the exclude-app title.
// Every mark goes through the item's state, never into its title: a glyph in
// the title draws in the text column, so the title slid right of its
// neighbours ("✓ Не исправлять", "⏸ Приостановить", "⏱ с …").
void applyMenuState(int switchOn, int contextOn, int spellOn, int excludeOn, const char *excludeTitle, const char *statsTitle) {
    NSString *title = excludeTitle ? [NSString stringWithUTF8String:excludeTitle] : @"Исключить приложение";
    NSString *stats = (statsTitle && statsTitle[0]) ? [NSString stringWithUTF8String:statsTitle] : nil;
    void (^apply)(void) = ^{
        if (switchItem)  switchItem.state  = switchOn  ? NSControlStateValueOn : NSControlStateValueOff;
        if (contextItem) contextItem.state = contextOn ? NSControlStateValueOn : NSControlStateValueOff;
        if (spellItem)   spellItem.state   = spellOn   ? NSControlStateValueOn : NSControlStateValueOff;
        if (excludeItem) {
            excludeItem.title = title;
            excludeItem.state = excludeOn ? NSControlStateValueOn : NSControlStateValueOff;
        }
        if (statsItem) {
            statsItem.title = stats ?: @"";
            statsItem.hidden = (stats == nil);
        }
    };
    // menuWillOpen calls back from the main thread. Apply its fresh Go state
    // before AppKit starts tracking, including the previously frontmost app.
    if ([NSThread isMainThread]) apply();
    else dispatch_async(dispatch_get_main_queue(), apply);
}

void removeTray(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
            [pointerArea stop];
            [pointerArea removeFromSuperview];
            pointerArea = nil;
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
        }
    });
}

void ensureApp(void) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

    if (!delegate) {
        delegate = [[TrayDelegate alloc] init];
    }

    statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    statusItem.button.title = @"⚡"; // replaced by updateTrayLayout once layout is known
    statusItem.button.font = [NSFont systemFontOfSize:14];

    buildMenu();
    statusItem.button.target = delegate;
    statusItem.button.action = @selector(showMenu:);
    if (@available(macOS 27.0, *)) statusItem.expandedInterfaceDelegate = delegate;
    [statusItem.button sendActionOn:NSEventMaskLeftMouseUp];
    [statusItem.button setAccessibilityLabel:@"Bzz"];
    pointerArea = [[TrayPointerArea alloc] initWithFrame:statusItem.button.bounds];
    pointerArea.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    pointerArea.target = delegate;
    pointerArea.action = @selector(showMenu:);
    [pointerArea setAccessibilityElement:NO];
    [statusItem.button addSubview:pointerArea];
}

// inputSourceChanged fires on any system keyboard-layout switch. It bounces to Go
// (goLayoutChanged) which re-reads the layout + enabled state and repaints the icon.
static void inputSourceChanged(CFNotificationCenterRef center, void *observer,
                               CFStringRef name, const void *object, CFDictionaryRef userInfo) {
    goLayoutChanged();
}

void installLayoutObserver(void) {
    CFNotificationCenterAddObserver(
        CFNotificationCenterGetDistributedCenter(),
        NULL,
        inputSourceChanged,
        kTISNotifySelectedKeyboardInputSourceChanged,
        NULL,
        CFNotificationSuspensionBehaviorDeliverImmediately);
}

void runNSApp(void) {
    [NSApp run];
}
