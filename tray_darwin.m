#include <Cocoa/Cocoa.h>
#include <Carbon/Carbon.h>

extern void goTrayToggle();
extern void goTrayQuit();
extern void goToggleSwitchLayout();
extern void goToggleContext();
extern void goExcludeApp();
extern void goMenuWillOpen();
extern void goLayoutChanged();

static NSStatusItem *statusItem = nil;
static NSMenu *statusMenu = nil;
static NSMenuItem *toggleItem = nil;
static NSMenuItem *switchItem = nil;
static NSMenuItem *contextItem = nil;
static NSMenuItem *excludeItem = nil;

@interface TrayDelegate : NSObject <NSMenuDelegate>
- (void)toggleAction:(id)sender;
- (void)quitAction:(id)sender;
- (void)switchLayoutAction:(id)sender;
- (void)contextAction:(id)sender;
- (void)excludeAction:(id)sender;
@end

@implementation TrayDelegate
- (void)toggleAction:(id)sender { goTrayToggle(); }
- (void)quitAction:(id)sender { goTrayQuit(); }
- (void)switchLayoutAction:(id)sender { goToggleSwitchLayout(); }
- (void)contextAction:(id)sender { goToggleContext(); }
- (void)excludeAction:(id)sender { goExcludeApp(); }
// Refresh checkmarks / exclude-app title from Go state just before the menu shows.
- (void)menuWillOpen:(NSMenu *)menu { goMenuWillOpen(); }
@end

static TrayDelegate *delegate = nil;

// buildMenu constructs the status menu with the settings submenu items. Called
// once from ensureApp on the main thread.
static void buildMenu(void) {
    statusMenu = [[NSMenu alloc] init];
    statusMenu.delegate = delegate;

    toggleItem = [[NSMenuItem alloc] initWithTitle:@"⏸ Приостановить"
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

    excludeItem = [[NSMenuItem alloc] initWithTitle:@"Исключить приложение"
                                             action:@selector(excludeAction:)
                                      keyEquivalent:@""];
    excludeItem.target = delegate;
    [statusMenu addItem:excludeItem];

    [statusMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Выйти"
                                                     action:@selector(quitAction:)
                                              keyEquivalent:@"q"];
    quitItem.target = delegate;
    [statusMenu addItem:quitItem];

    statusItem.menu = statusMenu;
}

// updateTrayLayout sets the menu-bar glyph: a flag for the active layout, or the
// sleep glyph when paused.
void updateTrayLayout(int enabled, int russian) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!statusItem) return;
        if (!enabled) {
            statusItem.button.title = @"💤";
        } else {
            statusItem.button.title = russian ? @"🇷🇺" : @"🇬🇧";
        }
        if (toggleItem) {
            toggleItem.title = enabled ? @"⏸ Приостановить" : @"▶ Включить";
        }
    });
}

// applyMenuState updates the settings checkmarks and the exclude-app title.
void applyMenuState(int switchOn, int contextOn, const char *excludeTitle) {
    NSString *title = excludeTitle ? [NSString stringWithUTF8String:excludeTitle] : @"Исключить приложение";
    dispatch_async(dispatch_get_main_queue(), ^{
        if (switchItem)  switchItem.state  = switchOn  ? NSControlStateValueOn : NSControlStateValueOff;
        if (contextItem) contextItem.state = contextOn ? NSControlStateValueOn : NSControlStateValueOff;
        if (excludeItem) excludeItem.title = title;
    });
}

void removeTray(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
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
