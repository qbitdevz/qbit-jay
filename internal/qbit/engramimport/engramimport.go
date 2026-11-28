// Package engramimport copies an Engram database into qbit-jay's data
// directory (spec 6.3).
//
// The copy uses the SQLite online backup API, so it is consistent even while
// Engram has the database open. The source is opened read-only and is never
// modified, moved or deleted. qbit-jay's own migrations run later, when the
// store opens the copy.
package engramimport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"

	"github.com/qbitdevz/qbit-jay/internal/qbit/env"
	"github.com/qbitdevz/qbit-jay/internal/qbit/paths"
)

const (
	// EngramDirName and EngramDatabaseFile are upstream's names.
	EngramDirName      = ".engram"
	EngramDatabaseFile = "engram.db"

	// NoImportVar disables the automatic import (QBIT_JAY_NO_IMPORT=1).
	NoImportVar = "NO_IMPORT"
)

// Options are test seams and output sinks. The zero value is usable.
type Options struct {
	Home   string           // defaults to the user home directory
	Stderr io.Writer        // receives the one-line import notice (never stdout: MCP speaks on it)
	Now    func() time.Time // defaults to time.Now
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o Options) home() (string, error) {
	if o.Home != "" {
		return o.Home, nil
	}
	return os.UserHomeDir()
}

// DefaultSourceDir is where Engram keeps its data: ENGRAM_DATA_DIR when set,
// otherwise ~/.engram.
func DefaultSourceDir(o Options) (string, error) {
	if dir, ok := env.LookupLegacy("DATA_DIR"); ok && strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	home, err := o.home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, EngramDirName), nil
}

// Result describes a finished import.
type Result struct {
	Source           string
	Destination      string
	Bytes            int64
	BackupOfExisting string // set when an existing qbit-jay database was set aside
}

func noImport() bool {
	switch strings.ToLower(strings.TrimSpace(env.Get(NoImportVar))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func sameDir(a, b string) bool {
	a, errA := filepath.Abs(a)
	b, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// AutoImport imports the Engram database on first start. It does nothing, and
// says nothing, when QBIT_JAY_NO_IMPORT is set, when the qbit-jay database
// already exists, when there is no Engram database, or when the Engram data
// directory is the qbit-jay one. It returns true when it imported.
func AutoImport(dataDir string, o Options) (bool, error) {
	if noImport() {
		return false, nil
	}
	target := filepath.Join(dataDir, paths.DatabaseFile)
	if _, err := os.Lstat(target); err == nil {
		return false, nil
	}
	srcDir, err := DefaultSourceDir(o)
	if err != nil {
		return false, nil // no home directory, nothing to import from
	}
	if sameDir(srcDir, dataDir) {
		return false, nil
	}
	srcDB := filepath.Join(srcDir, EngramDatabaseFile)
	info, err := os.Stat(srcDB)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	if err := Copy(srcDB, dataDir); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil // another qbit-jay process imported first
		}
		return false, fmt.Errorf("import Engram data from %s: %w", srcDB, err)
	}
	if o.Stderr != nil {
		size := int64(0)
		if st, err := os.Stat(target); err == nil {
			size = st.Size()
		}
		_, _ = fmt.Fprintf(o.Stderr, "qbit-jay: imported Engram data from %s into %s (%d KB; the Engram database was not modified; set QBIT_JAY_NO_IMPORT=1 to skip this)\n", srcDB, target, size/1024)
	}
	return true, nil
}

// Explicit implements `qbit-jay import --from-engram [path]`. src is an Engram
// database file or its data directory. When the qbit-jay database already
// holds data the import is refused unless force is set; force first renames
// the existing database (and its WAL sidecars) to a timestamped backup.
func Explicit(src, dataDir string, force bool, o Options) (*Result, error) {
	if strings.TrimSpace(src) == "" {
		dir, err := DefaultSourceDir(o)
		if err != nil {
			return nil, err
		}
		src = dir
	}
	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("Engram source %s: %w", src, err)
	}
	srcDB := src
	if info.IsDir() {
		srcDB = filepath.Join(src, EngramDatabaseFile)
		if _, err := os.Stat(srcDB); err != nil {
			return nil, fmt.Errorf("no %s in %s: %w", EngramDatabaseFile, src, err)
		}
	}
	target := filepath.Join(dataDir, paths.DatabaseFile)
	if sameFile(srcDB, target) {
		return nil, fmt.Errorf("the source and the qbit-jay database are the same file: %s", target)
	}

	res := &Result{Source: srcDB, Destination: target}
	if _, err := os.Lstat(target); err == nil {
		if !force && hasData(target) {
			return nil, fmt.Errorf("the qbit-jay database %s already has data; nothing was imported. Re-run with --force to set it aside as a timestamped backup and import over it", target)
		}
		backup, err := setAside(target, o.now())
		if err != nil {
			return nil, err
		}
		res.BackupOfExisting = backup
	}
	if err := Copy(srcDB, dataDir); err != nil {
		return res, err
	}
	if st, err := os.Stat(target); err == nil {
		res.Bytes = st.Size()
	}
	return res, nil
}

func sameFile(a, b string) bool {
	sa, errA := os.Stat(a)
	sb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(sa, sb)
}

// setAside renames the database and its sidecars to <name>.bak-<timestamp>.
func setAside(target string, now time.Time) (string, error) {
	stamp := now.UTC().Format("20060102T150405Z")
	backup := target + ".bak-" + stamp
	for n := 1; ; n++ {
		if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
			break
		}
		backup = fmt.Sprintf("%s.bak-%s-%d", target, stamp, n)
	}
	if err := os.Rename(target, backup); err != nil {
		return "", fmt.Errorf("set aside the existing database: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Lstat(target + suffix); err == nil {
			if err := os.Rename(target+suffix, backup+suffix); err != nil {
				return "", fmt.Errorf("set aside %s: %w", target+suffix, err)
			}
		}
	}
	return backup, nil
}

// hasData reports whether the database holds any session or observation. A
// database that cannot be read is treated as holding data.
func hasData(dbPath string) bool {
	db, err := sql.Open("sqlite", readOnlyDSN(dbPath, false))
	if err != nil {
		return true
	}
	defer func() { _ = db.Close() }()
	count := func(table string) (int, bool) {
		var n int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
		if err != nil {
			return 0, strings.Contains(err.Error(), "no such table")
		}
		return n, true
	}
	for _, table := range []string{"observations", "sessions"} {
		n, ok := count(table)
		if !ok || n > 0 {
			return true
		}
	}
	return false
}

func fileURI(path string, query string) string {
	slash := filepath.ToSlash(path)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash, RawQuery: query}).String()
}

// readOnlyDSN opens the file read-only. When nothing else has it open (no WAL
// sidecars) the immutable flag lets SQLite read it without creating any file
// next to it; otherwise the WAL is honoured so the copy sees committed data.
func readOnlyDSN(path string, immutableWhenIdle bool) string {
	if immutableWhenIdle && !exists(path+"-wal") && !exists(path+"-shm") {
		return fileURI(path, "mode=ro&immutable=1")
	}
	return fileURI(path, "mode=ro")
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

type backuper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

// Copy copies the SQLite database at srcDB to <dstDir>/qbit-jay.db with the
// online backup API. It never overwrites: an existing target is an error
// (os.ErrExist). The data is written to a temporary file and moved into place
// only when complete.
func Copy(srcDB, dstDir string) (err error) {
	target := filepath.Join(dstDir, paths.DatabaseFile)
	if exists(target) {
		return fmt.Errorf("%s already exists: %w", target, os.ErrExist)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dstDir, paths.DatabaseFile+".importing-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
			_ = os.Remove(tmpPath + "-journal")
		}
	}()

	if err := backup(srcDB, tmpPath); err != nil {
		return err
	}
	// Link then remove, so a concurrent importer never has its database replaced.
	if linkErr := os.Link(tmpPath, target); linkErr == nil {
		return os.Remove(tmpPath)
	} else if errors.Is(linkErr, os.ErrExist) {
		return fmt.Errorf("%s already exists: %w", target, os.ErrExist)
	}
	if exists(target) {
		return fmt.Errorf("%s already exists: %w", target, os.ErrExist)
	}
	return os.Rename(tmpPath, target)
}

func backup(srcDB, dstPath string) error {
	for _, immutable := range []bool{true, false} {
		err := backupOnce(srcDB, dstPath, immutable)
		if err == nil || !immutable {
			return err
		}
		// Retry honouring the WAL when the immutable read failed.
	}
	return nil
}

func backupOnce(srcDB, dstPath string, immutableWhenIdle bool) error {
	db, err := sql.Open("sqlite", readOnlyDSN(srcDB, immutableWhenIdle))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("open %s read-only: %w", srcDB, err)
	}
	defer func() { _ = conn.Close() }()
	// Fail early with a clear message when the file is not a database.
	var tables int
	if err := conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sqlite_master").Scan(&tables); err != nil {
		return fmt.Errorf("read %s: %w", srcDB, err)
	}
	return conn.Raw(func(driverConn any) error {
		b, ok := driverConn.(backuper)
		if !ok {
			return fmt.Errorf("the SQLite driver does not expose the backup API")
		}
		bk, err := b.NewBackup(dstPath)
		if err != nil {
			return err
		}
		for {
			more, err := bk.Step(-1)
			if err != nil {
				_ = bk.Finish()
				return err
			}
			if !more {
				break
			}
		}
		return bk.Finish()
	})
}
