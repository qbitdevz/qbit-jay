package store

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec S4/D2: the default data directory is ~/.qbit-jay and the database file
// is qbit-jay.db.
func TestDefaultConfigUsesQbitJayDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".qbit-jay"); cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestStoreCreatesQbitJayDatabaseFile(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = t.TempDir()
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "qbit-jay.db")); err != nil {
		t.Fatalf("qbit-jay.db not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "engram.db")); err == nil {
		t.Fatal("engram.db must not be created")
	}
}
