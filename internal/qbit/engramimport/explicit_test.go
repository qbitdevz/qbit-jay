package engramimport

import (
	"os"
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

func TestForceImportFromCorruptSourceKeepsExistingDatabase(t *testing.T) {
	good := makeEngramDir(t, 2)
	dataDir := filepath.Join(t.TempDir(), "d")
	if _, err := Explicit(filepath.Join(good, "engram.db"), dataDir, false, Options{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dataDir, store.DatabaseFile)
	before := fileHash(t, target)

	corrupt := filepath.Join(t.TempDir(), "engram.db")
	if err := os.WriteFile(corrupt, []byte("this is not a sqlite database, just text padding padding padding padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Explicit(corrupt, dataDir, true, Options{})
	if err == nil {
		t.Fatal("a corrupt source must fail")
	}
	if !strings.Contains(err.Error(), target) {
		t.Fatalf("the error must name the existing database %s: %v", target, err)
	}
	if fileHash(t, target) != before {
		t.Fatal("the existing database changed")
	}
	entries, _ := os.ReadDir(dataDir)
	for _, e := range entries {
		if e.Name() != store.DatabaseFile {
			t.Fatalf("unexpected leftover %s after a failed import", e.Name())
		}
	}
}

func TestAutoImportFailureWritesMarkerAndSkipsNextTime(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	home := t.TempDir()
	srcDir := filepath.Join(home, ".engram")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "engram.db"), []byte("not a database, padding padding padding padding padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(home, ".qbit-jay")
	if ok, err := AutoImport(dataDir, Options{Home: home}); ok || err == nil {
		t.Fatalf("first attempt must fail, got %v %v", ok, err)
	}
	marker := filepath.Join(dataDir, FailedMarkerFile)
	raw, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(raw), filepath.Join(srcDir, "engram.db")) {
		t.Fatalf("the marker must name the source: %q %v", raw, err)
	}
	var stderr strings.Builder
	ok, err := AutoImport(dataDir, Options{Home: home, Stderr: &stderr})
	if ok || err != nil {
		t.Fatalf("the second attempt must be skipped silently, got %v %v", ok, err)
	}
	if !strings.Contains(stderr.String(), "import --from-engram") || strings.Contains(stderr.String(), "--force") || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("one short warning pointing to the explicit import expected: %q", stderr.String())
	}

	// A successful explicit import clears the marker.
	good := makeEngramDir(t, 1)
	if _, err := Explicit(good, dataDir, false, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the marker must be removed after a successful import: %v", err)
	}
}
