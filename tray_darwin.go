//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon

#include <stdlib.h>

void updateTrayLayout(int enabled, int russian);
void applyMenuState(int switchOn, int contextOn, const char* excludeTitle);
void removeTray(void);
void ensureApp(void);
void runNSApp(void);
void installLayoutObserver(void);
*/
import "C"

import (
	"log"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"
)

var trayEnabled int32 = 1

//export goTrayToggle
func goTrayToggle() {
	if atomic.LoadInt32(&trayEnabled) == 1 {
		atomic.StoreInt32(&trayEnabled, 0)
		log.Println("Bzz paused")
	} else {
		atomic.StoreInt32(&trayEnabled, 1)
		log.Println("Bzz resumed")
	}
	refreshTrayIcon()
}

//export goTrayQuit
func goTrayQuit() {
	log.Println("Quit from tray")
	C.removeTray()
	os.Exit(0)
}

//export goToggleSwitchLayout
func goToggleSwitchLayout() {
	cfgMu.Lock()
	if activeCfg != nil {
		activeCfg.SwitchLayout = !activeCfg.SwitchLayout
		if activeCfg.SwitchLayout {
			atomic.StoreInt32(&switchLayoutEnabled, 1)
		} else {
			atomic.StoreInt32(&switchLayoutEnabled, 0)
		}
		if err := SaveConfig(activeCfg); err != nil {
			log.Printf("save config: %v", err)
		}
		log.Printf("Switch-layout mode: %v", activeCfg.SwitchLayout)
	}
	cfgMu.Unlock()
	goMenuWillOpen()
}

//export goToggleContext
func goToggleContext() {
	cfgMu.Lock()
	if activeCfg != nil {
		activeCfg.ContextAware = !activeCfg.ContextAware
		if activeDetector != nil {
			activeDetector.contextAware = activeCfg.ContextAware
		}
		if err := SaveConfig(activeCfg); err != nil {
			log.Printf("save config: %v", err)
		}
		log.Printf("Context-aware: %v", activeCfg.ContextAware)
	}
	cfgMu.Unlock()
	goMenuWillOpen()
}

//export goExcludeApp
func goExcludeApp() {
	app := pendingExclude
	if app == "" || activeCfg == nil {
		return
	}
	cfgMu.Lock()
	// Toggle exact-bundle-id membership. Substring rules (e.g. "idea") are left
	// to the config file; this only manages the full id we add from the menu.
	idx := -1
	for i, ex := range activeCfg.ExcludedApps {
		if strings.EqualFold(ex, app) {
			idx = i
			break
		}
	}
	if idx >= 0 {
		activeCfg.ExcludedApps = append(activeCfg.ExcludedApps[:idx], activeCfg.ExcludedApps[idx+1:]...)
		log.Printf("Un-excluded app: %s", app)
	} else {
		activeCfg.ExcludedApps = append(activeCfg.ExcludedApps, app)
		log.Printf("Excluded app: %s", app)
	}
	if err := SaveConfig(activeCfg); err != nil {
		log.Printf("save config: %v", err)
	}
	cfgMu.Unlock()
	goMenuWillOpen()
}

//export goMenuWillOpen
func goMenuWillOpen() {
	sw, ctx := false, false
	cfgMu.Lock()
	if activeCfg != nil {
		sw = activeCfg.SwitchLayout
		ctx = activeCfg.ContextAware
	}
	cfgMu.Unlock()

	app := FrontmostAppID()
	pendingExclude = app

	title := "Исключить приложение"
	if app != "" {
		short := app
		if i := strings.LastIndex(app, "."); i >= 0 && i+1 < len(app) {
			short = app[i+1:]
		}
		if activeCfg != nil && activeCfg.IsAppExcluded(app) {
			title = "✓ Не исправлять: " + short
		} else {
			title = "Не исправлять: " + short
		}
	}
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.applyMenuState(boolToCInt(sw), boolToCInt(ctx), cTitle)
}

//export goLayoutChanged
func goLayoutChanged() {
	refreshTrayIcon()
}

func boolToCInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// refreshTrayIcon repaints the menu-bar glyph from the current enabled + layout state.
func refreshTrayIcon() {
	enabled := 0
	if isTrayEnabled() {
		enabled = 1
	}
	russian := 0
	if IsRussianLayout() {
		russian = 1
	}
	C.updateTrayLayout(C.int(enabled), C.int(russian))
}

func startTray() {
	C.ensureApp()
}

// installLayoutObserver registers a system keyboard-layout-change observer so the
// tray flag repaints live. Call after the NSApp run loop is up.
func installLayoutObserver() {
	C.installLayoutObserver()
	refreshTrayIcon()
}

// runAppLoop runs NSApp run loop — blocks forever, must be called from main goroutine
func runAppLoop() {
	C.runNSApp()
}

func isTrayEnabled() bool {
	return atomic.LoadInt32(&trayEnabled) == 1
}
