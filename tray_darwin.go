//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

void createTray(int enabled);
void updateTray(int enabled);
void removeTray(void);
void ensureApp(void);
void runNSApp(void);
*/
import "C"

import (
	"log"
	"os"
	"sync/atomic"
)

var trayEnabled int32 = 1

//export goTrayToggle
func goTrayToggle() {
	if atomic.LoadInt32(&trayEnabled) == 1 {
		atomic.StoreInt32(&trayEnabled, 0)
		C.updateTray(0)
		log.Println("Bzz paused")
	} else {
		atomic.StoreInt32(&trayEnabled, 1)
		C.updateTray(1)
		log.Println("Bzz resumed")
	}
}

//export goTrayQuit
func goTrayQuit() {
	log.Println("Quit from tray")
	C.removeTray()
	os.Exit(0)
}

func startTray() {
	C.ensureApp()
}

// runAppLoop runs NSApp run loop — blocks forever, must be called from main goroutine
func runAppLoop() {
	C.runNSApp()
}

func isTrayEnabled() bool {
	return atomic.LoadInt32(&trayEnabled) == 1
}
