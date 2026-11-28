package telemetry

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// errorClass reduces a tool error message to a class. Error text can echo user
// content, so only the class is ever stored.
func errorClass(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "timed out"), strings.Contains(lower, "deadline"):
		return "timeout"
	case strings.Contains(lower, "limit"), strings.Contains(lower, "too large"), strings.Contains(lower, "too long"), strings.Contains(lower, "exceed"):
		return "limit"
	}
	return "error"
}

// loop is the writer goroutine body after the database is open (or failed).
func (r *Recorder) loop(db *sql.DB, path string) {
	writes := 0
	var savedDropped, savedErrs int64
	persist := func() {
		if db == nil {
			return
		}
		d, w := r.dropped.Load(), r.errs.Load()
		if bumpMeta(db, "dropped", d-savedDropped) == nil {
			savedDropped = d
		}
		if bumpMeta(db, "write_errors", w-savedErrs) == nil {
			savedErrs = w
		}
	}
	defer persist()
	for e := range r.ch {
		if e.flush != nil {
			persist()
			close(e.flush)
			continue
		}
		if db == nil {
			r.errs.Add(1)
			continue
		}
		if err := insert(db, r.session, e); err != nil {
			r.errs.Add(1)
			continue
		}
		writes++
		if writes%pruneEvery == 0 {
			if err := prune(db, path, r.opts.Now(), r.opts.MaxAge, r.opts.MaxBytes); err != nil {
				r.errs.Add(1)
			}
			persist()
		}
	}
}

func insert(db *sql.DB, session string, e event) error {
	res, err := db.Exec(`INSERT INTO events(ts, kind, session, project, query, mode, results, ids, latency_ms, bytes, tokens, error, extra)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ts.UnixMilli(), e.kind, session, e.project, e.query, e.mode, e.results, e.ids,
		float64(e.latency)/float64(time.Millisecond), e.bytes, e.tokens, e.errText, e.extra)
	if err != nil {
		return err
	}
	if len(e.injected) == 0 {
		return nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, it := range e.injected {
		if _, err := db.Exec(`INSERT INTO injected(event_id, session, title, reason) VALUES(?,?,?,?)`, id, session, it.title, it.reason); err != nil {
			return err
		}
	}
	return nil
}

func clip(s string) string {
	if len(s) <= maxText {
		return s
	}
	cut := maxText
	for cut > 0 && s[cut]&0xC0 == 0x80 { // do not split a UTF-8 sequence
		cut--
	}
	return s[:cut]
}

// ObserveTool implements the MCP layer's observer. It does bounded in-memory
// work and a non-blocking enqueue; it never returns an error.
func (r *Recorder) ObserveTool(name string, args map[string]any, text string, isError bool, latency time.Duration) {
	if r == nil {
		return
	}
	now := r.opts.Now()
	switch name {
	case "mem_search":
		r.observeSearch(now, args, text, isError, latency)
	case "mem_get_observation":
		r.observeRead(now, args, text, isError, "injected_read")
	case "mem_update":
		r.observeRead(now, args, text, isError, "injected_update")
	case "mem_context":
		r.observeContext(now, args, text, isError, latency)
	case "mem_save":
		r.observeSave(now, args, text, isError, latency)
	}
}

type searchEnvelope struct {
	Project string `json:"project"`
	Results []struct {
		ID int64 `json:"id"`
	} `json:"results"`
}

func envelopeProject(text string) string {
	var env struct {
		Project string `json:"project"`
	}
	_ = json.Unmarshal([]byte(text), &env)
	return env.Project
}

func (r *Recorder) observeSearch(now time.Time, args map[string]any, text string, isError bool, latency time.Duration) {
	query, _ := args["query"].(string)
	mode, _ := args["match_mode"].(string)
	if mode == "" {
		mode = "all"
	}
	var env searchEnvelope
	if !isError {
		_ = json.Unmarshal([]byte(text), &env)
	}
	project := env.Project
	if project == "" {
		project, _ = args["project"].(string)
	}
	ids := make([]string, 0, 10)
	idSet := map[int64]bool{}
	for i, res := range env.Results {
		idSet[res.ID] = true
		if i < 10 {
			ids = append(ids, strconv.FormatInt(res.ID, 10))
		}
	}
	e := event{ts: now, kind: "search", project: project, query: clip(query), mode: mode,
		results: len(env.Results), ids: strings.Join(ids, ","), latency: latency}
	if isError {
		e.errText = errorClass(text)
	}

	r.mu.Lock()
	var reform *event
	if n := len(r.searches); n > 0 && now.Sub(r.searches[n-1].ts) < reformulationWindow {
		prev := r.searches[n-1]
		reform = &event{ts: now, kind: "reformulation", project: project, query: clip(query), mode: mode, extra: clip(prev.query)}
	}
	if !isError {
		r.searches = append(r.searches, recentSearch{ts: now, query: query, ids: idSet})
	}
	kept := r.searches[:0]
	for _, s := range r.searches {
		if now.Sub(s.ts) <= openWindow {
			kept = append(kept, s)
		}
	}
	r.searches = kept
	r.mu.Unlock()

	r.enqueue(e)
	if reform != nil {
		r.enqueue(*reform)
	}
}

func idArg(args map[string]any) int64 {
	switch v := args["id"].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

// observeRead handles mem_get_observation (an open when the id came from a
// recent search of this session, and a read of injected context when the
// observation carries an injected title) and mem_update (injected update).
func (r *Recorder) observeRead(now time.Time, args map[string]any, text string, isError bool, injectedKind string) {
	if isError {
		return
	}
	id := idArg(args)
	if id == 0 {
		return
	}
	var out []event
	r.mu.Lock()
	if injectedKind == "injected_read" {
		for i := len(r.searches) - 1; i >= 0; i-- {
			if now.Sub(r.searches[i].ts) <= openWindow && r.searches[i].ids[id] {
				out = append(out, event{ts: now, kind: "open", project: envelopeProject(text), query: clip(r.searches[i].query), ids: strconv.FormatInt(id, 10), results: 1})
				break
			}
		}
	}
	for title, reason := range r.pending {
		if strings.Contains(text, title) {
			out = append(out, event{ts: now, kind: injectedKind, ids: strconv.FormatInt(id, 10), extra: clip(title), mode: reason})
			delete(r.pending, title) // consumed: one event per injected title
		}
	}
	r.mu.Unlock()
	for _, e := range out {
		r.enqueue(e)
	}
}

// parseContext extracts the "- [type] **title**" bullets of the Pinned and
// Recent Observations sections that mem_context renders. The section is the
// reason each memory was injected; ids are not rendered, so a later read or
// update of the same title (by id) links them.
func parseContext(text string) []injectedItem {
	var body struct {
		Result string `json:"result"`
	}
	if json.Unmarshal([]byte(text), &body) == nil && body.Result != "" {
		text = body.Result
	}
	var items []injectedItem
	section := ""
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "### Pinned"):
			section = "pinned"
		case strings.HasPrefix(line, "### Recent Observations"):
			section = "recent"
		case strings.HasPrefix(line, "### "), strings.HasPrefix(line, "---"):
			section = ""
		case section != "" && strings.HasPrefix(line, "- ["):
			if i := strings.Index(line, "**"); i >= 0 {
				if j := strings.Index(line[i+2:], "**"); j > 0 {
					items = append(items, injectedItem{title: line[i+2 : i+2+j], reason: section})
				}
			}
		}
	}
	return items
}

func (r *Recorder) observeContext(now time.Time, args map[string]any, text string, isError bool, latency time.Duration) {
	if isError {
		r.enqueue(event{ts: now, kind: "context", latency: latency, errText: errorClass(text)})
		return
	}
	items := parseContext(text)
	project, _ := args["project"].(string)
	if p := envelopeProject(text); p != "" {
		project = p
	}
	r.mu.Lock()
	for _, it := range items {
		if len(r.pending) >= maxTrackedInjected {
			break
		}
		r.pending[it.title] = it.reason
	}
	r.mu.Unlock()
	r.enqueue(event{ts: now, kind: "context", project: project, results: len(items), bytes: len(text),
		tokens: (len(text) + 3) / 4, latency: latency, injected: items})
}

func (r *Recorder) observeSave(now time.Time, args map[string]any, text string, isError bool, latency time.Duration) {
	content, _ := args["content"].(string)
	if content == "" {
		content, _ = args["observation"].(string)
	}
	e := event{ts: now, kind: "save", bytes: len(content), latency: latency}
	e.project, _ = args["project"].(string)
	if isError {
		e.errText = errorClass(text)
		if e.errText == "limit" {
			e.extra = "limit"
		} else {
			e.extra = "error"
		}
	}
	r.enqueue(e)
}
