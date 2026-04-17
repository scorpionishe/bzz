package main

import (
	"os"
	"path/filepath"
	"testing"
)

func newTempStore(t *testing.T) *ExceptionStore {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RUSWITCH_CONFIG_DIR", dir)
	s, err := NewExceptionStore()
	if err != nil {
		t.Fatalf("NewExceptionStore: %v", err)
	}
	return s
}

func TestStoreLoadEmpty(t *testing.T) {
	s := newTempStore(t)
	if got := s.List(); len(got) != 0 {
		t.Errorf("expected empty, got %d entries", len(got))
	}
	if s.IsException("com.jetbrains.WebStorm", "ru-en") {
		t.Error("expected false on empty store")
	}
}

func TestStoreAddAndCheck(t *testing.T) {
	s := newTempStore(t)
	if err := s.Add("com.jetbrains.WebStorm", "ru-en"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !s.IsException("com.jetbrains.WebStorm", "ru-en") {
		t.Error("expected exception to be hit")
	}
	if s.IsException("com.apple.Safari", "ru-en") {
		t.Error("app-specific exception should not cross apps")
	}
}

func TestStoreGlobalFallback(t *testing.T) {
	s := newTempStore(t)
	if err := s.Add("*", "github"); err != nil {
		t.Fatalf("Add global: %v", err)
	}
	if !s.IsException("any.app", "github") {
		t.Error("global exception should match any app")
	}
	if !s.IsException("", "github") {
		t.Error("global exception should match empty app")
	}
}

func TestStoreUpsertIncrementsHitCount(t *testing.T) {
	s := newTempStore(t)
	for i := 0; i < 3; i++ {
		if err := s.Add("com.apple.Terminal", "cd"); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].HitCount != 3 {
		t.Errorf("expected HitCount=3, got %d", list[0].HitCount)
	}
}

func TestStorePersistAndReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RUSWITCH_CONFIG_DIR", dir)

	s1, err := NewExceptionStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Add("com.jetbrains.WebStorm", "ru-en"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Add("*", "github"); err != nil {
		t.Fatal(err)
	}

	s2, err := NewExceptionStore()
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsException("com.jetbrains.WebStorm", "ru-en") {
		t.Error("persisted entry not loaded")
	}
	if !s2.IsException("anything", "github") {
		t.Error("persisted global entry not loaded")
	}
}

func TestStoreAtomicWrite(t *testing.T) {
	s := newTempStore(t)
	if err := s.Add("app", "word"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path); err != nil {
		t.Errorf("main file missing: %v", err)
	}
	if _, err := os.Stat(s.path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file should be cleaned up after rename")
	}
}

func TestStoreForget(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add("app1", "foo")
	_ = s.Add("app2", "foo")
	_ = s.Add("app1", "bar")

	removed, err := s.Forget("foo")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	if s.IsException("app1", "foo") {
		t.Error("foo should be gone")
	}
	if !s.IsException("app1", "bar") {
		t.Error("bar should remain")
	}
}

func TestStoreForgetApp(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add("app1", "foo")
	_ = s.Add("app1", "bar")
	_ = s.Add("app2", "foo")

	removed, err := s.ForgetApp("app1")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	if !s.IsException("app2", "foo") {
		t.Error("app2 entries should be kept")
	}
}

func TestStoreCorruptFileRecovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RUSWITCH_CONFIG_DIR", dir)
	path := filepath.Join(dir, exceptionsFileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewExceptionStore()
	if err != nil {
		t.Fatalf("should recover from corrupt: %v", err)
	}
	if len(s.List()) != 0 {
		t.Error("expected fresh empty store after corruption")
	}
	if _, err := os.Stat(path + ".corrupt.bak"); err != nil {
		t.Errorf("expected .corrupt.bak: %v", err)
	}
}

func TestStoreCaseInsensitive(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add("com.apple.Terminal", "CD")
	if !s.IsException("com.apple.terminal", "cd") {
		t.Error("lookup should be case-insensitive")
	}
}
