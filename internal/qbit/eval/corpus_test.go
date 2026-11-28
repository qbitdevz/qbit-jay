//go:build eval

package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateIsDeterministicAndSizeIndependent(t *testing.T) {
	a, b := Generate(DefaultSeed, 300), Generate(DefaultSeed, 1000)
	if a.Fingerprint(300) != Generate(DefaultSeed, 300).Fingerprint(300) {
		t.Fatal("same seed and size gave different corpora")
	}
	if a.Fingerprint(300) != b.Fingerprint(300) {
		t.Fatal("the first 300 memories depend on the corpus size")
	}
	if Generate(DefaultSeed+1, 300).Fingerprint(300) == a.Fingerprint(300) {
		t.Fatal("a different seed gave the same corpus")
	}
}

func TestGenerateShape(t *testing.T) {
	c := Generate(DefaultSeed, 2000)
	titles := map[string]bool{}
	langs := map[string]int{}
	topicKeys := map[string]bool{}
	var odd, withKey, long int
	for _, m := range c.Memories {
		if titles[m.Title] {
			t.Fatalf("duplicate title %q", m.Title)
		}
		titles[m.Title] = true
		langs[m.Lang]++
		if m.TopicKey != "" {
			withKey++
			if topicKeys[m.Project+m.TopicKey] {
				t.Fatalf("duplicate topic_key %q in %s", m.TopicKey, m.Project)
			}
			topicKeys[m.Project+m.TopicKey] = true
		}
		if m.ODD {
			odd++
			if !strings.HasPrefix(m.TopicKey, "odd/") {
				t.Fatalf("ODD mirror without odd/ topic_key: %q", m.TopicKey)
			}
			if len(m.Content) > 1000 {
				long++
			}
		}
		if strings.Contains(m.Content, "@") {
			t.Fatalf("content looks like it holds an address: %q", m.Content)
		}
	}
	if langs["en"] < 400 || langs["es"] < 400 {
		t.Fatalf("language mix too skewed: %v", langs)
	}
	if odd < 50 || long != odd || withKey < 800 {
		t.Fatalf("odd=%d long=%d withKey=%d", odd, long, withKey)
	}
}

func TestPlanSessionsNeverOverlapWithinAProject(t *testing.T) {
	c := Generate(DefaultSeed, 1500)
	plans := Plan(c)
	last := map[string]SessionPlan{}
	revised := 0
	created := map[int]bool{}
	for _, p := range plans {
		if prev, ok := last[p.Project]; ok && !p.Start.After(prev.End) {
			t.Fatalf("session %s starts before %s ends", p.ID, prev.ID)
		}
		last[p.Project] = p
		for _, r := range p.Revises {
			revised++
			if !created[r.Mem] {
				t.Fatalf("session %s revises memory %d before it was created", p.ID, r.Mem)
			}
			if c.Memories[r.Mem].TopicKey == "" {
				t.Fatalf("session %s revises a memory without topic_key", p.ID)
			}
		}
		for _, s := range p.Saves {
			created[s.Mem] = true
		}
	}
	if revised < 100 {
		t.Fatalf("only %d revisions in the timeline", revised)
	}
}

func TestE1ReplayAndRealMirrorAgree(t *testing.T) {
	dir := t.TempDir()
	c := Generate(DefaultSeed, 1200)
	res, err := RunE1(c, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sessions < 20 {
		t.Fatalf("only %d audited sessions", res.Sessions)
	}
	// Unbounded budget: the mirror sees the same recency-20 section as the real
	// function, so both must agree on the observations they include.
	mirror, err := RunE1Real(filepath.Join(dir, dbFile()))
	if err != nil {
		t.Fatal(err)
	}
	var a, b E1Row
	for _, r := range res.Rows {
		if r.Budget == 0 {
			a = r
		}
	}
	for _, r := range mirror.Rows {
		if r.Budget == 0 {
			b = r
		}
	}
	if mirror.Sessions != res.Sessions || a.Hit != b.Hit || a.Recall != b.Recall {
		t.Fatalf("replay %+v (%d sessions) != mirror %+v (%d sessions)", a, res.Sessions, b, mirror.Sessions)
	}
	// A larger budget never retrieves less.
	if res.Rows[0].Recall > res.Rows[1].Recall+1e-9 || res.Rows[1].Recall > res.Rows[2].Recall+1e-9 {
		t.Fatalf("recall not monotonic in budget: %+v", res.Rows)
	}
}

func TestFixtureMatchesGenerator(t *testing.T) {
	c := Generate(DefaultSeed, 1000)
	f, err := LoadFixture(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Queries) < 300 {
		t.Fatalf("fixture has only %d queries", len(f.Queries))
	}
	kinds := map[string]int{}
	byKey := map[string]Memory{}
	for _, m := range c.Memories[:f.Memories] {
		byKey[m.Key] = m
	}
	perMem := map[string]int{}
	for _, q := range f.Queries {
		m, ok := byKey[q.Mem]
		if !ok || strings.TrimSpace(q.Query) == "" {
			t.Fatalf("bad fixture query %+v", q)
		}
		kinds[q.Kind]++
		perMem[q.Mem]++
		if q.Kind == KindCross && q.Lang == m.Lang {
			t.Fatalf("cross query in the memory's language: %+v", q)
		}
		if q.Kind != KindCross && q.Lang != m.Lang {
			t.Fatalf("%s query not in the memory's language: %+v", q.Kind, q)
		}
	}
	if len(perMem) != f.Memories {
		t.Fatalf("%d of %d memories have queries", len(perMem), f.Memories)
	}
	if kinds[KindSame] == 0 || kinds[KindKW] == 0 || kinds[KindCross] == 0 {
		t.Fatalf("missing kinds: %v", kinds)
	}
}

func TestE2SmallCorpus(t *testing.T) {
	e := flat(t, 1000)
	f, err := LoadFixture(e.c)
	if err != nil {
		t.Fatal(err)
	}
	r, err := RunE2(e.l, e.c, f, "all")
	if err != nil {
		t.Fatal(err)
	}
	if r.Overall.Queries != len(f.Queries) {
		t.Fatalf("ran %d of %d queries", r.Overall.Queries, len(f.Queries))
	}
	if r.Overall.RecallAt5 > r.Overall.RecallAt10 {
		t.Fatalf("recall@5 above recall@10: %+v", r.Overall)
	}
	// Keywords in the memory's own language must mostly work on a 1k corpus.
	if kw := r.ByKind[KindKW]; kw.RecallAt10 < 0.5 {
		t.Fatalf("kw recall@10 only %.2f: %+v", kw.RecallAt10, r)
	}
}

// TestDumpE2Memories writes the memories the fixture queries are written from.
// Only for regenerating the fixture:
// QBIT_JAY_EVAL_DUMP=<file> go test -tags eval -run TestDumpE2Memories.
func TestDumpE2Memories(t *testing.T) {
	path := os.Getenv("QBIT_JAY_EVAL_DUMP")
	if path == "" {
		t.Skip("set QBIT_JAY_EVAL_DUMP to a file path")
	}
	c := Generate(DefaultSeed, e2Memories)
	var b strings.Builder
	for _, m := range c.Memories {
		content := m.Content
		if m.ODD {
			content = content[:min(len(content), 260)]
		}
		fmt.Fprintf(&b, "%s|%s|%s|%s\n%s\n", m.Key, m.Lang, m.Project, m.Title, strings.ReplaceAll(content, "\n", " / "))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
