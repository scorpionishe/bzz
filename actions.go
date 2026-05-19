package main

import (
	"log"
	"sync/atomic"
	"time"
)

func handleUndo(buf *Buffer, store *ExceptionStore) bool {
	original, replaced, ok := undo.Get()
	if !ok {
		return false
	}
	if store != nil {
		app := FrontmostAppID()
		if err := store.Add(app, original); err == nil {
			log.Printf("Learned exception (Cmd+Z): %q in %q", original, app)
		}
	}
	log.Printf("Undo: reverting %q → %q", replaced, original)
	go func() {
		atomic.StoreInt32(&replacing, 1)
		buf.Clear()
		for i := 0; i < len([]rune(replaced)); i++ {
			sendBackspaceKey()
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		for _, ch := range original {
			sendChar(ch)
			time.Sleep(5 * time.Millisecond)
		}
		switchLang()
		time.Sleep(30 * time.Millisecond)
		atomic.StoreInt32(&replacing, 0)
	}()
	return true
}

func performEnterReplacement(buf *Buffer, tracker *RollbackTracker, word, corrected string) {
	log.Printf("Fix (enter): %q → %q", word, corrected)
	atomic.StoreInt32(&replacing, 1)
	buf.Clear()
	for i := 0; i < len([]rune(word)); i++ {
		sendBackspaceKey()
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	newText := corrected
	for _, ch := range corrected {
		sendChar(ch)
		time.Sleep(5 * time.Millisecond)
	}
	undo.Save(word, newText)
	if tracker != nil {
		tracker.OnConversion(word, newText, FrontmostAppID())
	}
	switchLang()
	time.Sleep(30 * time.Millisecond)
	atomic.StoreInt32(&replacing, 0)
	time.Sleep(10 * time.Millisecond)
	sendEnter()
}
