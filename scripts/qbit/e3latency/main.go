// Command e3latency is the black-box half of the E3 performance evaluator.
//
// It drives a memory binary only through its public surfaces (the CLI and the
// MCP stdio server), so the same program measures upstream and the fork. It
// seeds a deterministic synthetic corpus through mem_save and reports p50/p95
// latency for mem_save, mem_search and mem_context, MCP startup time, peak
// resident memory, and the resulting database size.
//
// Usage:
//
//	go run ./scripts/qbit/e3latency -bin /path/to/binary [-env-prefix ENGRAM]
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

type server struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	nextID int
}

func startServer(bin string, args, env []string, dir string) (*server, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &server{cmd: cmd, stdin: stdin, stdout: bufio.NewReaderSize(stdout, 1<<20)}, nil
}

func (s *server) call(method string, params any) (json.RawMessage, error) {
	s.nextID++
	id := s.nextID
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if _, err := s.stdin.Write(append(payload, '\n')); err != nil {
		return nil, err
	}
	for {
		line, err := s.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("read %s response: %w", method, err)
		}
		var resp rpcResponse
		if json.Unmarshal(line, &resp) != nil || resp.ID == nil || *resp.ID != id {
			continue // notification or unrelated line
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: rpc error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (s *server) notify(method string) error {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method})
	if err != nil {
		return err
	}
	_, err = s.stdin.Write(append(payload, '\n'))
	return err
}

func (s *server) initialize() error {
	_, err := s.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "e3latency", "version": "0"},
	})
	if err != nil {
		return err
	}
	return s.notify("notifications/initialized")
}

// tool calls one MCP tool and returns its text, the response size in bytes and
// the elapsed time. A tool-level error is returned as an error.
func (s *server) tool(name string, args map[string]any) (string, time.Duration, error) {
	start := time.Now()
	raw, err := s.call("tools/call", map[string]any{"name": name, "arguments": args})
	elapsed := time.Since(start)
	if err != nil {
		return "", elapsed, err
	}
	var res toolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", elapsed, err
	}
	var text strings.Builder
	for _, c := range res.Content {
		text.WriteString(c.Text)
	}
	if res.IsError {
		return text.String(), elapsed, fmt.Errorf("%s: tool error: %.200s", name, text.String())
	}
	return text.String(), elapsed, nil
}

// peakRSSKB reads the high-water resident set size of the server (Linux only).
func (s *server) peakRSSKB() int {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(s.cmd.Process.Pid), "status"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.Atoi(fields[1])
				return kb
			}
		}
	}
	return 0
}

func (s *server) close() {
	_ = s.stdin.Close()
	done := make(chan struct{})
	go func() { _ = s.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}

type stats struct {
	N      int     `json:"n"`
	Errors int     `json:"errors"`
	P50ms  float64 `json:"p50_ms"`
	P95ms  float64 `json:"p95_ms"`
	MaxMs  float64 `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
}

func summarize(samples []time.Duration, errors int) stats {
	st := stats{N: len(samples), Errors: errors}
	if len(samples) == 0 {
		return st
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := func(p float64) time.Duration { // nearest-rank percentile
		idx := int(float64(len(sorted))*p+0.999999) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	}
	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	st.P50ms, st.P95ms, st.MaxMs = ms(rank(0.50)), ms(rank(0.95)), ms(sorted[len(sorted)-1])
	st.MeanMs = ms(total / time.Duration(len(sorted)))
	return st
}

var vocabulary = strings.Fields(`auth token session cache index migration schema query ranking context
search memory observation project branch commit merge conflict deploy release pipeline workflow
handler router middleware endpoint payload request response timeout retry backoff queue worker
database sqlite postgres transaction rollback checkpoint replication sync chunk cursor pagination
render component layout theme palette typography spacing dashboard table filter chart modal
parser lexer tokenizer trigram embedding vector similarity threshold budget truncate preview
config environment variable secret credential permission guard sandbox daemon socket port
plugin hook agent subagent orchestrator prompt summary decision bugfix pattern discovery
latency benchmark baseline regression profile allocation goroutine mutex channel deadlock
windows linux macos docker container image volume network proxy tunnel certificate`)

var memoryTypes = []string{"decision", "bugfix", "discovery", "pattern", "architecture", "config", "preference"}

func words(r *rand.Rand, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = vocabulary[r.Intn(len(vocabulary))]
	}
	return strings.Join(parts, " ")
}

// document builds a memory body of roughly targetChars characters, including a
// file path in about half of them, as the audited corpus does.
func document(r *rand.Rand, targetChars int) string {
	var b strings.Builder
	for b.Len() < targetChars {
		b.WriteString("**What**: " + words(r, 12) + ".\n")
		b.WriteString("**Why**: " + words(r, 16) + ".\n")
		if r.Intn(2) == 0 {
			fmt.Fprintf(&b, "**Where**: internal/%s/%s.go\n", vocabulary[r.Intn(len(vocabulary))], vocabulary[r.Intn(len(vocabulary))])
		}
		b.WriteString("**Learned**: " + words(r, 20) + ".\n\n")
	}
	return b.String()
}

func main() {
	bin := flag.String("bin", "", "path to the binary under test (required)")
	prefix := flag.String("env-prefix", "ENGRAM", "environment variable prefix of the binary")
	profile := flag.String("tools", "agent", "MCP tool profile")
	memories := flag.Int("memories", 2300, "synthetic memories to save")
	searches := flag.Int("searches", 300, "mem_search calls")
	contexts := flag.Int("contexts", 100, "mem_context calls")
	startups := flag.Int("startups", 30, "cold MCP and CLI starts to time")
	seed := flag.Int64("seed", 20261126, "corpus seed")
	jsonOut := flag.String("json", "", "write the report as JSON to this file")
	flag.Parse()
	if *bin == "" {
		flag.Usage()
		os.Exit(2)
	}

	dataDir, err := os.MkdirTemp("", "e3latency-data-")
	if err != nil {
		fatal(err)
	}
	defer func() { _ = os.RemoveAll(dataDir) }()
	workParent, err := os.MkdirTemp("", "e3latency-cwd-")
	if err != nil {
		fatal(err)
	}
	defer func() { _ = os.RemoveAll(workParent) }()

	// The server rejects an explicit project it has no context for, so every
	// call omits it and lets the server detect the project from the working
	// directory, which is named after the project.
	const project = "e3-bench"
	workDir := filepath.Join(workParent, project)
	if err := os.Mkdir(workDir, 0o755); err != nil {
		fatal(err)
	}
	env := append(os.Environ(),
		*prefix+"_DATA_DIR="+dataDir,
		*prefix+"_NO_UPDATE_CHECK=1",
		*prefix+"_CLOUD_AUTOSYNC=0",
	)
	mcpArgs := []string{"mcp", "--tools=" + *profile}
	report := map[string]any{"binary": *bin, "profile": *profile, "memories": *memories, "seed": *seed}

	// Workload on one long-lived MCP server, as an agent session uses it.
	srv, err := startServer(*bin, mcpArgs, env, workDir)
	if err != nil {
		fatal(err)
	}
	if err := srv.initialize(); err != nil {
		fatal(err)
	}
	if raw, err := srv.call("tools/list", map[string]any{}); err == nil {
		var listed struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if json.Unmarshal(raw, &listed) == nil {
			names := make([]string, 0, len(listed.Tools))
			for _, t := range listed.Tools {
				names = append(names, t.Name)
			}
			sort.Strings(names)
			report["tools"] = names
			report["tools_list_bytes"] = len(raw)
		}
	}

	r := rand.New(rand.NewSource(*seed))
	var firstError string
	note := func(err error) {
		if err != nil && firstError == "" {
			firstError = err.Error()
		}
	}

	var saveTimes []time.Duration
	saveErrors, savedChars := 0, 0
	for i := 0; i < *memories; i++ {
		body := document(r, 400+r.Intn(2800)) // mean near the audited 1,750 characters
		args := map[string]any{
			"title":   words(r, 5),
			"content": body,
			"type":    memoryTypes[r.Intn(len(memoryTypes))],
			"scope":   "project",
		}
		if r.Intn(100) < 79 { // 79 % of audited memories carry a topic_key
			args["topic_key"] = fmt.Sprintf("%s/%s-%d", vocabulary[r.Intn(len(vocabulary))], vocabulary[r.Intn(len(vocabulary))], r.Intn(1500))
		}
		_, elapsed, err := srv.tool("mem_save", args)
		if err != nil {
			saveErrors++
			note(err)
			continue
		}
		saveTimes = append(saveTimes, elapsed)
		savedChars += len(body)
	}
	report["mem_save"] = summarize(saveTimes, saveErrors)
	report["saved_content_bytes"] = savedChars

	var searchTimes []time.Duration
	searchErrors, searchBytes := 0, 0
	for i := 0; i < *searches; i++ {
		args := map[string]any{"query": words(r, 1+r.Intn(3))}
		if i%4 == 0 {
			args["match_mode"] = "any"
		}
		text, elapsed, err := srv.tool("mem_search", args)
		if err != nil {
			searchErrors++
			note(err)
			continue
		}
		searchTimes = append(searchTimes, elapsed)
		searchBytes += len(text)
	}
	report["mem_search"] = summarize(searchTimes, searchErrors)
	if len(searchTimes) > 0 {
		report["mem_search_mean_response_bytes"] = searchBytes / len(searchTimes)
	}

	var contextTimes []time.Duration
	contextErrors, contextBytes := 0, 0
	for i := 0; i < *contexts; i++ {
		text, elapsed, err := srv.tool("mem_context", map[string]any{})
		if err != nil {
			contextErrors++
			note(err)
			continue
		}
		contextTimes = append(contextTimes, elapsed)
		contextBytes = len(text)
	}
	report["mem_context"] = summarize(contextTimes, contextErrors)
	report["mem_context_response_bytes"] = contextBytes
	report["mem_context_estimated_tokens"] = contextBytes / 4
	report["rss_peak_kb_after_workload"] = srv.peakRSSKB()
	srv.close()
	if firstError != "" {
		report["first_error"] = firstError
	}

	var dbBytes int64
	_ = filepath.Walk(dataDir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			dbBytes += info.Size()
		}
		return nil
	})
	report["data_dir_bytes"] = dbBytes

	// Cold MCP start against the seeded database: spawn to initialize response.
	var startTimes []time.Duration
	startErrors, idleRSS := 0, 0
	for i := 0; i < *startups; i++ {
		begin := time.Now()
		s, err := startServer(*bin, mcpArgs, env, workDir)
		if err != nil {
			startErrors++
			continue
		}
		if err := s.initialize(); err != nil {
			startErrors++
			s.close()
			continue
		}
		startTimes = append(startTimes, time.Since(begin))
		idleRSS = s.peakRSSKB()
		s.close()
	}
	report["mcp_startup"] = summarize(startTimes, startErrors)
	report["rss_peak_kb_after_initialize"] = idleRSS

	// Cold CLI start: the cheapest command, as hooks and health probes pay it.
	var cliTimes []time.Duration
	cliErrors := 0
	for i := 0; i < *startups; i++ {
		cmd := exec.Command(*bin, "version")
		cmd.Env = env
		cmd.Dir = workDir
		begin := time.Now()
		if err := cmd.Run(); err != nil {
			cliErrors++
			continue
		}
		cliTimes = append(cliTimes, time.Since(begin))
	}
	report["cli_version_startup"] = summarize(cliTimes, cliErrors)

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, append(encoded, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	fmt.Println(string(encoded))
	if firstError != "" || startErrors > 0 || cliErrors > 0 {
		fmt.Fprintln(os.Stderr, "e3latency: some calls failed; the report is incomplete")
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "e3latency:", err)
	os.Exit(1)
}
