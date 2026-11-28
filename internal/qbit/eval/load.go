//go:build eval

package eval

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// tsFormat is the timestamp layout SQLite's datetime() produces and the store
// writes for sessions, prompts and observations.
const tsFormat = "2006-01-02 15:04:05"

// Loaded is a store filled from a synthetic corpus. The store API stamps every
// row with the wall clock, so a second SQL connection rewrites the timestamps
// to the synthetic ones right after each write. Only the eval package does
// that, and only on databases it created itself.
type Loaded struct {
	Dir   string
	cfg   store.Config
	Store *store.Store
	// IDs maps Memory.Index to the observation id the store assigned.
	IDs []int64
	raw *sql.DB
}

// Open creates an empty store in dir.
func Open(dir string) (*Loaded, error) {
	cfg := store.Config{
		DataDir:              dir,
		MaxObservationLength: 50000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         15 * time.Minute,
	}
	s, err := store.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	raw, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, store.DatabaseFile))+"?_pragma=busy_timeout(60000)")
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("open patch connection: %w", err)
	}
	raw.SetMaxOpenConns(1)
	return &Loaded{Dir: dir, cfg: cfg, Store: s, raw: raw}, nil
}

// The store's save path looks up the topic_key and the duplicate hash with
// predicates the SQLite planner answers by scanning every row of the scope
// (idx_obs_scope) when the database has no statistics, so loading N memories
// through the store costs O(N^2) (2k rows: 16 s, 8k rows: 97 s). ANALYZE while
// loading lets the planner use the selective indexes; the statistics are
// dropped again at the end (dropStats), because a real database has none and
// search and context must be measured against that.
func (l *Loaded) analyze() error {
	_, err := l.raw.Exec("ANALYZE")
	return err
}

// dropStats removes the planner statistics and reopens both connections so
// neither keeps them cached.
func (l *Loaded) dropStats() error {
	if _, err := l.raw.Exec("DELETE FROM sqlite_stat1"); err != nil {
		return err
	}
	if err := l.Close(); err != nil {
		return err
	}
	fresh, err := Open(l.Dir)
	if err != nil {
		return err
	}
	l.Store, l.raw = fresh.Store, fresh.raw
	return nil
}

// Close releases both connections.
func (l *Loaded) Close() error {
	rerr := l.raw.Close()
	if err := l.Store.Close(); err != nil {
		return err
	}
	return rerr
}

// DBPath is the SQLite file of the store.
func (l *Loaded) DBPath() string { return filepath.Join(l.Dir, store.DatabaseFile) }

// DBBytes is the size of the database file plus its WAL, if any.
func (l *Loaded) DBBytes() int64 {
	var total int64
	for _, suffix := range []string{"", "-wal"} {
		if fi, err := os.Stat(l.DBPath() + suffix); err == nil {
			total += fi.Size()
		}
	}
	return total
}

func ts(t time.Time) string { return t.UTC().Format(tsFormat) }

func obsParams(m Memory, session string, content string) store.AddObservationParams {
	return store.AddObservationParams{
		SessionID: session, Type: m.Type, Title: m.Title, Content: content,
		Project: m.Project, Scope: "project", TopicKey: m.TopicKey,
	}
}

// LoadFlat stores every memory of c, one session per project, with creation
// times spread evenly over six months. Used by E2 and E3.
func (l *Loaded) LoadFlat(c *Corpus) error {
	l.IDs = make([]int64, len(c.Memories))
	seen := map[string]bool{}
	for _, m := range c.Memories {
		sid := "flat-" + m.Project
		if !seen[sid] {
			seen[sid] = true
			if err := l.Store.CreateSession(sid, m.Project, "/synthetic/"+m.Project); err != nil {
				return fmt.Errorf("create session: %w", err)
			}
		}
		id, err := l.Store.AddObservation(obsParams(m, sid, m.Content))
		if err != nil {
			return fmt.Errorf("add %s: %w", m.Key, err)
		}
		l.IDs[m.Index] = id
		if m.Index == 500 || (m.Index > 0 && m.Index%20000 == 0) {
			if err := l.analyze(); err != nil {
				return err
			}
		}
	}
	if err := l.patchCreated(c); err != nil {
		return err
	}
	return l.dropStats()
}

func (l *Loaded) patchCreated(c *Corpus) error {
	tx, err := l.raw.Begin()
	if err != nil {
		return err
	}
	st, err := tx.Prepare(`UPDATE observations SET created_at = ?1, updated_at = ?1, last_seen_at = ?1 WHERE id = ?2`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer func() { _ = st.Close() }()
	for _, m := range c.Memories {
		if _, err := st.Exec(ts(m.CreatedAt), l.IDs[m.Index]); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

type eventKind int

// Kinds are ordered so that, at equal times, a session starts before anything
// happens in it and ends after.
const (
	evStart eventKind = iota
	evPrompt
	evSave
	evRevise
	evEnd
)

type event struct {
	at   time.Time
	kind eventKind
	plan int
	n    int // index into the plan's Prompts / Saves / Revises
}

// Replay applies the plans in chronological order, so the database passes
// through every state it had in the synthetic world. before is called with a
// plan just before its session is created: that is the instant a session would
// ask for its context. Memory.CreatedAt is ignored; times come from the plans.
func (l *Loaded) Replay(c *Corpus, plans []SessionPlan, before func(SessionPlan) error) error {
	var evs []event
	for pi, p := range plans {
		evs = append(evs, event{p.Start, evStart, pi, 0})
		for i, x := range p.Prompts {
			evs = append(evs, event{x.At, evPrompt, pi, i})
		}
		for i, x := range p.Saves {
			evs = append(evs, event{x.At, evSave, pi, i})
		}
		for i, x := range p.Revises {
			evs = append(evs, event{x.At, evRevise, pi, i})
		}
		if p.Ended {
			evs = append(evs, event{p.End, evEnd, pi, 0})
		}
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if !evs[i].at.Equal(evs[j].at) {
			return evs[i].at.Before(evs[j].at)
		}
		return evs[i].kind < evs[j].kind
	})
	l.IDs = make([]int64, len(c.Memories))
	for i, e := range evs {
		if i == 500 || (i > 0 && i%10000 == 0) {
			if err := l.analyze(); err != nil {
				return err
			}
		}
		p := plans[e.plan]
		if err := l.apply(c, p, e, before); err != nil {
			return fmt.Errorf("session %s: %w", p.ID, err)
		}
	}
	return l.dropStats()
}

func (l *Loaded) apply(c *Corpus, p SessionPlan, e event, before func(SessionPlan) error) error {
	switch e.kind {
	case evStart:
		if before != nil {
			if err := before(p); err != nil {
				return err
			}
		}
		if err := l.Store.CreateSession(p.ID, p.Project, "/synthetic/"+p.Project); err != nil {
			return err
		}
		_, err := l.raw.Exec(`UPDATE sessions SET started_at = ? WHERE id = ?`, ts(p.Start), p.ID)
		return err
	case evPrompt:
		pr := p.Prompts[e.n]
		id, err := l.Store.AddPrompt(store.AddPromptParams{SessionID: p.ID, Content: pr.Text, Project: p.Project})
		if err != nil {
			return err
		}
		_, err = l.raw.Exec(`UPDATE user_prompts SET created_at = ? WHERE id = ?`, ts(pr.At), id)
		return err
	case evSave:
		sv := p.Saves[e.n]
		m := c.Memories[sv.Mem]
		id, err := l.Store.AddObservation(obsParams(m, p.ID, m.Content))
		if err != nil {
			return err
		}
		l.IDs[m.Index] = id
		_, err = l.raw.Exec(`UPDATE observations SET created_at = ?1, updated_at = ?1, last_seen_at = ?1 WHERE id = ?2`, ts(sv.At), id)
		return err
	case evRevise:
		rv := p.Revises[e.n]
		m := c.Memories[rv.Mem]
		id, err := l.Store.AddObservation(obsParams(m, p.ID, rv.Content))
		if err != nil {
			return err
		}
		if id != l.IDs[m.Index] {
			return fmt.Errorf("revision of %s created observation %d instead of updating %d", m.Key, id, l.IDs[m.Index])
		}
		if _, err := l.raw.Exec(`UPDATE observations SET updated_at = ?1, last_seen_at = ?1 WHERE id = ?2`, ts(rv.At), id); err != nil {
			return err
		}
		_, err = l.raw.Exec(`UPDATE observation_versions SET created_at = ?1
			WHERE observation_id = ?2 AND version = (SELECT MAX(version) FROM observation_versions WHERE observation_id = ?2)`, ts(rv.At), id)
		return err
	case evEnd:
		if err := l.Store.EndSession(p.ID, p.Summary); err != nil {
			return err
		}
		_, err := l.raw.Exec(`UPDATE sessions SET ended_at = ? WHERE id = ?`, ts(p.End), p.ID)
		return err
	}
	return nil
}

// RawDB exposes the patch connection for read-only audit queries.
func (l *Loaded) RawDB() *sql.DB { return l.raw }

func dbFile() string { return store.DatabaseFile }
