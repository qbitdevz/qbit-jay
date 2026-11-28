//go:build eval

package eval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// SearchCase is one query of the E3 search workload.
type SearchCase struct {
	Project string
	Query   string
	Mode    string
}

// SearchWorkload builds n deterministic queries of one to three words taken
// from memory titles (the words of a title always hit that memory); a quarter
// run with match_mode=any, like the e3latency workload.
func SearchWorkload(c *Corpus, n int) []SearchCase {
	r := newRand(c.Seed, 31337)
	out := make([]SearchCase, 0, n)
	for len(out) < n {
		m := c.Memories[r.IntN(len(c.Memories))]
		var words []string
		for _, w := range strings.Fields(strings.ToLower(m.Title)) {
			if len(w) >= 4 && !strings.HasPrefix(w, "(m") {
				words = append(words, strings.Trim(w, ":,.;"))
			}
		}
		if len(words) == 0 {
			continue
		}
		k := min(1+r.IntN(3), len(words))
		start := r.IntN(len(words) - k + 1)
		mode := "all"
		if r.IntN(4) == 0 {
			mode = "any"
		}
		out = append(out, SearchCase{Project: m.Project, Query: strings.Join(words[start:start+k], " "), Mode: mode})
	}
	return out
}

// E3Result is the outcome of the E3 workload on one corpus size.
type E3Result struct {
	Size         int     `json:"size"`
	LoadSeconds  float64 `json:"load_seconds"`
	DBBytes      int64   `json:"db_bytes"`
	Searches     int     `json:"searches"`
	SearchP50ms  float64 `json:"search_p50_ms"`
	SearchP95ms  float64 `json:"search_p95_ms"`
	Contexts     int     `json:"contexts"`
	ContextP50ms float64 `json:"context_p50_ms"`
	ContextP95ms float64 `json:"context_p95_ms"`
}

func percentile(d []time.Duration, p float64) float64 {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(float64(len(s)-1) * p)
	return float64(s[idx].Microseconds()) / 1000
}

// RunE3 measures search and context latency against a loaded corpus. It goes
// through the store, not MCP; end-to-end latency, startup and RSS are measured
// by scripts/qbit/e3latency. Load time is passed in by the caller.
func RunE3(l *Loaded, c *Corpus, searches, contexts int) (E3Result, error) {
	res := E3Result{Size: len(c.Memories), DBBytes: l.DBBytes(), Searches: searches, Contexts: contexts}
	var sd []time.Duration
	for _, q := range SearchWorkload(c, searches) {
		t := time.Now()
		if _, err := l.Store.SearchPreviewsContext(context.Background(), q.Query, store.SearchOptions{
			Project: q.Project, Limit: 10, MatchMode: q.Mode}); err != nil {
			return res, fmt.Errorf("search %q: %w", q.Query, err)
		}
		sd = append(sd, time.Since(t))
	}
	var cd []time.Duration
	for i := range contexts {
		p := projects[i%len(projects)].name
		t := time.Now()
		if _, err := l.Store.FormatContextWithOptions(p, "", store.ContextOptions{MaxBytes: 16384, Pinned: 20}); err != nil {
			return res, fmt.Errorf("context %s: %w", p, err)
		}
		cd = append(cd, time.Since(t))
	}
	res.SearchP50ms, res.SearchP95ms = percentile(sd, 0.50), percentile(sd, 0.95)
	res.ContextP50ms, res.ContextP95ms = percentile(cd, 0.50), percentile(cd, 0.95)
	return res, nil
}

func searchOpts(q SearchCase) store.SearchOptions {
	return store.SearchOptions{Project: q.Project, Limit: 10, MatchMode: q.Mode}
}

func contextOpts() store.ContextOptions {
	return store.ContextOptions{MaxBytes: 16384, Pinned: 20}
}
