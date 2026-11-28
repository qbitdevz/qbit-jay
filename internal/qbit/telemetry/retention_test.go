package telemetry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	sum, err := telemetry.Load(dir)
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
	if err != nil || fi.Size() > 96<<10 {
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
	r := newRig(t, nil)
	r.save("Choose sqlite", "We chose sqlite for storage")
	r.rec.Flush()
	r.rec.Close()
	if err := os.Chmod(telemetry.Path(r.dataDir), 0o400); err != nil {
		t.Fatal(err)
	}
	rec := telemetry.New(telemetry.Options{DataDir: r.dataDir})
	rec.ObserveTool("mem_save", map[string]any{"content": "x"}, "ok", false, 0)
	rec.Flush()
	rec.Close()
	// Reaching this point without a panic or hang is the contract; the error
	// counter is informational only (root can still write to a 0400 file).
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
				case "id", "sync_id", "created_at", "updated_at", "result", "project_path":
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

	sum, err := telemetry.Load(r.dataDir)
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

	r.rec.Close()
	removed, err := telemetry.Clear(r.dataDir)
	if err != nil || !removed {
		t.Fatalf("clear: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(telemetry.Path(r.dataDir)); !os.IsNotExist(err) {
		t.Fatalf("file must be gone: %v", err)
	}
	after, err := telemetry.Load(r.dataDir)
	if err != nil || after.Exists {
		t.Fatalf("load after clear: %+v err=%v", after, err)
	}
	if _, err := os.Stat(telemetry.Path(r.dataDir)); !os.IsNotExist(err) {
		t.Fatal("Load must never create the file")
	}
}
