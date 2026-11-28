package telemetry_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/qbitdevz/qbit-jay/internal/qbit/telemetry"
)

func TestRetentionPrunesOldRowsAndCapsSize(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := telemetry.New(telemetry.Options{DataDir: dir, Now: func() time.Time { return now.Add(-100 * 24 * time.Hour) }})
	old.ObserveTool("mem_save", map[string]any{"content": "old"}, "ok", false, time.Millisecond)
	old.Close()

	fresh := telemetry.New(telemetry.Options{DataDir: dir, Now: func() time.Time { return now }})
	fresh.ObserveTool("mem_save", map[string]any{"content": "new"}, "ok", false, time.Millisecond)
	fresh.Close()
	sum, err := telemetry.LoadProject(dir, "")
	if err != nil || sum.Saves != 1 {
		t.Fatalf("90-day retention should keep only the fresh save: %+v err=%v", sum, err)
	}

	// Size cap: a tiny cap makes the writer drop the oldest rows.
	capDir := t.TempDir()
	capped := telemetry.New(telemetry.Options{DataDir: capDir, MaxBytes: 64 << 10, Now: func() time.Time { return now }})
	filler := strings.Repeat("x", 400)
	for i := 0; i < 1500; i++ {
		capped.ObserveTool("mem_search", map[string]any{"query": filler}, `{"results":[{"id":1}]}`, false, time.Millisecond)
	}
	capped.Close()
	reopened := telemetry.New(telemetry.Options{DataDir: capDir, MaxBytes: 64 << 10, Now: func() time.Time { return now }})
	reopened.Close()
	fi, err := os.Stat(telemetry.Path(capDir))
	if err != nil || fi.Size() > 64<<10 {
		t.Fatalf("size cap not enforced: %v", err)
	}
}

func TestWriteFailureDoesNotChangeToolResponses(t *testing.T) {
	plain := newRig(t, nil)
	plain.save("Choose sqlite", "We chose sqlite for storage")
	wantSearch, _ := plain.call("mem_search", map[string]any{"query": "sqlite"})

	broken := newRig(t, func(o *telemetry.Options) {
		// The telemetry directory sits under a regular file: it can never be opened.
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		o.DataDir = filepath.Join(blocker, "sub")
	})
	broken.save("Choose sqlite", "We chose sqlite for storage")
	got, bad := broken.call("mem_search", map[string]any{"query": "sqlite"})
	broken.rec.Flush()
	if bad {
		t.Fatalf("search must still succeed: %s", got)
	}
	if strip(got) != strip(wantSearch) {
		t.Fatalf("telemetry failure changed the response:\n%s\nvs\n%s", got, wantSearch)
	}
	if broken.rec.Errors() == 0 {
		t.Fatal("write failures must be counted")
	}
}

func TestReadOnlyFileDoesNotChangeToolResponses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write to a 0400 file")
	}
	plain := newRig(t, nil)
	plain.save("Choose sqlite", "We chose sqlite for storage")
	wantSearch, wantBad := plain.call("mem_search", map[string]any{"query": "sqlite"})

	dir := filepath.Join(t.TempDir(), "tel")
	seed := telemetry.New(telemetry.Options{DataDir: dir})
	seed.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0)
	seed.Close()
	if err := os.Chmod(telemetry.Path(dir), 0o400); err != nil {
		t.Fatal(err)
	}
	ro := newRig(t, func(o *telemetry.Options) { o.DataDir = dir })
	ro.save("Choose sqlite", "We chose sqlite for storage")
	got, bad := ro.call("mem_search", map[string]any{"query": "sqlite"})
	ro.rec.Flush()
	if bad != wantBad || strip(got) != strip(wantSearch) {
		t.Fatalf("read-only telemetry changed the response:\n%s\nvs\n%s", got, wantSearch)
	}
	if ro.rec.Errors() == 0 {
		t.Fatal("write failures on a read-only file must be counted")
	}
}

// strip removes fields that legitimately differ between two stores (ids,
// timestamps, sync ids) so only the response shape is compared.
func strip(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) != nil {
		return s
	}
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			for k, e := range t {
				switch k {
				case "id", "sync_id", "created_at", "updated_at", "review_after", "result", "project_path":
					delete(t, k)
				default:
					t[k] = walk(e)
				}
			}
		case []any:
			for i := range t {
				t[i] = walk(t[i])
			}
		}
		return x
	}
	out, _ := json.Marshal(walk(v))
	return string(out)
}

func TestObserverIgnoresGarbage(t *testing.T) {
	r := newRig(t, nil)
	r.rec.ObserveTool("mem_search", nil, "not json", false, 0)
	r.rec.ObserveTool("mem_get_observation", map[string]any{"id": "x"}, "", false, 0)
	r.rec.ObserveTool("unknown_tool", nil, "", true, 0)
	r.rec.Flush()
}

func TestStatsSummaryAndClear(t *testing.T) {
	r := newRig(t, nil)
	r.save("Choose sqlite", "We chose sqlite for storage")
	ids := r.searchIDs("sqlite")
	r.searchIDs("nonexistentterm")
	r.call("mem_get_observation", map[string]any{"id": float64(ids[0])})
	r.rec.Flush()

	sum, err := telemetry.LoadProject(r.dataDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Searches != 2 || sum.ZeroResult != 1 || sum.Opens != 1 || sum.Reformulations != 1 || sum.Saves != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	if len(sum.TopOpened) != 1 || sum.TopOpened[0].Count != 1 {
		t.Fatalf("top opened: %+v", sum.TopOpened)
	}
	var b strings.Builder
	sum.Render(&b)
	for _, want := range []string{"Searches: 2", "zero-result rate:     50.0%", "reformulation rate:   50.0%", "opens after search:   1", "Saves: 1"} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("render missing %q:\n%s", want, b.String())
		}
	}

	// Clear while the recorder (an MCP process) is still running and holds the file.
	cleared, err := telemetry.Clear(r.dataDir)
	if err != nil || !cleared {
		t.Fatalf("clear: cleared=%v err=%v", cleared, err)
	}
	after, err := telemetry.LoadProject(r.dataDir, "")
	if err != nil || after.Searches != 0 || after.Saves != 0 {
		t.Fatalf("load after clear: %+v err=%v", after, err)
	}
	// The running writer still has a valid database.
	r.save("Another", "after the clear")
	r.rec.Flush()
	again, err := telemetry.LoadProject(r.dataDir, "")
	if err != nil || again.Saves != 1 {
		t.Fatalf("writer must keep working after clear: %+v err=%v", again, err)
	}
	if r.rec.Errors() != 0 {
		t.Fatalf("clear must not cause write errors: %d", r.rec.Errors())
	}
	r.rec.Close()
	missing := filepath.Join(t.TempDir(), "missing")
	none, err := telemetry.Clear(missing)
	if err != nil || none {
		t.Fatalf("clear of a missing file: %v %v", none, err)
	}
	if _, err := os.Stat(telemetry.Path(missing)); !os.IsNotExist(err) {
		t.Fatal("Clear must never create the file")
	}
}

func TestSaveLimitAndErrorClassification(t *testing.T) {
	r := newRig(t, nil)
	r.rec.ObserveTool("mem_save", map[string]any{"content": "aaa", "project": "demo"}, "content exceeds limit of 100 bytes: SECRET-USER-TEXT", true, 0)
	r.rec.ObserveTool("mem_save", map[string]any{"content": "bbb"}, "boom SECRET-USER-TEXT", true, 0)
	r.rec.ObserveTool("mem_search", map[string]any{"query": "find me"}, "context deadline exceeded SECRET-USER-TEXT", true, 0)
	ev := r.rows(`SELECT kind, query, error, extra FROM events ORDER BY id`)
	if len(ev) != 3 {
		t.Fatalf("events: %v", ev)
	}
	if ev[0]["error"] != "limit" || ev[0]["extra"] != "limit" || ev[1]["error"] != "error" || ev[1]["extra"] != "error" {
		t.Fatalf("save classification: %v %v", ev[0], ev[1])
	}
	if ev[2]["error"] != "timeout" || ev[2]["query"] != "find me" {
		t.Fatalf("search error keeps the query and a class only: %v", ev[2])
	}
	sum, _ := telemetry.LoadProject(r.dataDir, "")
	if sum.SaveLimit != 1 || sum.SaveErrors != 2 {
		t.Fatalf("summary: %+v", sum)
	}
	r.rec.Close()
	raw, err := os.ReadFile(telemetry.Path(r.dataDir))
	if err != nil || strings.Contains(string(raw), "SECRET-USER-TEXT") {
		t.Fatalf("error text must never be persisted (err=%v)", err)
	}
}

func TestFilePermissionsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps default ACLs")
	}
	dir := filepath.Join(t.TempDir(), "newdir")
	rec := telemetry.New(telemetry.Options{DataDir: dir})
	rec.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0)
	rec.Flush()
	for _, p := range []string{dir, telemetry.Path(dir)} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s must be owner-only, got %v", p, fi.Mode().Perm())
		}
	}
	rec.Close()
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if fi, err := os.Stat(telemetry.Path(dir) + suffix); err == nil && fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s must be owner-only, got %v", suffix, fi.Mode().Perm())
		}
	}
}

func TestDroppedAndErrorCountersPersistAndRender(t *testing.T) {
	dir := t.TempDir()
	rec := telemetry.New(telemetry.Options{DataDir: dir})
	rec.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0)
	rec.Close()
	rec.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0) // after close: dropped
	if rec.Dropped() != 1 {
		t.Fatalf("dropped: %d", rec.Dropped())
	}
	// Counters of another process add to the persisted totals.
	db, err := sql.Open("sqlite", telemetry.Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO meta(key,value) VALUES('dropped',3),('write_errors',2) ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	sum, err := telemetry.LoadProject(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	sum.Render(&b)
	if sum.Dropped != 3 || sum.WriteErrors != 2 || !strings.Contains(b.String(), "dropped events: 3, write errors: 2") {
		t.Fatalf("counters: %+v\n%s", sum, b.String())
	}
}

func TestPersistedCountersFromRecorder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	rec := telemetry.New(telemetry.Options{DataDir: dir})
	// Overflow the bounded queue without waiting on the writer.
	for i := 0; i < 5000; i++ {
		rec.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0)
	}
	rec.Close()
	if rec.Dropped() == 0 {
		t.Skip("writer kept up with the producer; nothing was dropped")
	}
	sum, err := telemetry.LoadProject(dir, "")
	if err != nil || sum.Dropped != rec.Dropped() {
		t.Fatalf("persisted dropped %d != recorder %d (err=%v)", sum.Dropped, rec.Dropped(), err)
	}
}

func TestLoadProjectFilters(t *testing.T) {
	dir := t.TempDir()
	rec := telemetry.New(telemetry.Options{DataDir: dir})
	rec.ObserveTool("mem_search", map[string]any{"query": "a"}, `{"project":"alpha","results":[{"id":1}]}`, false, 0)
	rec.ObserveTool("mem_search", map[string]any{"query": "b"}, `{"project":"beta"}`, false, 0)
	rec.ObserveTool("mem_save", map[string]any{"content": "x", "project": "Alpha"}, "ok", false, 0)
	rec.Close()
	all, _ := telemetry.LoadProject(dir, "")
	one, err := telemetry.LoadProject(dir, "alpha")
	if err != nil || all.Searches != 2 || one.Searches != 1 || one.Saves != 1 || one.ZeroResult != 0 {
		t.Fatalf("all=%+v one=%+v err=%v", all, one, err)
	}
}

func TestDedupesInjectedReadPerTitle(t *testing.T) {
	r := newRig(t, nil)
	ctx := "### Pinned\n- [decision] **Choose sqlite**\n"
	r.rec.ObserveTool("mem_context", nil, ctx, false, 0)
	for i := 0; i < 3; i++ {
		r.rec.ObserveTool("mem_get_observation", map[string]any{"id": float64(5)}, `{"title":"Choose sqlite"}`, false, 0)
	}
	ev := r.rows(`SELECT kind FROM events WHERE kind='injected_read'`)
	if len(ev) != 1 {
		t.Fatalf("a pending title is consumed after its first event: %v", ev)
	}
}
