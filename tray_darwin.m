#include <Cocoa/Cocoa.h>

extern void goTrayToggle();
extern void goTrayQuit();

static NSStatusItem *statusItem = nil;
static NSMenu *statusMenu = nil;

@interface TrayDelegate : NSObject
- (void)toggleAction:(id)sender;
- (void)quitAction:(id)sender;
@end

@implementation TrayDelegate
- (void)toggleAction:(id)sender {
    goTrayToggle();
}
- (void)quitAction:(id)sender {
    goTrayQuit();
}
@end

static TrayDelegate *delegate = nil;

void createTray(int enabled) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!delegate) {
            delegate = [[TrayDelegate alloc] init];
        }

        statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
        statusItem.button.title = enabled ? @"⌨ RU" : @"⌨ ⏸";
        statusItem.button.font = [NSFont monospacedSystemFontOfSize:12 weight:NSFontWeightMedium];

        statusMenu = [[NSMenu alloc] init];

        NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:(enabled ? @"⏸ Приостановить" : @"▶ Включить")
                                                           action:@selector(toggleAction:)
                                                    keyEquivalent:@""];
        toggleItem.target = delegate;
        [statusMenu addItem:toggleItem];

        [statusMenu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Выйти"
                                                         action:@selector(quitAction:)
                                                  keyEquivalent:@"q"];
        quitItem.target = delegate;
        [statusMenu addItem:quitItem];

        statusItem.menu = statusMenu;
    });
}

void updateTray(int enabled) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
            statusItem.button.title = enabled ? @"⚡" : @"💤";
        }
        if (statusMenu && [statusMenu numberOfItems] > 0) {
            NSMenuItem *toggle = [statusMenu itemAtIndex:0];
            toggle.title = enabled ? @"⏸ Приостановить" : @"▶ Включить";
        }
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

    // Create tray synchronously on main thread
    if (!delegate) {
        delegate = [[TrayDelegate alloc] init];
    }

    statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    statusItem.button.title = @"⚡";  // active: vzhuh!
    statusItem.button.font = [NSFont systemFontOfSize:14];

    statusMenu = [[NSMenu alloc] init];

    NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:@"⏸ Приостановить"
                                                       action:@selector(toggleAction:)
                                                keyEquivalent:@""];
    toggleItem.target = delegate;
    [statusMenu addItem:toggleItem];

    [statusMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Выйти"
                                                     action:@selector(quitAction:)
                                              keyEquivalent:@"q"];
    quitItem.target = delegate;
    [statusMenu addItem:quitItem];

    // Standard menu on any click
    statusItem.menu = statusMenu;
}

void runNSApp(void) {
    // Use [NSApp run] — the standard way. This processes menu events correctly.
    // CGEventTap runs on its own thread via startHook(), so no conflict.
    [NSApp run];
}
