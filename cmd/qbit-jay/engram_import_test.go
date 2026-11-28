package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// Spec S2 (6.3): first-start import, `import --from-engram`, `migrate-copy`
// with an engram.db source. Everything runs against temporary directories.

func engramFixture(t *testing.T, observations int) string {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(store.Config{DataDir: dir, MaxObservationLength: 50000, MaxContextResults: 20, MaxSearchResults: 20, DedupeWindow: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession("s1", "proj", dir); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < observations; i++ {
		if _, err := s.AddObservation(store.AddObservationParams{SessionID: "s1", Type: "note", Title: "title " + string(rune('a'+i)), Content: "body " + string(rune('a'+i)), Project: "proj"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, store.DatabaseFile), filepath.Join(dir, "engram.db")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func hashFile(t *testing.T, path string) [32]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

func observationCount(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(dbPath), "/"), RawQuery: "mode=ro"}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestImportFromEngramCommand(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	src := engramFixture(t, 2)
	srcDB := filepath.Join(src, "engram.db")
	before := hashFile(t, srcDB)
	dataDir := filepath.Join(t.TempDir(), "qbit")

	var stdout, stderr bytes.Buffer
	if code := runImportFromEngram([]string{src}, dataDir, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), srcDB) || !strings.Contains(stdout.String(), filepath.Join(dataDir, store.DatabaseFile)) {
		t.Fatalf("output must say what was imported and from where: %q", stdout.String())
	}
	if n := observationCount(t, filepath.Join(dataDir, store.DatabaseFile)); n != 2 {
		t.Fatalf("imported %d observations, want 2", n)
	}
	if hashFile(t, srcDB) != before {
		t.Fatal("source database changed")
	}

	// A second import refuses with a non-zero exit and leaves the target alone.
	targetHash := hashFile(t, filepath.Join(dataDir, store.DatabaseFile))
	stdout.Reset()
	stderr.Reset()
	if code := runImportFromEngram([]string{srcDB}, dataDir, &stdout, &stderr); code == 0 {
		t.Fatal("import over existing data must exit non-zero")
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("the refusal must explain --force: %q", stderr.String())
	}
	if hashFile(t, filepath.Join(dataDir, store.DatabaseFile)) != targetHash {
		t.Fatal("refused import modified the target")
	}

	// --force sets the old database aside and imports again.
	stdout.Reset()
	stderr.Reset()
	if code := runImportFromEngram([]string{"--force", srcDB}, dataDir, &stdout, &stderr); code != 0 {
		t.Fatalf("--force exit %d: %s", code, stderr.String())
	}
	matches, _ := filepath.Glob(filepath.Join(dataDir, store.DatabaseFile+".bak-????????T??????Z"))
	if len(matches) != 1 || hashFile(t, matches[0]) != targetHash {
		t.Fatalf("expected one backup holding the previous bytes, got %v", matches)
	}
	if !strings.Contains(stdout.String(), matches[0]) {
		t.Fatalf("output must name the backup: %q", stdout.String())
	}
	if hashFile(t, srcDB) != before {
		t.Fatal("source database changed")
	}
}

func TestImportFromEngramUsageErrors(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	var stdout, stderr bytes.Buffer
	if code := runImportFromEngram([]string{"--bogus"}, t.TempDir(), &stdout, &stderr); code != 2 {
		t.Fatalf("unknown flag exit = %d", code)
	}
	if code := runImportFromEngram([]string{filepath.Join(t.TempDir(), "missing")}, t.TempDir(), &stdout, &stderr); code == 0 {
		t.Fatal("a missing source must fail")
	}
}

func TestStartupAutoImport(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.Rename(engramFixture(t, 3), filepath.Join(home, ".engram")); err != nil {
		t.Fatal(err)
	}
	srcDB := filepath.Join(home, ".engram", "engram.db")
	before := hashFile(t, srcDB)
	dataDir := filepath.Join(home, ".qbit-jay")

	t.Run("opt-out", func(t *testing.T) {
		t.Setenv("QBIT_JAY_NO_IMPORT", "1")
		var stderr bytes.Buffer
		autoImportFromEngram(dataDir, &stderr)
		if _, err := os.Stat(dataDir); !os.IsNotExist(err) || stderr.Len() != 0 {
			t.Fatalf("QBIT_JAY_NO_IMPORT=1 must not import: %v %q", err, stderr.String())
		}
	})

	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	var stderr bytes.Buffer
	autoImportFromEngram(dataDir, &stderr)
	if n := observationCount(t, filepath.Join(dataDir, store.DatabaseFile)); n != 3 {
		t.Fatalf("imported %d observations, want 3", n)
	}
	if !strings.Contains(stderr.String(), "imported Engram data from") {
		t.Fatalf("expected the one-line notice, got %q", stderr.String())
	}
	if strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("the notice must be one line: %q", stderr.String())
	}
	if hashFile(t, srcDB) != before {
		t.Fatal("source database changed")
	}

	stderr.Reset()
	autoImportFromEngram(dataDir, &stderr)
	if stderr.Len() != 0 {
		t.Fatalf("second start must be silent: %q", stderr.String())
	}
}

func TestCmdImportDispatchesFromEngram(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	src := engramFixture(t, 1)
	dataDir := filepath.Join(t.TempDir(), "qbit")
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"qbit-jay", "import", "--from-engram", src}
	stdout, _ := captureOutput(t, func() { cmdImport(store.Config{DataDir: dataDir}) })
	if !strings.Contains(stdout, "Imported Engram data") {
		t.Fatalf("unexpected output: %q", stdout)
	}
	if n := observationCount(t, filepath.Join(dataDir, store.DatabaseFile)); n != 1 {
		t.Fatalf("imported %d observations", n)
	}
}

func TestMigrateCopyAcceptsEngramDatabaseSource(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	src := engramFixture(t, 2)
	srcDB := filepath.Join(src, "engram.db")
	before := hashFile(t, srcDB)
	out := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := runMigrateCopy([]string{"--data-dir", src, "--output-dir", out, "--confirm-quiescent", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var result store.MigrateCopyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("result: %v %q", err, stdout.String())
	}
	if result.SourceDir != src || !result.MigrationCompleted {
		t.Fatalf("result = %+v", result)
	}
	if n := observationCount(t, filepath.Join(out, store.DatabaseFile)); n != 2 {
		t.Fatalf("copy has %d observations", n)
	}
	if hashFile(t, srcDB) != before {
		t.Fatal("source database changed")
	}
}

func TestImportFromEngramForceWithCorruptSourceKeepsDatabase(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	dataDir := filepath.Join(t.TempDir(), "qbit")
	var stdout, stderr bytes.Buffer
	if code := runImportFromEngram([]string{engramFixture(t, 2)}, dataDir, &stdout, &stderr); code != 0 {
		t.Fatalf("seed import exit %d: %s", code, stderr.String())
	}
	db := filepath.Join(dataDir, store.DatabaseFile)
	before := hashFile(t, db)
	corrupt := filepath.Join(t.TempDir(), "engram.db")
	if err := os.WriteFile(corrupt, []byte("not a database, padding padding padding padding padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := runImportFromEngram([]string{corrupt, "--force"}, dataDir, &stdout, &stderr); code == 0 {
		t.Fatal("a corrupt source must exit non-zero")
	}
	if hashFile(t, db) != before {
		t.Fatal("the original qbit-jay database changed")
	}
	if !strings.Contains(stderr.String(), db) {
		t.Fatalf("the error must name the existing database: %q", stderr.String())
	}
}

func TestStartupAutoImportFailureFallbackDoesNotSuggestForce(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".engram"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".engram", "engram.db"), []byte("not a database, padding padding padding padding padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(home, ".qbit-jay")
	var stderr bytes.Buffer
	autoImportFromEngram(dataDir, &stderr)
	if !strings.Contains(stderr.String(), "import --from-engram") || strings.Contains(stderr.String(), "--force") {
		t.Fatalf("fallback message: %q", stderr.String())
	}
	stderr.Reset()
	autoImportFromEngram(dataDir, &stderr)
	if strings.Count(stderr.String(), "\n") != 1 || strings.Contains(stderr.String(), "--force") {
		t.Fatalf("the second start must print one short warning: %q", stderr.String())
	}
}
