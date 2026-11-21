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

@interface TrayMenuPanel : NSPanel
@end
@implementation TrayMenuPanel
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return NO; }
@end

@interface TrayMenuRow : NSButton
@property(nonatomic, strong) NSMenuItem *item;
@property(nonatomic) BOOL selected;
@end
@implementation TrayMenuRow
- (BOOL)allowsVibrancy { return NO; }
- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    for (NSTrackingArea *area in self.trackingAreas) [self removeTrackingArea:area];
    [self addTrackingArea:[[NSTrackingArea alloc] initWithRect:NSZeroRect
        options:NSTrackingActiveAlways | NSTrackingInVisibleRect | NSTrackingMouseEnteredAndExited
        owner:self userInfo:nil]];
}
- (void)mouseEntered:(NSEvent *)event {
    for (NSView *view in self.superview.subviews) if ([view isKindOfClass:TrayMenuRow.class]) {
        ((TrayMenuRow *)view).selected = NO; view.needsDisplay = YES;
    }
    self.selected = self.enabled; self.needsDisplay = YES;
}
- (void)mouseExited:(NSEvent *)event { self.selected = NO; self.needsDisplay = YES; }
- (void)drawRect:(NSRect)dirty {
    BOOL active = self.selected || self.isHighlighted;
    if (active && self.enabled) {
        [NSColor.controlAccentColor setFill];
        [[NSBezierPath bezierPathWithRoundedRect:self.bounds xRadius:6 yRadius:6] fill];
    }
    NSColor *color = !self.enabled ? NSColor.secondaryLabelColor :
        active ? NSColor.whiteColor : NSColor.labelColor;
    NSDictionary *attrs = @{NSFontAttributeName:[NSFont menuFontOfSize:13], NSForegroundColorAttributeName:color};
    CGFloat y = (self.bounds.size.height - [self.item.title sizeWithAttributes:attrs].height) / 2;
    [self.item.title drawAtPoint:NSMakePoint(24, y) withAttributes:attrs];
    if (self.item.state == NSControlStateValueOn) [@"✓" drawAtPoint:NSMakePoint(6,y) withAttributes:attrs];
    if ([self.item.keyEquivalent isEqualToString:@"q"]) {
        NSString *hint = @"⌘Q";
        [hint drawAtPoint:NSMakePoint(self.bounds.size.width - [hint sizeWithAttributes:attrs].width - 8,y) withAttributes:attrs];
    }
}
@end

@interface TrayDelegate : NSObject <NSStatusItemExpandedInterfaceDelegate>
@property(nonatomic, strong) TrayMenuPanel *panel;
@property(nonatomic, strong) NSArray<TrayMenuRow *> *rows;
@property(nonatomic, strong) id outsideLocal;
@property(nonatomic, strong) id outsideGlobal;
@property(nonatomic, strong) id panelKeys;
- (void)closePanel;
- (void)rowAction:(TrayMenuRow *)row;
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
    [self closePanel];
}
- (void)showMenu:(id)sender {
    if (menuTracking) { [self closePanel]; return; }
    NSStatusBarButton *button = statusItem.button;
    if (!button.window.isVisible) return;
    goMenuWillOpen();
    menuTracking = YES;
    NSScreen *screen = button.window.screen;
    if (!screen) { [self closePanel]; return; }
    CGFloat width = 280, height = 16;
    for (NSMenuItem *item in statusMenu.itemArray) {
        if (item.hidden) continue;
        height += item.isSeparatorItem ? 10 : 26;
        width = MAX(width, [item.title sizeWithAttributes:@{NSFontAttributeName:[NSFont menuFontOfSize:13]}].width + 76);
    }
    NSRect anchor = [button.window convertRectToScreen:[button convertRect:button.bounds toView:nil]];
    // Same visible-card placement as AdSilencio's card_origin: centered under
    // the anchor, 7pt below, clamped to the screen containing that anchor.
    NSRect bounds = screen.frame;
    width = MIN(width, bounds.size.width);
    CGFloat x = MIN(MAX(NSMidX(anchor)-width/2, NSMinX(bounds)), NSMaxX(bounds)-width);
    CGFloat top = NSMinY(anchor) - 7;
    CGFloat y = MAX(NSMinY(bounds), top-height);
    self.panel = [[TrayMenuPanel alloc] initWithContentRect:NSMakeRect(x,y,width,height)
        styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
        backing:NSBackingStoreBuffered defer:NO];
    self.panel.releasedWhenClosed = NO;
    self.panel.opaque = NO;
    self.panel.backgroundColor = NSColor.clearColor;
    self.panel.hasShadow = YES;
    self.panel.hidesOnDeactivate = NO;
    self.panel.level = NSPopUpMenuWindowLevel;
    self.panel.collectionBehavior = NSWindowCollectionBehaviorTransient | NSWindowCollectionBehaviorFullScreenAuxiliary;
    self.panel.title = @"Меню Bzz";
    NSVisualEffectView *body = [[NSVisualEffectView alloc] initWithFrame:NSMakeRect(0,0,width,height)];
    body.material = NSVisualEffectMaterialMenu;
    body.state = NSVisualEffectStateActive;
    body.wantsLayer = YES;
    body.layer.cornerRadius = 10;
    body.layer.masksToBounds = YES;
    self.panel.contentView = body;
    NSMutableArray *rows = [NSMutableArray array];
    CGFloat offset = height-8;
    for (NSMenuItem *item in statusMenu.itemArray) {
        if (item.hidden) continue;
        if (item.isSeparatorItem) {
            offset -= 10;
            NSBox *line = [[NSBox alloc] initWithFrame:NSMakeRect(26,offset+4,width-40,1)];
            line.boxType = NSBoxSeparator;
            [body addSubview:line];
            continue;
        }
        offset -= 26;
        TrayMenuRow *row = [[TrayMenuRow alloc] initWithFrame:NSMakeRect(5,offset,width-10,26)];
        row.item = item;
        row.title = item.title;
        row.bordered = NO;
        row.enabled = item.enabled && item.action != NULL;
        row.target = self;
        row.action = @selector(rowAction:);
        [row setAccessibilityLabel:item.title];
        if (item == switchItem || item == contextItem || item == spellItem || item == excludeItem) {
            [row setAccessibilityRole:NSAccessibilityCheckBoxRole];
            [row setAccessibilityValue:@(item.state == NSControlStateValueOn)];
        }
        [body addSubview:row];
        [rows addObject:row];
    }
    self.rows = rows;
    __weak TrayDelegate *weakSelf = self;
    void (^outside)(void) = ^{
        TrayDelegate *owner = weakSelf;
        if (!owner.panel) return;
        NSStatusBarButton *b = statusItem.button;
        NSRect icon = [b.window convertRectToScreen:[b convertRect:b.bounds toView:nil]];
        NSPoint p = NSEvent.mouseLocation;
        if (!NSPointInRect(p,owner.panel.frame) && !NSPointInRect(p,icon)) [owner closePanel];
    };
    NSEventMask clicks = NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown;
    self.outsideLocal = [NSEvent addLocalMonitorForEventsMatchingMask:clicks handler:^NSEvent *(NSEvent *event) { outside(); return event; }];
    self.outsideGlobal = [NSEvent addGlobalMonitorForEventsMatchingMask:clicks handler:^(NSEvent *event) { outside(); }];
    self.panelKeys = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown handler:^NSEvent *(NSEvent *event) {
        TrayDelegate *owner = weakSelf;
        if (!owner.panel) return event;
        if (event.keyCode == 53) { [owner closePanel]; return nil; }
        NSEventModifierFlags modifiers = event.modifierFlags & (NSEventModifierFlagCommand |
            NSEventModifierFlagControl | NSEventModifierFlagOption | NSEventModifierFlagShift);
        if (modifiers == NSEventModifierFlagCommand && [event.charactersIgnoringModifiers.lowercaseString isEqualToString:@"q"]) {
            [owner closePanel]; goTrayQuit(); return nil;
        }
        NSArray *enabled = [owner.rows filteredArrayUsingPredicate:[NSPredicate predicateWithBlock:^BOOL(TrayMenuRow *row, NSDictionary *bindings) { return row.enabled; }]];
        NSInteger index = -1;
        for (NSUInteger i=0;i<enabled.count;i++) if (((TrayMenuRow *)enabled[i]).selected) index=i;
        if (enabled.count && (event.keyCode == 125 || event.keyCode == 126)) {
            NSInteger next = index < 0 ? (event.keyCode == 125 ? 0 : enabled.count-1) :
                (index + (event.keyCode == 125 ? 1 : -1) + enabled.count) % enabled.count;
            for (TrayMenuRow *row in owner.rows) { row.selected = (row == enabled[next]); row.needsDisplay = YES; }
            return nil;
        }
        if (index >= 0 && (event.keyCode == 36 || event.keyCode == 49)) { [owner rowAction:enabled[index]]; return nil; }
        return event;
    }];
    [self.panel makeKeyAndOrderFront:nil];
}
- (void)closePanel {
    menuTracking = NO;
    [self.panel orderOut:nil];
    self.panel = nil;
    self.rows = nil;
    for (id monitor in @[self.outsideLocal ?: NSNull.null, self.outsideGlobal ?: NSNull.null, self.panelKeys ?: NSNull.null])
        if (monitor != NSNull.null) [NSEvent removeMonitor:monitor];
    self.outsideLocal = self.outsideGlobal = self.panelKeys = nil;
    if (@available(macOS 27.0, *)) [statusItem.expandedInterfaceSession cancel];
}
- (void)rowAction:(TrayMenuRow *)row {
    NSMenuItem *item = row.item;
    if (!row.enabled) return;
    [self closePanel];
    [NSApp sendAction:item.action to:item.target from:item];
}
- (void)toggleAction:(id)sender { goTrayToggle(); }
- (void)quitAction:(id)sender { goTrayQuit(); }
- (void)switchLayoutAction:(id)sender { goToggleSwitchLayout(); }
- (void)contextAction:(id)sender { goToggleContext(); }
- (void)spellAction:(id)sender { goToggleSpellcheck(); }
- (void)excludeAction:(id)sender { goExcludeApp(); }
- (void)aboutAction:(id)sender { goShowAbout(); }
@end

static TrayDelegate *delegate = nil;

// buildMenu constructs the status menu with the settings submenu items. Called
// once from ensureApp on the main thread.
static void buildMenu(void) {
    statusMenu = [[NSMenu alloc] init];

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
            [delegate closePanel];
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
