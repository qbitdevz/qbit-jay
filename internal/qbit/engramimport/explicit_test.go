package engramimport

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

func TestExplicitImportRefusesExistingDataUnlessForce(t *testing.T) {
	src := makeEngramDir(t, 2)
	srcDB := filepath.Join(src, "engram.db")
	dataDir := filepath.Join(t.TempDir(), "d")

	if _, err := Explicit(srcDB, dataDir, false, Options{}); err != nil {
		t.Fatalf("import into an empty dir: %v", err)
	}
	first := fileHash(t, filepath.Join(dataDir, store.DatabaseFile))

	_, err := Explicit(srcDB, dataDir, false, Options{})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("second import must refuse and mention --force, got %v", err)
	}
	if fileHash(t, filepath.Join(dataDir, store.DatabaseFile)) != first {
		t.Fatal("refused import changed the target")
	}

	fixed := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	res, err := Explicit(srcDB, dataDir, true, Options{Now: func() time.Time { return fixed }})
	if err != nil {
		t.Fatalf("--force: %v", err)
	}
	if res.BackupOfExisting == "" {
		t.Fatal("--force must report the backup of the existing database")
	}
	if fileHash(t, res.BackupOfExisting) != first {
		t.Fatal("the backup must hold the previous database bytes")
	}
	if !strings.Contains(filepath.Base(res.BackupOfExisting), "20261128T120000Z") {
		t.Fatalf("backup name should be timestamped: %s", res.BackupOfExisting)
	}
}

func TestExplicitImportAcceptsDirectoryAndEmptyExistingDatabase(t *testing.T) {
	src := makeEngramDir(t, 1)
	dataDir := t.TempDir()
	// An empty qbit-jay database (created by a command run with the import
	// disabled) does not count as data.
	s, err := store.New(store.Config{DataDir: dataDir, MaxObservationLength: 50000, MaxContextResults: 20, MaxSearchResults: 20, DedupeWindow: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	res, err := Explicit(src, dataDir, false, Options{})
	if err != nil {
		t.Fatalf("a directory source and an empty target must work: %v", err)
	}
	if res.BackupOfExisting == "" {
		t.Fatal("the empty database must be set aside, not deleted")
	}
	if n := countObservations(t, filepath.Join(dataDir, store.DatabaseFile)); n != 1 {
		t.Fatalf("got %d observations", n)
	}
}
