//go:build eval

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// FixturePath is the E2 query fixture, relative to this package.
var FixturePath = filepath.Join("testdata", "e2_queries.json")

// Query kinds in the fixture.
//
//	same:  a natural-language paraphrase in the memory's own language
//	kw:    two or three keywords in the memory's own language
//	cross: a natural-language query in the OTHER language (Spanish query
//	       against an English memory, or the reverse)
const (
	KindSame  = "same"
	KindKW    = "kw"
	KindCross = "cross"
)

// FixtureQuery is one synthetic query about one memory.
type FixtureQuery struct {
	Mem   string `json:"mem"`  // Memory.Key
	Lang  string `json:"lang"` // language of the query
	Kind  string `json:"kind"` // same | kw | cross
	Query string `json:"query"`
}

// Fixture is the E2 ground truth: queries generated once by an LLM from the
// synthetic memories, stored in the repository.
type Fixture struct {
	Seed        uint64         `json:"seed"`
	Memories    int            `json:"memories"` // queries target memories m00000..m(Memories-1)
	Fingerprint string         `json:"corpus_fingerprint"`
	Queries     []FixtureQuery `json:"queries"`
}

// LoadFixture reads the fixture and checks it still matches the generator.
func LoadFixture(c *Corpus) (*Fixture, error) {
	data, err := os.ReadFile(FixturePath)
	if err != nil {
		return nil, err
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FixturePath, err)
	}
	if f.Seed != c.Seed {
		return nil, fmt.Errorf("fixture seed %d does not match corpus seed %d", f.Seed, c.Seed)
	}
	if len(c.Memories) < f.Memories {
		return nil, fmt.Errorf("corpus has %d memories, the fixture needs %d", len(c.Memories), f.Memories)
	}
	if got := c.Fingerprint(f.Memories); got != f.Fingerprint {
		return nil, fmt.Errorf("the corpus generator changed: fingerprint %s != fixture %s; the fixture queries must be regenerated (see testdata/README.md)", got, f.Fingerprint)
	}
	return &f, nil
}

// E2Metrics are the retrieval metrics of a group of queries.
type E2Metrics struct {
	Queries    int     `json:"queries"`
	RecallAt5  float64 `json:"recall_at_5"`
	RecallAt10 float64 `json:"recall_at_10"`
	MRR        float64 `json:"mrr"`
	ZeroResult float64 `json:"zero_result_rate"`
}

// E2Result is the outcome of E2 on one corpus size and match mode.
type E2Result struct {
	Size      int                  `json:"size"`
	MatchMode string               `json:"match_mode"`
	Overall   E2Metrics            `json:"overall"`
	ByKind    map[string]E2Metrics `json:"by_kind"`
	// CrossLanguage are the Spanish queries against English memories and the
	// English queries against Spanish memories, separately.
	ByDirection map[string]E2Metrics `json:"by_direction"`
}

type e2acc struct {
	n, r5, r10, zero int
	rr               float64
}

func (a *e2acc) add(rank, nres int) {
	a.n++
	if nres == 0 {
		a.zero++
	}
	if rank > 0 {
		a.rr += 1 / float64(rank)
		if rank <= 5 {
			a.r5++
		}
		if rank <= 10 {
			a.r10++
		}
	}
}

func (a *e2acc) metrics() E2Metrics {
	if a.n == 0 {
		return E2Metrics{}
	}
	n := float64(a.n)
	return E2Metrics{Queries: a.n, RecallAt5: float64(a.r5) / n, RecallAt10: float64(a.r10) / n,
		MRR: a.rr / n, ZeroResult: float64(a.zero) / n}
}

// RunE2 runs every fixture query through the same search the mem_search tool
// uses (Store.SearchPreviewsContext), restricted to the memory's project, with
// limit 10, and scores where the target memory lands.
func RunE2(l *Loaded, c *Corpus, f *Fixture, matchMode string) (E2Result, error) {
	overall := &e2acc{}
	byKind := map[string]*e2acc{}
	byDir := map[string]*e2acc{}
	for _, q := range f.Queries {
		var target Memory
		found := false
		for _, m := range c.Memories[:f.Memories] {
			if m.Key == q.Mem {
				target, found = m, true
				break
			}
		}
		if !found {
			return E2Result{}, fmt.Errorf("fixture names unknown memory %s", q.Mem)
		}
		res, err := l.Store.SearchPreviewsContext(context.Background(), q.Query, store.SearchOptions{
			Project: target.Project, Limit: 10, MatchMode: matchMode,
		})
		if err != nil {
			return E2Result{}, fmt.Errorf("search %q: %w", q.Query, err)
		}
		rank := 0
		for i, r := range res {
			if r.ID == l.IDs[target.Index] {
				rank = i + 1
				break
			}
		}
		overall.add(rank, len(res))
		get := func(m map[string]*e2acc, k string) *e2acc {
			if m[k] == nil {
				m[k] = &e2acc{}
			}
			return m[k]
		}
		get(byKind, q.Kind).add(rank, len(res))
		get(byDir, q.Lang+"_query_vs_"+target.Lang+"_memory").add(rank, len(res))
	}
	out := E2Result{Size: len(c.Memories), MatchMode: matchMode, Overall: overall.metrics(),
		ByKind: map[string]E2Metrics{}, ByDirection: map[string]E2Metrics{}}
	for k, a := range byKind {
		out.ByKind[k] = a.metrics()
	}
	for k, a := range byDir {
		out.ByDirection[k] = a.metrics()
	}
	return out, nil
}

// SortedKeys returns the keys of m in order, for stable reports.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// e2Memories is how many memories the E2 fixture covers.
const e2Memories = 150
