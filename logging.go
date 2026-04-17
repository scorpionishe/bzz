package main

import (
	"log"
	"sync/atomic"
)

// verbose controls whether low-level per-keystroke logs are emitted.
// Toggled via the -verbose CLI flag. Default: off.
var verbose int32

func setVerbose(v bool) {
	if v {
		atomic.StoreInt32(&verbose, 1)
	} else {
		atomic.StoreInt32(&verbose, 0)
	}
}

// vlog writes to the log only when verbose mode is enabled. Used for
// per-keystroke / per-word tracing that would otherwise flood the log file.
func vlog(format string, args ...interface{}) {
	if atomic.LoadInt32(&verbose) == 1 {
		log.Printf(format, args...)
	}
}
