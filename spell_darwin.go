//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit

#include <stdlib.h>
#include <string.h>
#include <AppKit/AppKit.h>

// bzzSpellHasLanguage reports whether the system spell checker offers lang.
static int bzzSpellHasLanguage(const char *lang) {
    @autoreleasepool {
        NSString *l = [NSString stringWithUTF8String:lang];
        return [[[NSSpellChecker sharedSpellChecker] availableLanguages] containsObject:l] ? 1 : 0;
    }
}

// bzzSpellIsCorrect returns 1 when the checker finds no misspelling in word.
static int bzzSpellIsCorrect(const char *word, const char *lang) {
    @autoreleasepool {
        NSString *w = [NSString stringWithUTF8String:word];
        NSString *l = [NSString stringWithUTF8String:lang];
        NSRange r = [[NSSpellChecker sharedSpellChecker] checkSpellingOfString:w
                                                                   startingAt:0
                                                                     language:l
                                                                         wrap:NO
                                                       inSpellDocumentWithTag:0
                                                                    wordCount:NULL];
        return r.location == NSNotFound ? 1 : 0;
    }
}

// bzzSpellGuesses returns the checker's suggestions joined by '\n' (strdup'd,
// caller frees), or NULL when there are none.
static char *bzzSpellGuesses(const char *word, const char *lang) {
    @autoreleasepool {
        NSString *w = [NSString stringWithUTF8String:word];
        NSString *l = [NSString stringWithUTF8String:lang];
        NSArray<NSString *> *g = [[NSSpellChecker sharedSpellChecker] guessesForWordRange:NSMakeRange(0, w.length)
                                                                                inString:w
                                                                                language:l
                                                                  inSpellDocumentWithTag:0];
        if (g == nil || g.count == 0) return NULL;
        return strdup([[g componentsJoinedByString:@"\n"] UTF8String]);
    }
}
*/
import "C"

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

// systemSpellChecker wraps NSSpellChecker. Calls are serialized: the checker
// is not documented as thread-safe, and we reach it from the event-tap thread
// (Misspelled) and from replace goroutines (Suggest).
type systemSpellChecker struct {
	mu   sync.Mutex
	lang *C.char
}

// newSystemSpellChecker returns a checker for lang ("ru"), or an error when
// the system has no dictionary for it. The first lookup is slow (~70 ms) while
// the system loads the dictionary, so it is warmed up here.
func newSystemSpellChecker(lang string) (SpellChecker, error) {
	cl := C.CString(lang)
	if C.bzzSpellHasLanguage(cl) == 0 {
		C.free(unsafe.Pointer(cl))
		return nil, fmt.Errorf("system spell checker has no %q dictionary", lang)
	}
	s := &systemSpellChecker{lang: cl}
	s.IsCorrect("слово") // warm-up
	return s, nil
}

func (s *systemSpellChecker) IsCorrect(word string) bool {
	cw := C.CString(word)
	defer C.free(unsafe.Pointer(cw))
	s.mu.Lock()
	defer s.mu.Unlock()
	return C.bzzSpellIsCorrect(cw, s.lang) == 1
}

func (s *systemSpellChecker) Guesses(word string) []string {
	cw := C.CString(word)
	defer C.free(unsafe.Pointer(cw))
	s.mu.Lock()
	out := C.bzzSpellGuesses(cw, s.lang)
	s.mu.Unlock()
	if out == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(out))
	return strings.Split(C.GoString(out), "\n")
}
