package main

// exceptions.go — Persistent store for learned rollback exceptions.
//
// Storage layout:
//   macOS:   ~/Library/Application Support/Bzz/exceptions.json
//   Windows: %APPDATA%\Bzz\exceptions.json
//   Override: BZZ_CONFIG_DIR env var (used in tests; RUSWITCH_CONFIG_DIR also honored for compat)
//
// Thread-safe: all public methods acquire RWMutex.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	exceptionsFileName    = "exceptions.json"
	exceptionsSchemaVer   = 1
	exceptionsHardCap     = 10000
	exceptionsGlobalApp   = "*"
	exceptionsPruneMinAge = 30 * 24 * time.Hour
)

type Exception struct {
	App      string    `json:"app"`
	Word     string    `json:"word"`
	Added    time.Time `json:"added"`
	HitCount int       `json:"hit_count"`
}

type exceptionsFile struct {
	Version int         `json:"version"`
	Updated time.Time   `json:"updated"`
	Entries []Exception `json:"entries"`
}

type ExceptionStore struct {
	mu      sync.RWMutex
	path    string
	index   map[string]*Exception // key = app + "\x00" + word
	entries []*Exception          // ordered for persistence
}

// NewExceptionStore loads the store from disk, creating parent dir if needed.
// If the file is missing, returns an empty store.
// If the file is corrupt, renames it to .corrupt.bak and starts fresh.
func NewExceptionStore() (*ExceptionStore, error) {
	dir, err := defaultConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir config: %w", err)
	}
	path := filepath.Join(dir, exceptionsFileName)
	s := &ExceptionStore{
		path:  path,
		index: make(map[string]*Exception),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func defaultConfigDir() (string, error) {
	// BZZ_CONFIG_DIR overrides; legacy RUSWITCH_CONFIG_DIR still works for tests.
	if v := os.Getenv("BZZ_CONFIG_DIR"); v != "" {
		return v, nil
	}
	if v := os.Getenv("RUSWITCH_CONFIG_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		newDir := filepath.Join(home, "Library", "Application Support", "Bzz")
		// Migrate from old RuSwitch directory if present and new one doesn't exist.
		oldDir := filepath.Join(home, "Library", "Application Support", "RuSwitch")
		migrateConfigDir(oldDir, newDir)
		return newDir, nil
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		newDir := filepath.Join(base, "Bzz")
		migrateConfigDir(filepath.Join(base, "RuSwitch"), newDir)
		return newDir, nil
	default:
		newDir := filepath.Join(home, ".config", "bzz")
		migrateConfigDir(filepath.Join(home, ".config", "ruswitch"), newDir)
		return newDir, nil
	}
}

// migrateConfigDir renames the old config dir to the new one if the old one
// exists and the new one doesn't. Best-effort: silent failure.
func migrateConfigDir(oldDir, newDir string) {
	if _, err := os.Stat(newDir); err == nil {
		return // new dir already exists, do nothing
	}
	if _, err := os.Stat(oldDir); err != nil {
		return // old dir doesn't exist, nothing to migrate
	}
	_ = os.Rename(oldDir, newDir)
}

func makeKey(app, word string) string {
	return strings.ToLower(app) + "\x00" + strings.ToLower(word)
}

func (s *ExceptionStore) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var file exceptionsFile
	if err := json.Unmarshal(data, &file); err != nil {
		// corrupt — move aside, start empty
		backup := s.path + ".corrupt.bak"
		_ = os.Rename(s.path, backup)
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
	for i := range file.Entries {
		entry := file.Entries[i]
		if entry.Word == "" {
			continue
		}
		if entry.App == "" {
			entry.App = exceptionsGlobalApp
		}
		ptr := &entry
		s.entries = append(s.entries, ptr)
		s.index[makeKey(entry.App, entry.Word)] = ptr
	}
	return nil
}

// IsException returns true if (app, word) or (*, word) is present.
// Lookup is case-insensitive. Uses full Lock because we increment HitCount.
func (s *ExceptionStore) IsException(app, word string) bool {
	if word == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if app != "" {
		if e, ok := s.index[makeKey(app, word)]; ok {
			e.HitCount++
			return true
		}
	}
	if e, ok := s.index[makeKey(exceptionsGlobalApp, word)]; ok {
		e.HitCount++
		return true
	}
	return false
}

// Add inserts (app, word) into the store and persists to disk.
// Repeated adds increment HitCount instead of duplicating.
// Pass app == "" to store globally.
func (s *ExceptionStore) Add(app, word string) error {
	if word == "" {
		return errors.New("empty word")
	}
	if app == "" {
		app = exceptionsGlobalApp
	}

	s.mu.Lock()
	key := makeKey(app, word)
	if existing, ok := s.index[key]; ok {
		existing.HitCount++
	} else {
		e := &Exception{
			App:      app,
			Word:     word,
			Added:    time.Now().UTC(),
			HitCount: 1,
		}
		s.entries = append(s.entries, e)
		s.index[key] = e
	}
	s.maybePruneLocked()
	snapshot := s.snapshotLocked()
	s.mu.Unlock()

	return writeAtomic(s.path, snapshot)
}

// Forget removes entries matching word from all apps. Returns count removed.
func (s *ExceptionStore) Forget(word string) (int, error) {
	s.mu.Lock()
	removed := 0
	kept := s.entries[:0]
	for _, e := range s.entries {
		if strings.EqualFold(e.Word, word) {
			delete(s.index, makeKey(e.App, e.Word))
			removed++
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	if removed == 0 {
		return 0, nil
	}
	if err := writeAtomic(s.path, snapshot); err != nil {
		return removed, err
	}
	return removed, nil
}

// ForgetApp removes all entries for a single app bundle id.
func (s *ExceptionStore) ForgetApp(app string) (int, error) {
	s.mu.Lock()
	removed := 0
	kept := s.entries[:0]
	for _, e := range s.entries {
		if strings.EqualFold(e.App, app) {
			delete(s.index, makeKey(e.App, e.Word))
			removed++
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	if removed == 0 {
		return 0, nil
	}
	return removed, writeAtomic(s.path, snapshot)
}

// Clear removes every entry.
func (s *ExceptionStore) Clear() error {
	s.mu.Lock()
	s.entries = nil
	s.index = make(map[string]*Exception)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return writeAtomic(s.path, snapshot)
}

// List returns a copy of all entries, for CLI/UI.
func (s *ExceptionStore) List() []Exception {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Exception, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	return out
}

func (s *ExceptionStore) maybePruneLocked() {
	if len(s.entries) < exceptionsHardCap {
		return
	}
	cutoff := time.Now().Add(-exceptionsPruneMinAge)
	kept := s.entries[:0]
	for _, e := range s.entries {
		if e.HitCount == 1 && e.Added.Before(cutoff) {
			delete(s.index, makeKey(e.App, e.Word))
			continue
		}
		kept = append(kept, e)
	}
	s.entries = kept
}

func (s *ExceptionStore) snapshotLocked() exceptionsFile {
	out := exceptionsFile{
		Version: exceptionsSchemaVer,
		Updated: time.Now().UTC(),
		Entries: make([]Exception, len(s.entries)),
	}
	for i, e := range s.entries {
		out.Entries[i] = *e
	}
	return out
}

func writeAtomic(path string, file exceptionsFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
