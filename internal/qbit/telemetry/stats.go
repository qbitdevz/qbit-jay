package telemetry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// IDCount is an observation id with how many times it was opened.
type IDCount struct {
	ID    string
	Count int
}

// SessionTokens is the context tokens injected in one session.
type SessionTokens struct {
	Session string
	Tokens  int
	Items   int
}

// Summary is what `qbit-jay stats --telemetry` prints.
type Summary struct {
	Path            string
	Exists          bool
	Searches        int
	ZeroResult      int
	Reformulations  int
	Opens           int
	AvgSearchMS     float64
	TopOpened       []IDCount
	ContextEvents   int
	InjectedTokens  int
	InjectedItems   int
	InjectedRead    int
	InjectedUpdated int
	PerSession      []SessionTokens
	Saves           int
	SaveErrors      int
	SaveLimit       int
	AvgSaveMS       float64
	Project         string // non-empty: events were filtered to this project
	Dropped         int64  // events lost (queue full), all processes, all time
	WriteErrors     int64
}

// ZeroRate, ReformulationRate and OpenRate are fractions of searches.
func (s Summary) ZeroRate() float64          { return ratio(s.ZeroResult, s.Searches) }
func (s Summary) ReformulationRate() float64 { return ratio(s.Reformulations, s.Searches) }
func (s Summary) OpenRate() float64          { return ratio(s.Opens, s.Searches) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// LoadProject summarizes the telemetry database of dataDir, restricted to one
// project when project is non-empty (matched case-insensitively; events
// recorded without a project, such as a save that relied on the server
// default, are not counted for a filtered view). The dropped/error counters
// are global and never filtered. A missing file is not an error: it yields an
// empty summary with Exists=false. It never creates the file.
func LoadProject(dataDir, project string) (Summary, error) {
	path := Path(dataDir)
	sum := Summary{Path: path, Project: project}
	db, err := openDB(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return sum, nil
	}
	if err != nil {
		return sum, err
	}
	defer func() { _ = db.Close() }()
	sum.Exists = true

	where, wargs := "", []any(nil)
	andKind := ""
	if project != "" {
		where, andKind, wargs = " WHERE project = ? COLLATE NOCASE", " AND project = ? COLLATE NOCASE", []any{project}
	}
	one := func(dst any, q string, args ...any) error { return db.QueryRow(q, args...).Scan(dst) }
	if err := db.QueryRow(`SELECT
		COALESCE(SUM(kind='search'),0), COALESCE(SUM(kind='search' AND results=0 AND error=''),0),
		COALESCE(SUM(kind='reformulation'),0), COALESCE(SUM(kind='open'),0),
		COALESCE(SUM(kind='context'),0), COALESCE(SUM(CASE WHEN kind='context' THEN tokens END),0),
		COALESCE(SUM(CASE WHEN kind='context' THEN results END),0),
		COALESCE(SUM(kind='injected_read'),0), COALESCE(SUM(kind='injected_update'),0),
		COALESCE(SUM(kind='save'),0), COALESCE(SUM(kind='save' AND error<>''),0),
		COALESCE(SUM(kind='save' AND extra='limit'),0)
		FROM events`+where, wargs...).Scan(&sum.Searches, &sum.ZeroResult, &sum.Reformulations, &sum.Opens,
		&sum.ContextEvents, &sum.InjectedTokens, &sum.InjectedItems, &sum.InjectedRead, &sum.InjectedUpdated,
		&sum.Saves, &sum.SaveErrors, &sum.SaveLimit); err != nil {
		return sum, err
	}
	if err := one(&sum.AvgSearchMS, `SELECT COALESCE(AVG(latency_ms),0) FROM events WHERE kind='search'`+andKind, wargs...); err != nil {
		return sum, err
	}
	if err := one(&sum.AvgSaveMS, `SELECT COALESCE(AVG(latency_ms),0) FROM events WHERE kind='save'`+andKind, wargs...); err != nil {
		return sum, err
	}
	rows, err := db.Query(`SELECT ids, COUNT(*) c FROM events WHERE kind='open'`+andKind+` GROUP BY ids ORDER BY c DESC, ids LIMIT 5`, wargs...)
	if err != nil {
		return sum, err
	}
	for rows.Next() {
		var ic IDCount
		if err := rows.Scan(&ic.ID, &ic.Count); err != nil {
			_ = rows.Close()
			return sum, err
		}
		sum.TopOpened = append(sum.TopOpened, ic)
	}
	if err := rows.Close(); err != nil {
		return sum, err
	}
	rows, err = db.Query(`SELECT session, SUM(tokens), SUM(results) FROM events WHERE kind='context'`+andKind+` GROUP BY session ORDER BY MAX(ts) DESC LIMIT 10`, wargs...)
	if err != nil {
		return sum, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var st SessionTokens
		if err := rows.Scan(&st.Session, &st.Tokens, &st.Items); err != nil {
			return sum, err
		}
		sum.PerSession = append(sum.PerSession, st)
	}
	if err := rows.Err(); err != nil {
		return sum, err
	}
	_ = db.QueryRow(`SELECT COALESCE((SELECT value FROM meta WHERE key='dropped'),0), COALESCE((SELECT value FROM meta WHERE key='write_errors'),0)`).Scan(&sum.Dropped, &sum.WriteErrors)
	return sum, nil
}

// Render writes the human-readable summary.
func (s Summary) Render(w io.Writer) {
	pf := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	pf("qbit-jay local telemetry (%s)\n", s.Path)
	if s.Project != "" {
		pf("Project: %s (events recorded without a project are not counted)\n", s.Project)
	}
	if !s.Exists {
		pf("No telemetry recorded yet.\n")
		if !Enabled() {
			pf("Telemetry is off (QBIT_JAY_TELEMETRY).\n")
		}
		return
	}
	pf("Searches: %d (avg %.1f ms)\n", s.Searches, s.AvgSearchMS)
	pf("  zero-result rate:     %.1f%% (%d)\n", 100*s.ZeroRate(), s.ZeroResult)
	pf("  reformulation rate:   %.1f%% (%d within 60 s)\n", 100*s.ReformulationRate(), s.Reformulations)
	pf("  opens after search:   %d (%.1f%% of searches)\n", s.Opens, 100*s.OpenRate())
	if len(s.TopOpened) > 0 {
		parts := make([]string, 0, len(s.TopOpened))
		for _, t := range s.TopOpened {
			parts = append(parts, fmt.Sprintf("#%s x%d", t.ID, t.Count))
		}
		pf("  top opened ids:       %s\n", strings.Join(parts, ", "))
	}
	pf("Context: %d injections, %d items, ~%d tokens\n", s.ContextEvents, s.InjectedItems, s.InjectedTokens)
	pf("  later read: %d, later updated: %d\n", s.InjectedRead, s.InjectedUpdated)
	for _, st := range s.PerSession {
		pf("  session %s: ~%d tokens, %d items\n", st.Session, st.Tokens, st.Items)
	}
	pf("Saves: %d (avg %.1f ms), errors: %d, limit rejections: %d\n", s.Saves, s.AvgSaveMS, s.SaveErrors, s.SaveLimit)
	pf("Telemetry health: dropped events: %d, write errors: %d\n", s.Dropped, s.WriteErrors)
}

// Clear deletes every telemetry row (and the persisted counters) through
// SQLite and compacts the file, so a running MCP process that holds the file
// keeps a valid database. Only when the file is not a usable database (nothing
// can hold it open meaningfully) is it removed instead. It reports whether
// the telemetry file existed and was cleared.
func Clear(dataDir string) (bool, error) {
	path := Path(dataDir)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := clearRows(path); err != nil {
		if isBusy(err) {
			return false, fmt.Errorf("telemetry file is busy, try again: %w", err)
		}
		// Not a usable database: remove it and any leftovers.
		for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
			if rerr := os.Remove(path + suffix); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
				return false, rerr
			}
		}
	}
	return true, nil
}

func isBusy(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "locked") || strings.Contains(m, "busy")
}

func clearRows(path string) error {
	db, err := openDB(path, false)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	for _, q := range []string{`DELETE FROM injected`, `DELETE FROM events`, `DELETE FROM meta`, `DELETE FROM sqlite_sequence WHERE name='events'`} {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	// Compaction is best effort: the rows are already gone, and another
	// process may briefly hold the file.
	_, _ = db.Exec(`VACUUM`)
	return nil
}
