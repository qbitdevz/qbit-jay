package telemetry

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         INTEGER NOT NULL,          -- unix milliseconds
	kind       TEXT    NOT NULL,          -- search, open, reformulation, context, injected_read, injected_update, save
	session    TEXT    NOT NULL,
	project    TEXT    NOT NULL DEFAULT '',
	query      TEXT    NOT NULL DEFAULT '',
	mode       TEXT    NOT NULL DEFAULT '',
	results    INTEGER NOT NULL DEFAULT 0,
	ids        TEXT    NOT NULL DEFAULT '', -- comma separated observation ids
	latency_ms REAL    NOT NULL DEFAULT 0,
	bytes      INTEGER NOT NULL DEFAULT 0,
	tokens     INTEGER NOT NULL DEFAULT 0,
	error      TEXT    NOT NULL DEFAULT '',
	extra      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS events_kind ON events(kind, ts);
CREATE TABLE IF NOT EXISTS injected (
	event_id INTEGER NOT NULL,
	session  TEXT    NOT NULL,
	title    TEXT    NOT NULL,
	reason   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS injected_event ON injected(event_id);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL DEFAULT 0
);
`

// FileName is the telemetry database inside the data directory.
const FileName = "telemetry.db"

// Path returns the telemetry database path for a data directory.
func Path(dataDir string) string { return filepath.Join(dataDir, FileName) }

// openDB opens (creating it when create is set) the telemetry database. The
// journal mode is DELETE so the telemetry state is one file, never a -wal pair.
func openDB(path string, create bool) (*sql.DB, error) {
	if !create {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := securePermissions(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(2000)&_pragma=journal_mode(DELETE)&_pragma=synchronous(OFF)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// securePermissions makes sure the database file exists owner-only before
// SQLite touches it (SQLite gives its journal the mode of the database file),
// and strips group/other bits from files left by an earlier run. Windows keeps
// the default ACLs. A read-only file is never made writable.
func securePermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrPermission) { // exists but not writable
			f = nil
		} else {
			return err
		}
	}
	if f != nil {
		_ = f.Close()
	}
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		if fi, err := os.Stat(path + suffix); err == nil && fi.Mode().Perm()&0o077 != 0 {
			_ = os.Chmod(path+suffix, fi.Mode().Perm()&^0o077)
		}
	}
	return nil
}

// bumpMeta adds delta to a persisted counter.
func bumpMeta(db *sql.DB, key string, delta int64) error {
	if delta == 0 {
		return nil
	}
	_, err := db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = value + excluded.value`, key, delta)
	return err
}

// prune enforces the retention window and the size cap. It is cheap when
// there is nothing to remove (one indexed delete and a size check).
func prune(db *sql.DB, path string, now time.Time, maxAge time.Duration, maxBytes int64) error {
	cutoff := now.Add(-maxAge).UnixMilli()
	if _, err := db.Exec(`DELETE FROM events WHERE ts < ?`, cutoff); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM injected WHERE event_id NOT IN (SELECT id FROM events)`); err != nil {
		return err
	}
	if maxBytes <= 0 {
		return nil
	}
	// Loop until under the cap. Each pass drops the oldest quarter (at least
	// one row), so the loop is bounded by the row count.
	for {
		fi, err := os.Stat(path)
		if err != nil || fi.Size() <= maxBytes {
			return nil
		}
		var n int64
		if err := db.QueryRow(`SELECT count(*) FROM events`).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return nil // nothing left to drop; the empty schema is the floor
		}
		if _, err := db.Exec(`DELETE FROM events WHERE id IN (SELECT id FROM events ORDER BY ts, id LIMIT max(1, ?/4))`, n); err != nil {
			return err
		}
		if _, err := db.Exec(`DELETE FROM injected WHERE event_id NOT IN (SELECT id FROM events)`); err != nil {
			return err
		}
		if _, err := db.Exec(`VACUUM`); err != nil {
			return fmt.Errorf("vacuum: %w", err)
		}
	}
}
