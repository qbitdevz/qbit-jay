package telemetry_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	_ "modernc.org/sqlite"

	"github.com/qbitdevz/qbit-jay/internal/mcp"
	"github.com/qbitdevz/qbit-jay/internal/qbit/telemetry"
	"github.com/qbitdevz/qbit-jay/internal/store"
)

type rig struct {
	t       *testing.T
	srv     *mcpserver.MCPServer
	rec     *telemetry.Recorder
	dataDir string
	mainDB  string
	clock   *time.Time
}

// newRig builds a real MCP server on a real store in a temp HOME, observed by
// a telemetry recorder writing to its own directory.
func newRig(t *testing.T, opts func(*telemetry.Options)) *rig {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("QBIT_JAY_TELEMETRY", "")
	t.Setenv("ENGRAM_TELEMETRY", "")
	cfg, err := store.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = filepath.Join(home, ".qbit-jay")
	s, err := store.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now()
	o := telemetry.Options{DataDir: cfg.DataDir, Now: func() time.Time { return now }}
	if opts != nil {
		opts(&o)
	}
	rec := telemetry.New(o)
	r := &rig{t: t, rec: rec, dataDir: cfg.DataDir, mainDB: filepath.Join(cfg.DataDir, "qbit-jay.db"), clock: &now}
	mcfg := mcp.MCPConfig{DefaultProject: "demo"}
	if rec != nil {
		mcfg.Observer = rec
		t.Cleanup(rec.Close)
	}
	r.srv = mcp.NewServerWithConfig(s, mcfg, nil)
	return r
}

func (r *rig) call(name string, args map[string]any) (string, bool) {
	r.t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	raw, _ := json.Marshal(r.srv.HandleMessage(context.Background(), req))
	var wire struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire.Result.Content) == 0 {
		r.t.Fatalf("bad response for %s: %s", name, raw)
	}
	return wire.Result.Content[0].Text, wire.Result.IsError
}

func (r *rig) save(title, content string) {
	r.t.Helper()
	if out, bad := r.call("mem_save", map[string]any{"title": title, "content": content, "type": "decision"}); bad {
		r.t.Fatalf("save failed: %s", out)
	}
}

func (r *rig) searchIDs(query string) []int64 {
	r.t.Helper()
	out, bad := r.call("mem_search", map[string]any{"query": query})
	if bad {
		r.t.Fatalf("search failed: %s", out)
	}
	var env struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	_ = json.Unmarshal([]byte(out), &env)
	ids := make([]int64, 0, len(env.Results))
	for _, x := range env.Results {
		ids = append(ids, x.ID)
	}
	return ids
}

func (r *rig) rows(q string, args ...any) []map[string]any {
	r.t.Helper()
	r.rec.Flush()
	db, err := sql.Open("sqlite", telemetry.Path(r.dataDir))
	if err != nil {
		r.t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rs, err := db.Query(q, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	defer func() { _ = rs.Close() }()
	cols, _ := rs.Columns()
	var out []map[string]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			r.t.Fatal(err)
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out
}

func TestSearchEventLandsInSeparateDBAndMainSchemaIsUntouched(t *testing.T) {
	r := newRig(t, nil)
	r.save("Choose sqlite", "We chose sqlite for storage")
	ids := r.searchIDs("sqlite")
	if len(ids) == 0 {
		t.Fatal("expected a search hit")
	}
	ev := r.rows(`SELECT query, mode, results, ids, project, latency_ms FROM events WHERE kind='search'`)
	if len(ev) != 1 || ev[0]["query"] != "sqlite" || ev[0]["mode"] != "all" || ev[0]["project"] != "demo" {
		t.Fatalf("unexpected search events: %v", ev)
	}
	if ev[0]["results"].(int64) != int64(len(ids)) || !strings.HasPrefix(ev[0]["ids"].(string), "") || ev[0]["ids"] == "" {
		t.Fatalf("results/ids not recorded: %v", ev[0])
	}
	// The main database must not know telemetry exists.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(r.mainDB)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('events','injected') OR name LIKE '%telemetry%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("telemetry leaked into main schema: n=%d err=%v", n, err)
	}
}

func TestDisabledCreatesNoFile(t *testing.T) {
	for _, v := range []string{"off", "OFF", "0", "false"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("QBIT_JAY_TELEMETRY", v)
			dir := t.TempDir()
			if rec := telemetry.New(telemetry.Options{DataDir: dir}); rec != nil {
				t.Fatalf("recorder must be nil when off")
			}
			if _, err := os.Stat(telemetry.Path(dir)); !os.IsNotExist(err) {
				t.Fatalf("telemetry.db must not exist, err=%v", err)
			}
		})
	}
}

func TestOpenAndReformulationDetection(t *testing.T) {
	r := newRig(t, nil)
	r.save("Choose sqlite", "We chose sqlite for storage")
	other := r.save2()
	ids := r.searchIDs("sqlite")
	*r.clock = r.clock.Add(20 * time.Second) // within 60 s
	r.searchIDs("storage")
	r.call("mem_get_observation", map[string]any{"id": float64(ids[0])})
	r.call("mem_get_observation", map[string]any{"id": float64(other)}) // never returned by a search: no open

	if got := r.rows(`SELECT query, extra FROM events WHERE kind='reformulation'`); len(got) != 1 || got[0]["extra"] != "sqlite" || got[0]["query"] != "storage" {
		t.Fatalf("reformulation: %v", got)
	}
	if got := r.rows(`SELECT ids FROM events WHERE kind='open'`); len(got) != 1 {
		t.Fatalf("expected exactly one open, got %v", got)
	}

	*r.clock = r.clock.Add(5 * time.Minute) // outside 60 s: no new reformulation
	r.searchIDs("sqlite")
	if got := r.rows(`SELECT 1 FROM events WHERE kind='reformulation'`); len(got) != 1 {
		t.Fatalf("a search after 60 s is not a reformulation: %v", got)
	}
}

// save2 stores a memory that the searches in the tests never match.
func (r *rig) save2() int64 {
	r.t.Helper()
	out, bad := r.call("mem_save", map[string]any{"title": "Zebra note", "content": "unrelated zebra", "type": "decision"})
	if bad {
		r.t.Fatal(out)
	}
	hits := r.searchIDs("zebra")
	*r.clock = r.clock.Add(11 * time.Minute) // expire this lookup so opening it is not an open
	if len(hits) == 0 {
		r.t.Fatal("zebra not found")
	}
	r.rec.Flush()
	return hits[0]
}

func TestContextEventRecordsBytesTokensAndLaterRead(t *testing.T) {
	r := newRig(t, nil)
	r.save("Pinned decision", "keep this one")
	ids := r.searchIDs("decision")
	out, bad := r.call("mem_context", map[string]any{})
	if bad {
		t.Fatal(out)
	}
	ev := r.rows(`SELECT bytes, tokens, results FROM events WHERE kind='context'`)
	if len(ev) != 1 || ev[0]["bytes"].(int64) != int64(len(out)) || ev[0]["tokens"].(int64) <= 0 {
		t.Fatalf("context event: %v (len %d)", ev, len(out))
	}
	inj := r.rows(`SELECT title, reason FROM injected`)
	if len(inj) == 0 {
		t.Fatalf("expected injected titles; context was:\n%s", out)
	}
	r.call("mem_get_observation", map[string]any{"id": float64(ids[0])})
	if got := r.rows(`SELECT ids FROM events WHERE kind='injected_read'`); len(got) != 1 {
		t.Fatalf("expected a later read of an injected memory, got %v", got)
	}
}

func TestSaveEvents(t *testing.T) {
	r := newRig(t, nil)
	r.save("Choose sqlite", "We chose sqlite for storage")
	if _, bad := r.call("mem_save", map[string]any{"content": "no title"}); !bad {
		t.Fatal("a save without title must fail")
	}
	ev := r.rows(`SELECT bytes, error, extra, latency_ms FROM events WHERE kind='save' ORDER BY id`)
	if len(ev) != 2 {
		t.Fatalf("save events: %v", ev)
	}
	if ev[0]["error"] != "" || ev[0]["bytes"].(int64) != int64(len("We chose sqlite for storage")) {
		t.Fatalf("ok save: %v", ev[0])
	}
	if ev[1]["error"] == "" || ev[1]["extra"] != "error" {
		t.Fatalf("failed save: %v", ev[1])
	}
}
