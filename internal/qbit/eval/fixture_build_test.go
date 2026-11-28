//go:build eval

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestBuildFixture turns the hand-authored query list into testdata/e2_queries.json.
// Only for regenerating the fixture (see testdata/README.md):
//
//	QBIT_JAY_EVAL_BUILD_FIXTURE=<queries.txt> go test -tags eval -run TestBuildFixture ./internal/qbit/eval/
//
// Input: one line per memory, "key|same|keywords|cross". The first two are in
// the memory's language, the last one in the other language.
func TestBuildFixture(t *testing.T) {
	path := os.Getenv("QBIT_JAY_EVAL_BUILD_FIXTURE")
	if path == "" {
		t.Skip("set QBIT_JAY_EVAL_BUILD_FIXTURE to the query list")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := Generate(DefaultSeed, e2Memories)
	byKey := map[string]Memory{}
	for _, m := range c.Memories {
		byKey[m.Key] = m
	}
	f := Fixture{Seed: DefaultSeed, Memories: e2Memories, Fingerprint: c.Fingerprint(e2Memories)}
	seen := map[string]bool{}
	for n, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			t.Fatalf("line %d: want 4 fields, got %d", n+1, len(parts))
		}
		m, ok := byKey[parts[0]]
		if !ok || seen[m.Key] {
			t.Fatalf("line %d: unknown or repeated memory %q", n+1, parts[0])
		}
		seen[m.Key] = true
		other := "es"
		if m.Lang == "es" {
			other = "en"
		}
		f.Queries = append(f.Queries,
			FixtureQuery{Mem: m.Key, Lang: m.Lang, Kind: KindSame, Query: strings.TrimSpace(parts[1])},
			FixtureQuery{Mem: m.Key, Lang: m.Lang, Kind: KindKW, Query: strings.TrimSpace(parts[2])},
			FixtureQuery{Mem: m.Key, Lang: other, Kind: KindCross, Query: strings.TrimSpace(parts[3])},
		)
	}
	if len(seen) != e2Memories {
		t.Fatalf("%d memories covered, want %d", len(seen), e2Memories)
	}
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(FixturePath, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("wrote %s with %d queries\n", FixturePath, len(f.Queries))
}
