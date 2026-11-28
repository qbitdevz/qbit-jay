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

// Load summarizes the telemetry database of dataDir. A missing file is not an
// error: it yields an empty summary with Exists=false. Load never creates the
// file.
func Load(dataDir string) (Summary, error) {
	path := Path(dataDir)
	sum := Summary{Path: path}
	db, err := openDB(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return sum, nil
	}
	if err != nil {
		return sum, err
	}
	defer db.Close()
	sum.Exists = true

	one := func(dst any, q string, args ...any) error { return db.QueryRow(q, args...).Scan(dst) }
	if err := db.QueryRow(`SELECT
		COALESCE(SUM(kind='search'),0), COALESCE(SUM(kind='search' AND results=0 AND error=''),0),
		COALESCE(SUM(kind='reformulation'),0), COALESCE(SUM(kind='open'),0),
		COALESCE(SUM(kind='context'),0), COALESCE(SUM(CASE WHEN kind='context' THEN tokens END),0),
		COALESCE(SUM(CASE WHEN kind='context' THEN results END),0),
		COALESCE(SUM(kind='injected_read'),0), COALESCE(SUM(kind='injected_update'),0),
		COALESCE(SUM(kind='save'),0), COALESCE(SUM(kind='save' AND error<>''),0),
		COALESCE(SUM(kind='save' AND extra='limit'),0)
		FROM events`).Scan(&sum.Searches, &sum.ZeroResult, &sum.Reformulations, &sum.Opens,
		&sum.ContextEvents, &sum.InjectedTokens, &sum.InjectedItems, &sum.InjectedRead, &sum.InjectedUpdated,
		&sum.Saves, &sum.SaveErrors, &sum.SaveLimit); err != nil {
		return sum, err
	}
	if err := one(&sum.AvgSearchMS, `SELECT COALESCE(AVG(latency_ms),0) FROM events WHERE kind='search'`); err != nil {
		return sum, err
	}
	if err := one(&sum.AvgSaveMS, `SELECT COALESCE(AVG(latency_ms),0) FROM events WHERE kind='save'`); err != nil {
		return sum, err
	}
	rows, err := db.Query(`SELECT ids, COUNT(*) c FROM events WHERE kind='open' GROUP BY ids ORDER BY c DESC, ids LIMIT 5`)
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
	rows, err = db.Query(`SELECT session, SUM(tokens), SUM(results) FROM events WHERE kind='context' GROUP BY session ORDER BY MAX(ts) DESC LIMIT 10`)
	if err != nil {
		return sum, err
	}
	defer rows.Close()
	for rows.Next() {
		var st SessionTokens
		if err := rows.Scan(&st.Session, &st.Tokens, &st.Items); err != nil {
			return sum, err
		}
		sum.PerSession = append(sum.PerSession, st)
	}
	return sum, rows.Err()
}

// Render writes the human-readable summary.
func (s Summary) Render(w io.Writer) {
	fmt.Fprintf(w, "qbit-jay local telemetry (%s)\n", s.Path)
	if !s.Exists {
		fmt.Fprintln(w, "No telemetry recorded yet.")
		if !Enabled() {
			fmt.Fprintln(w, "Telemetry is off (QBIT_JAY_TELEMETRY).")
		}
		return
	}
	fmt.Fprintf(w, "Searches: %d (avg %.1f ms)\n", s.Searches, s.AvgSearchMS)
	fmt.Fprintf(w, "  zero-result rate:     %.1f%% (%d)\n", 100*s.ZeroRate(), s.ZeroResult)
	fmt.Fprintf(w, "  reformulation rate:   %.1f%% (%d within 60 s)\n", 100*s.ReformulationRate(), s.Reformulations)
	fmt.Fprintf(w, "  opens after search:   %d (%.1f%% of searches)\n", s.Opens, 100*s.OpenRate())
	if len(s.TopOpened) > 0 {
		parts := make([]string, 0, len(s.TopOpened))
		for _, t := range s.TopOpened {
			parts = append(parts, fmt.Sprintf("#%s x%d", t.ID, t.Count))
		}
		fmt.Fprintf(w, "  top opened ids:       %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(w, "Context: %d injections, %d items, ~%d tokens\n", s.ContextEvents, s.InjectedItems, s.InjectedTokens)
	fmt.Fprintf(w, "  later read: %d, later updated: %d\n", s.InjectedRead, s.InjectedUpdated)
	for _, st := range s.PerSession {
		fmt.Fprintf(w, "  session %s: ~%d tokens, %d items\n", st.Session, st.Tokens, st.Items)
	}
	fmt.Fprintf(w, "Saves: %d (avg %.1f ms), errors: %d, limit rejections: %d\n", s.Saves, s.AvgSaveMS, s.SaveErrors, s.SaveLimit)
}

// Clear deletes the telemetry database and returns whether a file was removed.
func Clear(dataDir string) (bool, error) {
	removed := false
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		err := os.Remove(Path(dataDir) + suffix)
		if err == nil {
			removed = removed || suffix == ""
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
	}
	return removed, nil
}
