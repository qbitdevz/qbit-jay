package engramimport

import (
	"crypto/sha256"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// Spec S2 (6.3 Importación automática): the Engram database is copied with the
// SQLite backup API and the source is never modified.

func fileHash(t *testing.T, path string) [32]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

// makeEngramDir builds a real, migrated Engram-shaped data dir (engram.db) with
// n observations, closed cleanly.
func makeEngramDir(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(store.Config{DataDir: dir, MaxObservationLength: 50000, MaxContextResults: 20, MaxSearchResults: 20, DedupeWindow: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession("s1", "proj", dir); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if _, err := s.AddObservation(store.AddObservationParams{SessionID: "s1", Type: "note", Title: "t" + string(rune('a'+i)), Content: "content " + string(rune('a'+i)), Project: "proj"}); err != nil {
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

func countObservations(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCopyLeavesSourceByteIdenticalWithOpenWriter(t *testing.T) {
	src := makeEngramDir(t, 3)
	srcDB := filepath.Join(src, "engram.db")

	// An Engram process still has the database open in WAL mode with a row that
	// has not been checkpointed into the main file.
	dsn := (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(srcDB), "/"), RawQuery: "_pragma=journal_mode(WAL)&_pragma=wal_autocheckpoint(0)"}).String()
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec(`INSERT INTO observations (sync_id, session_id, type, title, content, scope, project, normalized_hash, revision_count, duplicate_count, created_at, updated_at) VALUES ('wal-only','s1','note','wal row','only in the wal','project','proj','h',1,1,datetime('now'),datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	before := fileHash(t, srcDB)

	dst := filepath.Join(t.TempDir(), "data")
	if err := Copy(srcDB, dst); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := countObservations(t, filepath.Join(dst, store.DatabaseFile)); got != 4 {
		t.Fatalf("copy has %d observations, want 4 (3 + the one only in the WAL)", got)
	}
	if after := fileHash(t, srcDB); after != before {
		t.Fatal("source database file changed")
	}
	// qbit-jay's own migrations run on the copy.
	s, err := store.New(store.Config{DataDir: dst, MaxObservationLength: 50000, MaxContextResults: 20, MaxSearchResults: 20, DedupeWindow: time.Hour})
	if err != nil {
		t.Fatalf("open copy with the qbit-jay store: %v", err)
	}
	_ = s.Close()
	if after := fileHash(t, srcDB); after != before {
		t.Fatal("source database file changed")
	}
}

func TestCopyOfCleanlyClosedSourceTouchesNothingInSourceDir(t *testing.T) {
	src := makeEngramDir(t, 2)
	before := dirSnapshot(t, src)
	if err := Copy(filepath.Join(src, "engram.db"), filepath.Join(t.TempDir(), "d")); err != nil {
		t.Fatal(err)
	}
	if after := dirSnapshot(t, src); after != before {
		t.Fatalf("source directory changed:\nbefore %s\nafter  %s", before, after)
	}
}

func dirSnapshot(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		info, _ := e.Info()
		b.WriteString(e.Name() + ":" + string(rune(info.Size())) + ";")
		if !e.IsDir() {
			h := fileHash(t, filepath.Join(dir, e.Name()))
			b.WriteString(string(h[:4]))
		}
	}
	return b.String()
}

func TestCopyRefusesMissingSourceAndExistingTarget(t *testing.T) {
	dst := t.TempDir()
	if err := Copy(filepath.Join(dst, "nope.db"), filepath.Join(dst, "d")); err == nil {
		t.Fatal("missing source must fail")
	}
	src := makeEngramDir(t, 1)
	if err := os.WriteFile(filepath.Join(dst, store.DatabaseFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Copy(filepath.Join(src, "engram.db"), dst); err == nil {
		t.Fatal("an existing target database must never be overwritten by Copy")
	}
}

func TestAutoImport(t *testing.T) {
	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	t.Setenv("ENGRAM_DATA_DIR", "")
	home := t.TempDir()
	engramDir := filepath.Join(home, ".engram")
	made := makeEngramDir(t, 2)
	if err := os.Rename(made, engramDir); err != nil {
		t.Fatal(err)
	}
	opts := Options{Home: home, Stderr: &strings.Builder{}}
	dataDir := filepath.Join(home, ".qbit-jay")

	var out strings.Builder
	opts.Stderr = &out
	imported, err := AutoImport(dataDir, opts)
	if err != nil || !imported {
		t.Fatalf("AutoImport = %v, %v", imported, err)
	}
	if !strings.Contains(out.String(), filepath.Join(engramDir, "engram.db")) || !strings.Contains(out.String(), "QBIT_JAY_NO_IMPORT") {
		t.Fatalf("message must say what and from where, and name the opt-out: %q", out.String())
	}
	if n := countObservations(t, filepath.Join(dataDir, store.DatabaseFile)); n != 2 {
		t.Fatalf("imported %d observations, want 2", n)
	}

	// The qbit-jay db exists now: nothing happens, silently, and it is not replaced.
	h := fileHash(t, filepath.Join(dataDir, store.DatabaseFile))
	out.Reset()
	imported, err = AutoImport(dataDir, opts)
	if err != nil || imported || out.Len() != 0 {
		t.Fatalf("second start must skip silently: %v %v %q", imported, err, out.String())
	}
	if fileHash(t, filepath.Join(dataDir, store.DatabaseFile)) != h {
		t.Fatal("existing qbit-jay db was modified")
	}
}

func TestAutoImportSkips(t *testing.T) {
	t.Setenv("ENGRAM_DATA_DIR", "")
	cases := map[string]struct {
		env     string
		engram  bool
		sameDir bool
	}{
		"opt-out":          {env: "1", engram: true},
		"no engram db":     {engram: false},
		"engram is target": {engram: true, sameDir: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("QBIT_JAY_NO_IMPORT", c.env)
			home := t.TempDir()
			if c.engram {
				if err := os.Rename(makeEngramDir(t, 1), filepath.Join(home, ".engram")); err != nil {
					t.Fatal(err)
				}
			}
			dataDir := filepath.Join(home, ".qbit-jay")
			if c.sameDir {
				dataDir = filepath.Join(home, ".engram")
				t.Setenv("ENGRAM_DATA_DIR", dataDir)
			}
			var out strings.Builder
			imported, err := AutoImport(dataDir, Options{Home: home, Stderr: &out})
			if err != nil || imported || out.Len() != 0 {
				t.Fatalf("want a silent skip, got %v %v %q", imported, err, out.String())
			}
			if _, err := os.Stat(filepath.Join(dataDir, store.DatabaseFile)); !os.IsNotExist(err) {
				t.Fatalf("no qbit-jay db may be created: %v", err)
			}
		})
	}
}

func TestAutoImportHonoursEngramDataDir(t *testing.T) {
	t.Setenv("QBIT_JAY_NO_IMPORT", "")
	home := t.TempDir()
	custom := makeEngramDir(t, 1)
	t.Setenv("ENGRAM_DATA_DIR", custom)
	dataDir := filepath.Join(home, ".qbit-jay")
	imported, err := AutoImport(dataDir, Options{Home: home, Stderr: &strings.Builder{}})
	if err != nil || !imported {
		t.Fatalf("AutoImport = %v, %v", imported, err)
	}
}
