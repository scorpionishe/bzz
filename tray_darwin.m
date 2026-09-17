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

@interface TrayDelegate : NSObject <NSMenuDelegate>
- (void)toggleAction:(id)sender;
- (void)quitAction:(id)sender;
- (void)switchLayoutAction:(id)sender;
- (void)contextAction:(id)sender;
- (void)spellAction:(id)sender;
- (void)excludeAction:(id)sender;
- (void)aboutAction:(id)sender;
@end

@implementation TrayDelegate
- (void)toggleAction:(id)sender { goTrayToggle(); }
- (void)quitAction:(id)sender { goTrayQuit(); }
- (void)switchLayoutAction:(id)sender { goToggleSwitchLayout(); }
- (void)contextAction:(id)sender { goToggleContext(); }
- (void)spellAction:(id)sender { goToggleSpellcheck(); }
- (void)excludeAction:(id)sender { goExcludeApp(); }
- (void)aboutAction:(id)sender { goShowAbout(); }
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

    statusItem.menu = statusMenu;
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
            toggleItem.title = enabled ? @"⏸ Приостановить" : @"▶ Включить";
        }
    });
}

// applyMenuState updates the settings checkmarks and the exclude-app title.
void applyMenuState(int switchOn, int contextOn, int spellOn, const char *excludeTitle, const char *statsTitle) {
    NSString *title = excludeTitle ? [NSString stringWithUTF8String:excludeTitle] : @"Исключить приложение";
    NSString *stats = (statsTitle && statsTitle[0]) ? [NSString stringWithUTF8String:statsTitle] : nil;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (switchItem)  switchItem.state  = switchOn  ? NSControlStateValueOn : NSControlStateValueOff;
        if (contextItem) contextItem.state = contextOn ? NSControlStateValueOn : NSControlStateValueOff;
        if (spellItem)   spellItem.state   = spellOn   ? NSControlStateValueOn : NSControlStateValueOff;
        if (excludeItem) excludeItem.title = title;
        if (statsItem) {
            statsItem.title = stats ? [@"⏱ " stringByAppendingString:stats] : @"";
            statsItem.hidden = (stats == nil);
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
