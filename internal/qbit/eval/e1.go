//go:build eval

package eval

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// E1 reproduces the audit method (spec 13.1, H4):
//
//   - sessions with a recorded end, with prompts, and with no other session
//     overlapping them in the same project;
//   - a memory is "needed" by a session when it was created before the session
//     started and has a version created during it;
//   - the context the session would receive at its start is simulated and
//     compared against the needed memories.
//
// On a synthetic corpus the context comes from the real store function
// (FormatContextWithOptions, the one mem_context calls) on a database replayed
// to the state it had at each session start. On a real database copy
// (QBIT_JAY_EVAL_DB) the store has no "as of" parameter, so the recency
// strategy is mirrored in SQL; see RunE1Real.

// E1Budgets are the context budgets measured, in bytes. 0 means unbounded.
// 16384 is the mem_context default.
var E1Budgets = []int{4096, 16384, 0}

// AuditSession is one session that passes the audit filters.
type AuditSession struct {
	ID      string
	Project string
	Start   string
	End     string
	Needed  []NeededMem
}

// NeededMem is a memory a session revisited.
type NeededMem struct {
	ID    int64
	Title string
}

// AuditSessions applies the audit filters to any database with the store
// schema (read-only use) and returns only sessions that need at least one
// earlier memory, as the audit did.
func AuditSessions(db *sql.DB) ([]AuditSession, error) {
	rows, err := db.Query(`
		SELECT s.id, s.project, s.started_at, s.ended_at
		FROM sessions s
		WHERE s.ended_at IS NOT NULL
		  AND EXISTS (SELECT 1 FROM user_prompts p WHERE p.session_id = s.id)
		  AND NOT EXISTS (
		        SELECT 1 FROM sessions o
		        WHERE o.project = s.project AND o.id <> s.id
		          AND o.started_at < s.ended_at
		          AND COALESCE(o.ended_at, o.started_at) > s.started_at)
		ORDER BY s.started_at, s.id`)
	if err != nil {
		return nil, fmt.Errorf("audit sessions: %w", err)
	}
	var cand []AuditSession
	for rows.Next() {
		var a AuditSession
		if err := rows.Scan(&a.ID, &a.Project, &a.Start, &a.End); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cand = append(cand, a)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []AuditSession
	for _, a := range cand {
		nr, err := db.Query(`
			SELECT DISTINCT o.id, o.title
			FROM observation_versions v JOIN observations o ON o.id = v.observation_id
			WHERE v.created_at >= ?1 AND v.created_at <= ?2
			  AND o.created_at < ?1 AND o.deleted_at IS NULL
			  AND LOWER(o.project) = LOWER(?3)
			ORDER BY o.id`, a.Start, a.End, a.Project)
		if err != nil {
			return nil, fmt.Errorf("needed memories: %w", err)
		}
		for nr.Next() {
			var n NeededMem
			if err := nr.Scan(&n.ID, &n.Title); err != nil {
				_ = nr.Close()
				return nil, err
			}
			a.Needed = append(a.Needed, n)
		}
		if err := nr.Close(); err != nil {
			return nil, err
		}
		if len(a.Needed) > 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

// E1Row is the outcome for one budget.
type E1Row struct {
	Budget    int     `json:"budget_bytes"`
	Hit       float64 `json:"hit_rate"`
	Recall    float64 `json:"recall_within_budget"`
	MeanBytes float64 `json:"mean_bytes"`
}

// E1Result is the outcome of E1 on one corpus.
type E1Result struct {
	Source       string  `json:"source"`
	Size         int     `json:"size"`
	Strategy     string  `json:"strategy"`
	Sessions     int     `json:"sessions"`
	MeanNeeded   float64 `json:"mean_needed_per_session"`
	Rows         []E1Row `json:"rows"`
	LoadSeconds  float64 `json:"load_seconds"`
	PlannedCount int     `json:"planned_sessions"`
}

type acc struct {
	hits, n  int
	recall   float64
	bytesSum int
}

func (a *acc) add(found, needed, bytes int) {
	a.n++
	if found > 0 {
		a.hits++
	}
	a.recall += float64(found) / float64(needed)
	a.bytesSum += bytes
}

func (a *acc) row(budget int) E1Row {
	if a.n == 0 {
		return E1Row{Budget: budget}
	}
	return E1Row{Budget: budget, Hit: float64(a.hits) / float64(a.n),
		Recall: a.recall / float64(a.n), MeanBytes: float64(a.bytesSum) / float64(a.n)}
}

// RunE1 replays a synthetic corpus into a fresh store under dir and measures
// the real context function at the start of every audited session.
func RunE1(c *Corpus, dir string) (E1Result, error) {
	plans := Plan(c)
	l, err := Open(dir)
	if err != nil {
		return E1Result{}, err
	}
	defer func() { _ = l.Close() }()

	// Keep contexts only for sessions that can pass the audit.
	contexts := map[string]map[int]string{}
	probe := func(p SessionPlan) error {
		if !p.Ended || len(p.Prompts) == 0 || len(p.Revises) == 0 {
			return nil
		}
		byBudget := map[int]string{}
		for _, b := range E1Budgets {
			out, err := l.Store.FormatContextWithOptions(p.Project, "", store.ContextOptions{MaxBytes: b, Pinned: 20})
			if err != nil {
				return err
			}
			byBudget[b] = out
		}
		contexts[p.ID] = byBudget
		return nil
	}
	if err := l.Replay(c, plans, probe); err != nil {
		return E1Result{}, err
	}
	audited, err := AuditSessions(l.RawDB())
	if err != nil {
		return E1Result{}, err
	}
	accs := map[int]*acc{}
	for _, b := range E1Budgets {
		accs[b] = &acc{}
	}
	needed := 0
	for _, a := range audited {
		ctxs, ok := contexts[a.ID]
		if !ok {
			return E1Result{}, fmt.Errorf("audit selected session %s but the replay took no context for it", a.ID)
		}
		needed += len(a.Needed)
		for _, b := range E1Budgets {
			found := 0
			for _, n := range a.Needed {
				if strings.Contains(ctxs[b], "**"+n.Title+"**") {
					found++
				}
			}
			accs[b].add(found, len(a.Needed), len(ctxs[b]))
		}
	}
	res := E1Result{
		Source: "synthetic", Size: len(c.Memories), Sessions: len(audited), PlannedCount: len(plans),
		Strategy: "store.FormatContextWithOptions (recency), Pinned=20",
	}
	if len(audited) > 0 {
		res.MeanNeeded = float64(needed) / float64(len(audited))
	}
	for _, b := range E1Budgets {
		res.Rows = append(res.Rows, accs[b].row(b))
	}
	return res, nil
}

// RunE1Real audits a read-only copy of a real database (path from
// QBIT_JAY_EVAL_DB). The store has no "as of" parameter, so the recency
// strategy is mirrored in SQL: the 20 newest unpinned live memories of the
// project created before the session started, rendered as the store renders
// them. Only that section is counted, not the sessions and prompts sections,
// so recall within a budget is an upper bound. Never used in tests or CI.
func RunE1Real(path string) (E1Result, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&immutable=1")
	if err != nil {
		return E1Result{}, err
	}
	defer func() { _ = db.Close() }()
	audited, err := AuditSessions(db)
	if err != nil {
		return E1Result{}, err
	}
	accs := map[int]*acc{}
	for _, b := range E1Budgets {
		accs[b] = &acc{}
	}
	needed := 0
	for _, a := range audited {
		rows, err := db.Query(`
			SELECT id, type, title, content FROM observations
			WHERE deleted_at IS NULL AND pinned = 0 AND LOWER(project) = LOWER(?1)
			  AND datetime(created_at) < datetime(?2)
			ORDER BY datetime(created_at) DESC, id DESC LIMIT 20`, a.Project, a.Start)
		if err != nil {
			return E1Result{}, err
		}
		type bullet struct {
			id   int64
			size int
		}
		var bullets []bullet
		for rows.Next() {
			var id int64
			var typ, title, content string
			if err := rows.Scan(&id, &typ, &title, &content); err != nil {
				_ = rows.Close()
				return E1Result{}, err
			}
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			bullets = append(bullets, bullet{id, len(fmt.Sprintf("- [%s] **%s**: %s\n", typ, title, content))})
		}
		if err := rows.Close(); err != nil {
			return E1Result{}, err
		}
		needed += len(a.Needed)
		const header = len("## Memory from Previous Sessions\n\n### Recent Observations\n")
		for _, b := range E1Budgets {
			used, found := header, 0
			inCtx := map[int64]bool{}
			for _, bu := range bullets {
				if b > 0 && used+bu.size > b {
					break
				}
				used += bu.size
				inCtx[bu.id] = true
			}
			for _, n := range a.Needed {
				if inCtx[n.ID] {
					found++
				}
			}
			accs[b].add(found, len(a.Needed), used)
		}
	}
	res := E1Result{
		Source: "QBIT_JAY_EVAL_DB", Sessions: len(audited),
		Strategy: "recency-20 mirrored in SQL (observations section only)",
	}
	if len(audited) > 0 {
		res.MeanNeeded = float64(needed) / float64(len(audited))
	}
	for _, b := range E1Budgets {
		res.Rows = append(res.Rows, accs[b].row(b))
	}
	return res, nil
}
