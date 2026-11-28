package compat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// moduleRoot returns the repository root (two levels above internal/qbit).
func moduleRoot(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

var (
	buildOnce sync.Once
	buildDir  string
	buildErr  error
)

// builtBinary builds cmd/qbit-jay once per test run, stamped with the
// release-style version 0.1.0 exactly like .goreleaser.yaml does
// (-X main.version=...). The binary lives outside the repository.
func builtBinary(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("compat tests build and run the real binary; skipped with -short")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "qbit-jay-compat-")
		if err != nil {
			buildErr = err
			return
		}
		buildDir = dir
		name := "qbit-jay"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		cmd := exec.Command("go", "build", "-ldflags", "-X main.version=0.1.0", "-o", filepath.Join(dir, name), "./cmd/qbit-jay")
		cmd.Dir = moduleRoot(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/qbit-jay: %w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	name := "qbit-jay"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(buildDir, name)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// isolatedEnv returns an environment with a throwaway HOME and data dir, and
// none of the caller's ENGRAM_* or QBIT_JAY_* variables. Variables are set the
// way gentle-ai sets them (ENGRAM_* names), so the legacy fallback is what is
// under test. extra entries (KEY=VALUE) win.
func isolatedEnv(t testing.TB, extra ...string) (env []string, home, dataDir string) {
	t.Helper()
	home = t.TempDir()
	dataDir = filepath.Join(t.TempDir(), "data")
	for _, kv := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(kv, "=", 2)[0])
		if strings.HasPrefix(key, "ENGRAM_") || strings.HasPrefix(key, "QBIT_JAY_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"HOME="+home,
		"USERPROFILE="+home,
		"ENGRAM_NO_UPDATE_CHECK=1",
		"ENGRAM_DATA_DIR="+dataDir,
	)
	return append(env, extra...), home, dataDir
}

// runOutput runs the binary with stdin detached and returns stdout, stderr and
// the exit error, with a hard deadline.
func runOutput(t testing.TB, bin string, env []string, timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Stdin = nil
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %v did not finish within %s", filepath.Base(bin), args, timeout)
	}
	return out.String(), errb.String(), err
}

// mcpSession is a minimal newline-delimited JSON-RPC stdio client.
type mcpSession struct {
	t      testing.TB
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	nextID int
	stderr *bytes.Buffer
}

func startMCP(t testing.TB, bin, dir string, env []string, args ...string) *mcpSession {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &mcpSession{t: t, cmd: cmd, stdin: stdin, lines: make(chan string, 64), nextID: 1, stderr: stderr}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
		close(s.lines)
	}()
	t.Cleanup(func() { s.close() })
	return s
}

// close ends the session the way an agent does: stdin EOF, then wait.
func (s *mcpSession) close() {
	_ = s.stdin.Close()
	done := make(chan struct{})
	go func() { _ = s.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}

func (s *mcpSession) send(method string, id *int, params any) {
	s.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		msg["id"] = *id
	}
	if params != nil {
		msg["params"] = params
	}
	b, _ := json.Marshal(msg)
	if _, err := s.stdin.Write(append(b, '\n')); err != nil {
		s.t.Fatalf("write %s: %v", method, err)
	}
}

// call sends a request and returns the raw JSON-RPC response with a matching id.
func (s *mcpSession) call(method string, params any) json.RawMessage {
	s.t.Helper()
	id := s.nextID
	s.nextID++
	s.send(method, &id, params)
	deadline := time.After(30 * time.Second)
	for {
		select {
		case line, ok := <-s.lines:
			if !ok {
				s.t.Fatalf("mcp exited before answering %s; stderr: %s", method, s.stderr.String())
			}
			var resp struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal([]byte(line), &resp) == nil && string(bytes.TrimSpace(resp.ID)) == fmt.Sprint(id) {
				return json.RawMessage(line)
			}
		case <-deadline:
			s.t.Fatalf("timed out waiting for %s", method)
		}
	}
}

func (s *mcpSession) initialize() {
	s.t.Helper()
	resp := s.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "compat-test", "version": "0"},
	})
	if !bytes.Contains(resp, []byte(`"result"`)) {
		s.t.Fatalf("initialize failed: %s", resp)
	}
	s.send("notifications/initialized", nil, nil)
}

type toolResult struct {
	Text    string
	IsError bool
}

// tool calls an MCP tool and returns the concatenated text content.
func (s *mcpSession) tool(name string, args map[string]any) toolResult {
	s.t.Helper()
	raw := s.call("tools/call", map[string]any{"name": name, "arguments": args})
	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		s.t.Fatalf("decode %s response: %v\n%s", name, err, raw)
	}
	if len(resp.Error) > 0 {
		s.t.Fatalf("%s returned a JSON-RPC error: %s", name, resp.Error)
	}
	var sb strings.Builder
	for _, c := range resp.Result.Content {
		sb.WriteString(c.Text)
	}
	return toolResult{Text: sb.String(), IsError: resp.Result.IsError}
}
