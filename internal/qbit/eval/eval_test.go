//go:build eval

package eval

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// flatEntry is a flat corpus loaded once per size and shared by E2, E3 and the
// benchmarks of one test binary.
type flatEntry struct {
	l       *Loaded
	c       *Corpus
	seconds float64
}

var (
	flatMu    sync.Mutex
	flatCache = map[int]*flatEntry{}
	tmpRoot   string
)

func TestMain(m *testing.M) {
	var err error
	tmpRoot, err = os.MkdirTemp("", "qbit-jay-eval-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	for _, e := range flatCache {
		_ = e.l.Close()
	}
	_ = os.RemoveAll(tmpRoot)
	os.Exit(code)
}

func seedFromEnv() uint64 {
	if v := os.Getenv("QBIT_JAY_EVAL_SEED"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return DefaultSeed
}

func flat(tb testing.TB, size int) *flatEntry {
	tb.Helper()
	flatMu.Lock()
	defer flatMu.Unlock()
	if e, ok := flatCache[size]; ok {
		return e
	}
	c := Generate(seedFromEnv(), size)
	l, err := Open(fmt.Sprintf("%s/flat-%d", tmpRoot, size))
	if err != nil {
		tb.Fatal(err)
	}
	start := time.Now()
	if err := l.LoadFlat(c); err != nil {
		tb.Fatal(err)
	}
	e := &flatEntry{l: l, c: c, seconds: time.Since(start).Seconds()}
	flatCache[size] = e
	return e
}

func sizesFromEnv(tb testing.TB) []int {
	raw := os.Getenv("QBIT_JAY_EVAL_SIZES")
	if raw == "" {
		raw = "1000,10000"
	}
	var out []int
	for _, s := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 200 {
			tb.Fatalf("QBIT_JAY_EVAL_SIZES: bad size %q (need an integer >= 200)", s)
		}
		out = append(out, n)
	}
	return out
}

// TestEval runs E1, E2 and E3 and prints the results. Sizes come from
// QBIT_JAY_EVAL_SIZES (default 1000,10000; add 100000 to opt in). Set
// QBIT_JAY_EVAL_JSON to also write the report as JSON, QBIT_JAY_EVAL_LABEL and
// QBIT_JAY_EVAL_COMMIT to label it, and QBIT_JAY_EVAL_DB to add E1 on a
// read-only copy of a real database (never set in tests or CI).
func TestEval(t *testing.T) {
	sizes := sizesFromEnv(t)
	rep := &Report{Meta: Meta{
		Label: os.Getenv("QBIT_JAY_EVAL_LABEL"), Commit: os.Getenv("QBIT_JAY_EVAL_COMMIT"),
		GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(),
		Seed: seedFromEnv(), Sizes: sizes, Date: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}}
	if rep.Meta.Label == "" {
		rep.Meta.Label = "unlabelled"
	}
	if rep.Meta.Commit == "" {
		rep.Meta.Commit = "unknown"
	}
	for _, size := range sizes {
		e := flat(t, size)
		f, err := LoadFixture(e.c)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"all", "any"} {
			r, err := RunE2(e.l, e.c, f, mode)
			if err != nil {
				t.Fatal(err)
			}
			rep.E2 = append(rep.E2, r)
		}
		r3, err := RunE3(e.l, e.c, 300, 100)
		if err != nil {
			t.Fatal(err)
		}
		r3.LoadSeconds = e.seconds
		rep.E3 = append(rep.E3, r3)

		if size <= 10000 {
			start := time.Now()
			r1, err := RunE1(Generate(seedFromEnv(), size), t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			r1.LoadSeconds = time.Since(start).Seconds()
			rep.E1 = append(rep.E1, r1)
		}
	}
	if path := os.Getenv("QBIT_JAY_EVAL_DB"); path != "" {
		r, err := RunE1Real(path)
		if err != nil {
			t.Fatal(err)
		}
		rep.E1 = append(rep.E1, r)
	}
	rep.Print(os.Stdout)
	if path := os.Getenv("QBIT_JAY_EVAL_JSON"); path != "" {
		if err := rep.WriteJSON(path); err != nil {
			t.Fatal(err)
		}
	}
}

// Benchmarks: store search and context over synthetic corpora of 10k and 100k
// memories. 100k takes minutes to build, so it is opt-in:
// QBIT_JAY_EVAL_100K=1.
func benchSizes(b *testing.B) []int {
	b.Helper()
	sizes := []int{10000}
	if os.Getenv("QBIT_JAY_EVAL_100K") == "1" {
		sizes = append(sizes, 100000)
	}
	return sizes
}

func BenchmarkSearch(b *testing.B) {
	for _, size := range benchSizes(b) {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			e := flat(b, size)
			work := SearchWorkload(e.c, 300)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q := work[i%len(work)]
				if _, err := e.l.Store.SearchPreviewsContext(b.Context(), q.Query, searchOpts(q)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkContext(b *testing.B) {
	for _, size := range benchSizes(b) {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			e := flat(b, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := e.l.Store.FormatContextWithOptions(projects[i%len(projects)].name, "", contextOpts()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
