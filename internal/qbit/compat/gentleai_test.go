package compat

// Compatibility with gentle-ai (G1, G2, G3, G5, G6; spec 7.3).
//
// Every invocation below is one gentle-ai makes against the Engram binary
// (docs/INTEGRATION.md has the full inventory with file:line references).
// The parsers are copied, minimally, from gentle-ai so the test fails if
// qbit-jay's output stops satisfying them:
//
//	gentle-ai, MIT License, Copyright (c) 2025 Gentleman Programming
//	https://github.com/Gentleman-Programming/gentle-ai
//	commit d64b1c1baa22cf34d3a8da29050d57103a07cf6a (v5.1.0 line)
//
// Copied: versionRegexp/devVersionRegexp/parseVersionFromOutput
// (internal/update/detect.go:30-34,135-152), engramVersionPattern
// (internal/components/engram/protocol.go:32) and the initialize validation
// of the stdio health probe (internal/components/engram/healthprobe.go,
// validateInitializeResponse and validateInitializeResult).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ── parsers copied from gentle-ai ───────────────────────────────────────────

var versionRegexp = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

var devVersionRegexp = regexp.MustCompile(`(?i)(?:^|\s)dev(?:$|\s)`)

// engramVersionPattern gates the slim protocol (protocol.go:32).
var engramVersionPattern = regexp.MustCompile(`^(?:engram\s+)?v?(\d+)\.(\d+)\.(\d+)$`)

func parseVersionFromOutput(output string) string {
	if output == "" {
		return ""
	}
	if devVersionRegexp.MatchString(output) {
		return "dev"
	}
	if match := versionRegexp.FindStringSubmatch(output); len(match) >= 2 {
		return match[1]
	}
	return ""
}

func validateInitializeResponse(line []byte) error {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return errors.New("invalid MCP stdout: empty frame")
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  json.RawMessage `json:"method"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(line, &response); err != nil {
		return fmt.Errorf("invalid MCP stdout: malformed JSON-RPC frame: %w", err)
	}
	if response.JSONRPC != "2.0" {
		return errors.New(`invalid MCP stdout: jsonrpc must be "2.0"`)
	}
	if !bytes.Equal(bytes.TrimSpace(response.ID), []byte("1")) {
		return errors.New("invalid MCP stdout: unexpected response id")
	}
	if len(response.Method) > 0 {
		return errors.New("invalid MCP stdout: initialize response must not include method")
	}
	if len(response.Error) > 0 {
		return fmt.Errorf("initialize returned error: %s", response.Error)
	}
	if len(response.Result) == 0 {
		return errors.New("initialize returned invalid result: result is required")
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(response.Result, &result); err != nil || result == nil {
		return errors.New("result must be an object")
	}
	if !nonEmptyString(result["protocolVersion"]) {
		return errors.New("protocolVersion must be a non-empty string")
	}
	var capabilities map[string]json.RawMessage
	if json.Unmarshal(result["capabilities"], &capabilities) != nil || capabilities == nil {
		return errors.New("capabilities must be an object")
	}
	var server map[string]json.RawMessage
	if json.Unmarshal(result["serverInfo"], &server) != nil || server == nil {
		return errors.New("serverInfo must be an object")
	}
	if !nonEmptyString(server["name"]) || !nonEmptyString(server["version"]) {
		return errors.New("serverInfo.name and serverInfo.version must be non-empty strings")
	}
	return nil
}

func nonEmptyString(raw json.RawMessage) bool {
	var v string
	return json.Unmarshal(raw, &v) == nil && strings.TrimSpace(v) != ""
}

// ── G1: command line surface ────────────────────────────────────────────────

// gentle-ai runs `<bin> version` (verify.go:107, doctor_tool_probe.go:25,
// update/registry.go:45) and keeps the trimmed stdout.
func TestG1VersionOutputParses(t *testing.T) {
	bin := builtBinary(t)
	env, _, _ := isolatedEnv(t)
	stdout, _, err := runOutput(t, bin, env, 20*time.Second, "version")
	if err != nil {
		t.Fatalf("version exited with error: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Fatal("gentle-ai rejects an empty version (verify.go:112)")
	}
	if got := parseVersionFromOutput(trimmed); got != "0.1.0" {
		t.Errorf("update check parses %q as %q, want 0.1.0", trimmed, got)
	}
	// The slim protocol gate (protocol.go:32) only accepts "[engram ]X.Y.Z".
	// qbit-jay prints "qbit-jay X.Y.Z", which does not match, so gentle-ai
	// keeps the full protocol, its safe default. Pinned so a format change is
	// a conscious decision (docs/INTEGRATION.md, "version").
	if engramVersionPattern.MatchString(trimmed) {
		t.Errorf("version %q now matches gentle-ai's slim gate; update docs/INTEGRATION.md", trimmed)
	}
}

// An unstamped build prints "qbit-jay dev"; gentle-ai reads it as "dev".
func TestG1DevVersionIsRecognized(t *testing.T) {
	if got := parseVersionFromOutput("qbit-jay dev"); got != "dev" {
		t.Errorf("parse(qbit-jay dev) = %q, want dev", got)
	}
}

// `<bin> setup --help` with a 5 second deadline, stdin detached and only
// stdout read; the capability is the substring "--protocol" (setup.go:46 and
// run.go, strings.Contains(stdout, "--protocol")).
func TestG1SetupHelpAdvertisesProtocolFlag(t *testing.T) {
	bin := builtBinary(t)
	env, home, _ := isolatedEnv(t)
	stdout, _, err := runOutput(t, bin, env, 5*time.Second, "setup", "--help")
	if err != nil {
		t.Fatalf("setup --help failed: %v", err)
	}
	if !strings.Contains(stdout, "--protocol") {
		t.Errorf("setup --help stdout does not mention --protocol:\n%s", stdout)
	}
	assertEmptyDir(t, home, "setup --help must be side-effect free")
}

// `<bin> instance-id` with ENGRAM_NO_UPDATE_CHECK=1 and ENGRAM_DATA_DIR set
// to a throwaway directory (verify.go:46-56). gentle-ai only treats a failure
// that says "unknown command" as unsupported.
func TestG1InstanceIDHonorsLegacyDataDir(t *testing.T) {
	bin := builtBinary(t)
	env, home, dataDir := isolatedEnv(t)
	stdout, stderr, err := runOutput(t, bin, env, 20*time.Second, "instance-id")
	if err != nil {
		t.Fatalf("instance-id failed: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("instance-id printed nothing")
	}
	if strings.Contains(strings.ToLower(stderr), "unknown command") {
		t.Errorf("gentle-ai would report instance-id as unsupported: %s", stderr)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("ENGRAM_DATA_DIR was not used: %v", err)
	}
	assertEmptyDir(t, home, "instance-id wrote outside ENGRAM_DATA_DIR")
}

// The same variable under its qbit-jay name wins over the Engram one.
func TestG1InstanceIDPrefersQbitJayDataDir(t *testing.T) {
	bin := builtBinary(t)
	preferred := filepath.Join(t.TempDir(), "preferred")
	env, _, legacy := isolatedEnv(t, "QBIT_JAY_DATA_DIR="+preferred)
	if _, stderr, err := runOutput(t, bin, env, 20*time.Second, "instance-id"); err != nil {
		t.Fatalf("instance-id failed: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(preferred); err != nil {
		t.Errorf("QBIT_JAY_DATA_DIR not used: %v", err)
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Errorf("ENGRAM_DATA_DIR %s was used although QBIT_JAY_DATA_DIR is set", legacy)
	}
}

// The doctor's stdio probe (healthprobe.go stdioHandshake): send one
// initialize request, accept exactly one valid response frame, close stdin and
// reject any further stdout frame.
func TestG1MCPHandshakeSatisfiesDoctorProbe(t *testing.T) {
	bin := builtBinary(t)
	env, _, _ := isolatedEnv(t)
	cmd := exec.Command(bin, "mcp", "--tools=agent")
	cmd.Env = env
	cmd.Dir = t.TempDir()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"gentle-ai-doctor","version":"0"}}}` + "\n"
	if _, err := io.WriteString(stdin, request); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		first  string
		extras []string
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var o outcome
		if !sc.Scan() {
			o.err = fmt.Errorf("mcp exited without answering initialize: %v", sc.Err())
			done <- o
			return
		}
		o.first = sc.Text()
		_ = stdin.Close()
		for sc.Scan() {
			o.extras = append(o.extras, sc.Text())
		}
		done <- o
	}()
	var o outcome
	select {
	case o = <-done:
	case <-time.After(15 * time.Second): // gentle-ai's default probe deadline
		_ = cmd.Process.Kill()
		t.Fatalf("no initialize response within gentle-ai's 15s probe deadline; stderr: %s", stderr.String())
	}
	_ = cmd.Wait()
	if o.err != nil {
		t.Fatalf("%v; stderr: %s", o.err, stderr.String())
	}
	if err := validateInitializeResponse([]byte(o.first)); err != nil {
		t.Fatalf("gentle-ai would reject the initialize response: %v\n%s", err, o.first)
	}
	if len(o.extras) > 0 {
		t.Errorf("extra stdout frame after the initialize response (gentle-ai rejects it): %q", o.extras)
	}
}

// ── G6 and the agent profile ────────────────────────────────────────────────

// Every tool the protocol gentle-ai injects names (assets/engram/protocol.md
// and the ODD skills) must be served by `mcp --tools=agent`.
func TestG6AgentProfileServesToolsNamedByGentleAI(t *testing.T) {
	bin := builtBinary(t)
	env, _, _ := isolatedEnv(t)
	s := startMCP(t, bin, t.TempDir(), env, "mcp", "--tools=agent")
	s.initialize()
	raw := s.call("tools/list", nil)
	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		have[tool.Name] = true
	}
	// Names found in gentle-ai internal/assets at the commit above.
	for _, name := range []string{
		"mem_save", "mem_search", "mem_get_observation", "mem_review", "mem_context",
		"mem_session_summary", "mem_session_start", "mem_session_end", "mem_save_prompt",
		"mem_update", "mem_judge", "mem_current_project", "mem_capture_passive",
		"mem_suggest_topic_key", "mem_doctor",
	} {
		if !have[name] {
			t.Errorf("tool %s named by gentle-ai's protocol is missing from --tools=agent", name)
		}
	}
	// Admin tools stay out of the agent profile.
	for _, name := range []string{"mem_delete", "mem_stats", "mem_timeline", "mem_merge_projects"} {
		if have[name] {
			t.Errorf("tool %s must not be in the agent profile", name)
		}
	}
}

// ── G2, G3, G5: ODD mirror contract ─────────────────────────────────────────

// odd is a small ODD-style client over one MCP session in a project directory.
type odd struct {
	t testing.TB
	s *mcpSession
}

func newODD(t testing.TB, project string) *odd {
	t.Helper()
	bin := builtBinary(t)
	env, _, _ := isolatedEnv(t)
	dir := filepath.Join(t.TempDir(), project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := startMCP(t, bin, dir, env, "mcp", "--tools=agent")
	s.initialize()
	return &odd{t: t, s: s}
}

type saved struct {
	ID         int64  `json:"id"`
	Project    string `json:"project"`
	Result     string `json:"result"`
	Truncation *struct {
		OriginalBytes int  `json:"original_bytes"`
		LimitBytes    int  `json:"limit_bytes"`
		Truncated     bool `json:"truncated"`
	} `json:"truncation"`
}

// mirror saves an ODD mirror exactly as the ODD contract says: project scope,
// topic_key odd/<feature>/tasks, capture_prompt false.
func (o *odd) mirror(feature, content string) saved {
	o.t.Helper()
	r := o.s.tool("mem_save", map[string]any{
		"title":          "ODD mirror: " + feature,
		"content":        content,
		"type":           "architecture",
		"scope":          "project",
		"topic_key":      "odd/" + feature + "/tasks",
		"capture_prompt": false,
	})
	if r.IsError {
		o.t.Fatalf("mem_save failed: %s", r.Text)
	}
	var out saved
	if err := json.Unmarshal([]byte(r.Text), &out); err != nil {
		o.t.Fatalf("mem_save response is not JSON: %v\n%s", err, r.Text)
	}
	if out.ID == 0 {
		o.t.Fatalf("mem_save returned no id: %s", r.Text)
	}
	return out
}

// get returns the stored content as mem_get_observation reports it. The
// "result" text is a header line, the content, then a "Session: ..." trailer,
// so the content is what sits between the header line and the trailer.
func (o *odd) get(id int64) string {
	o.t.Helper()
	r := o.s.tool("mem_get_observation", map[string]any{"id": id})
	if r.IsError {
		o.t.Fatalf("mem_get_observation failed: %s", r.Text)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(r.Text), &out); err != nil {
		o.t.Fatalf("mem_get_observation response is not JSON: %v\n%s", err, r.Text)
	}
	_, rest, ok := strings.Cut(out.Result, "\n")
	end := strings.LastIndex(rest, "\nSession: ")
	if !ok || end < 0 {
		o.t.Fatalf("unexpected mem_get_observation layout: %q", out.Result)
	}
	return rest[:end]
}

// G2: the same topic_key, project and scope upserts (same id); mem_update by
// id replaces the content; mem_get_observation returns what was saved. Upstream
// trims surrounding whitespace on save, so the comparison is against the
// trimmed text (a trailing newline of the local file is not mirrored).
func TestG2UpsertUpdateAndReadBack(t *testing.T) {
	o := newODD(t, "odd-g2")
	first := "# Feature\n\n- [ ] T1 first task\n\nPath: odd/tasks/feature.md\n"
	a := o.mirror("feature", first)
	if got := o.get(a.ID); got != strings.TrimSpace(first) {
		t.Errorf("mem_get_observation = %q, want exactly %q", got, strings.TrimSpace(first))
	}

	second := "# Feature\n\n- [x] T1 first task\n- [ ] T2 second task\n\nPath: odd/tasks/feature.md\n"
	b := o.mirror("feature", second)
	if b.ID != a.ID {
		t.Errorf("same topic_key created observation %d, want upsert of %d", b.ID, a.ID)
	}
	if got := o.get(a.ID); got != strings.TrimSpace(second) {
		t.Errorf("upsert did not overwrite the mirror: got %q, want %q", got, strings.TrimSpace(second))
	}

	third := "# Feature\n\n- [x] T1\n- [x] T2\n\nPath: odd/tasks/feature.md\n"
	u := o.s.tool("mem_update", map[string]any{"id": a.ID, "expected_project": a.Project, "content": third})
	if u.IsError {
		t.Fatalf("mem_update failed: %s", u.Text)
	}
	if got := o.get(a.ID); got != strings.TrimSpace(third) {
		t.Errorf("after mem_update got %q, want %q", got, strings.TrimSpace(third))
	}

	// A different feature is a different topic and a different observation.
	if other := o.mirror("other", "# Other\n"); other.ID == a.ID {
		t.Error("a different topic_key reused the same observation")
	}
}

// G2: capture_prompt is accepted as a boolean and rejected otherwise.
func TestG2CapturePromptParameter(t *testing.T) {
	o := newODD(t, "odd-capture")
	for _, value := range []bool{false, true} {
		r := o.s.tool("mem_save", map[string]any{
			"title": "capture " + fmt.Sprint(value), "content": "x",
			"topic_key": fmt.Sprintf("odd/capture-%v/tasks", value), "capture_prompt": value,
		})
		if r.IsError {
			t.Errorf("capture_prompt=%v rejected: %s", value, r.Text)
		}
	}
	bad := o.s.tool("mem_save", map[string]any{"title": "bad", "content": "x", "capture_prompt": "no"})
	if !bad.IsError || !strings.Contains(bad.Text, "capture_prompt must be a boolean") {
		t.Errorf("non-boolean capture_prompt should be rejected, got error=%v %s", bad.IsError, bad.Text)
	}
}

// bigMirror builds a mirror of n bytes made of distinct numbered lines, so a
// truncation point is visible.
func bigMirror(n int) string {
	var b strings.Builder
	b.WriteString("# Big mirror\n")
	for i := 0; b.Len() < n; i++ {
		fmt.Fprintf(&b, "line %06d: ODD task document body, kept whole\n", i)
	}
	return b.String()[:n]
}

// G3 (finding for Phase 1 H2). Upstream cuts content at 50,000 bytes on save,
// so a larger ODD mirror does not come back whole. The save response reports
// it (truncation.truncated), so the cut is not silent at the protocol level,
// but gentle-ai does not read that field: the mirror loses its tail.
//
// This test pins today's behavior. When H2 lands (no truncation), this test
// must fail and be flipped to assert an exact round trip, and the G3 row of
// docs/INTEGRATION.md updated.
func TestG3LargeMirrorIsTruncatedAtFiftyThousandBytesTodayH2(t *testing.T) {
	o := newODD(t, "odd-g3")

	// Under the limit: exact.
	under := bigMirror(49_000)
	a := o.mirror("under", under)
	if a.Truncation == nil || a.Truncation.Truncated {
		t.Errorf("49,000 bytes should not be truncated: %+v", a.Truncation)
	}
	if o.get(a.ID) != under {
		t.Error("a mirror under the limit did not round-trip exactly")
	}

	// Over the limit: reported, and the tail is gone.
	over := bigMirror(60_000)
	b := o.mirror("over", over)
	if b.Truncation == nil || !b.Truncation.Truncated || b.Truncation.LimitBytes != 50_000 || b.Truncation.OriginalBytes != 60_000 {
		t.Fatalf("expected a reported truncation at 50,000 of 60,000 bytes, got %+v", b.Truncation)
	}
	got := o.get(b.ID)
	if got == over {
		t.Fatal("60,000-byte mirror now round-trips whole: H2 may be fixed; flip this test (see its comment)")
	}
	if len(got) >= len(over) || strings.Contains(got, over[len(over)-200:]) {
		t.Errorf("expected the stored mirror to lose its tail, got %d of %d bytes", len(got), len(over))
	}
	t.Logf("finding G3: a %d-byte mirror is stored as %d bytes (reported via truncation.truncated)", len(over), len(got))
	if !strings.HasPrefix(got, over[:50_000]) || !strings.HasSuffix(got, "... [truncated]") {
		t.Error("expected the first 50,000 bytes kept and a visible \"... [truncated]\" marker")
	}
	// mem_update, the other write ODD uses, applies the same limit.
	u := o.s.tool("mem_update", map[string]any{"id": a.ID, "expected_project": a.Project, "content": over})
	if u.IsError {
		t.Fatalf("mem_update failed: %s", u.Text)
	}
	if updated := o.get(a.ID); updated == over || len(updated) >= len(over) {
		t.Errorf("mem_update of %d bytes was stored as %d bytes; expected the same 50,000-byte limit", len(over), len(updated))
	}
	if !utf8.ValidString(got) {
		t.Error("truncated content is not valid UTF-8")
	}
}

// G5: ODD finds a mirror by its topic and with "/" in the query.
func TestG5SearchFindsMirrorsByTopic(t *testing.T) {
	o := newODD(t, "odd-g5")
	a := o.mirror("qbit-jay-fase-0", "# Phase 0\n\n- [ ] T2 compat\n")
	o.mirror("unrelated", "# Something else\n")

	for _, query := range []string{
		"odd/qbit-jay-fase-0/tasks", // the exact topic_key, with slashes
		"qbit-jay-fase-0",           // the feature name
		"odd/",                      // trailing slash
		"qbit-jay-fase-0/tasks",     // slash inside the query
	} {
		r := o.s.tool("mem_search", map[string]any{"query": query})
		if r.IsError {
			t.Errorf("mem_search %q failed: %s", query, r.Text)
			continue
		}
		var out struct {
			Results []struct {
				ID       int64  `json:"id"`
				TopicKey string `json:"topic_key"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(r.Text), &out); err != nil {
			t.Errorf("mem_search %q: not JSON: %v", query, err)
			continue
		}
		found := false
		for _, hit := range out.Results {
			if hit.ID == a.ID && hit.TopicKey == "odd/qbit-jay-fase-0/tasks" {
				found = true
			}
		}
		if !found {
			t.Errorf("mem_search %q did not return the mirror (#%d): %s", query, a.ID, r.Text)
		}
	}
}

// ── G1: HTTP health probe ───────────────────────────────────────────────────

// gentle-ai's HTTP probe (doctor.go checkEngramHTTP, only with
// ENGRAM_BASE_URL) accepts any 2xx from <base>/health. gentle-engram also
// expects the service name "engram" in the body, which qbit-jay keeps.
func TestG1HTTPHealthEndpoint(t *testing.T) {
	bin := builtBinary(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot reserve a port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	env, _, _ := isolatedEnv(t, fmt.Sprintf("ENGRAM_PORT=%d", port))
	cmd := exec.Command(bin, "serve")
	cmd.Env = env
	cmd.Dir = t.TempDir()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(url) //nolint:gosec,noctx // loopback test server
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var health struct {
				Status  string `json:"status"`
				Service string `json:"service"`
			}
			if jsonErr := json.Unmarshal(body, &health); jsonErr != nil {
				t.Fatalf("health body is not JSON: %v: %s", jsonErr, body)
			}
			// While starting, /health answers 503 {"status":"starting"}; the
			// probe only counts once the server is ready.
			if resp.StatusCode == http.StatusServiceUnavailable && health.Status == "starting" && time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				t.Fatalf("GET /health = %d %s, gentle-ai needs 2xx", resp.StatusCode, body)
			}
			if health.Status != "ok" || health.Service != "engram" {
				t.Errorf("health = %+v, want status ok and service engram (kept protocol identifier)", health)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve did not answer on %s: %v\n%s", url, err, stderr.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func assertEmptyDir(t testing.TB, dir, msg string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("%s: %s now contains %v", msg, dir, names)
	}
}
