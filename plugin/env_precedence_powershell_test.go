package plugin_test

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// hitServer counts the requests it receives; the PowerShell hooks only ever
// reach it through the port they resolved from the environment.
func hitServer(t *testing.T) (port string, hits *atomic.Int64) {
	t.Helper()
	hits = new(atomic.Int64)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/project/current":
			_, _ = w.Write([]byte(`{"project":"p","project_source":"config"}`))
		case "/prompts/capture-decision":
			_, _ = w.Write([]byte(`{"decision":"skip"}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return strings.TrimPrefix(listener.Addr().String(), "127.0.0.1:"), hits
}

func powershellEnv(env map[string]string) []string {
	var out []string
	for _, entry := range os.Environ() {
		upper := strings.ToUpper(entry)
		if strings.HasPrefix(upper, "ENGRAM_") || strings.HasPrefix(upper, "QBIT_JAY_") || strings.HasPrefix(upper, "PI_CODING_AGENT=") {
			continue
		}
		out = append(out, entry)
	}
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// runPortPrecedence runs matrix cases for PORT against two counting servers.
// The default port (7437) cannot be served by the test, so the default case
// asserts that neither server was reached.
func runPortPrecedence(t *testing.T, run func(t *testing.T, env map[string]string)) {
	t.Helper()
	for _, tt := range precedenceMatrix("PORT", "new", "legacy", "default") {
		t.Run(tt.name, func(t *testing.T) {
			newPort, newHits := hitServer(t)
			legacyPort, legacyHits := hitServer(t)
			env := map[string]string{}
			for k, v := range tt.env {
				switch v {
				case "new":
					v = newPort
				case "legacy":
					v = legacyPort
				}
				env[k] = v
			}
			run(t, env)
			got := [2]bool{newHits.Load() > 0, legacyHits.Load() > 0}
			want := [2]bool{tt.want == "new", tt.want == "legacy"}
			if got != want {
				t.Fatalf("servers reached (new, legacy) = %v, want %v (effective port: %s)", got, want, tt.want)
			}
		})
	}
}

func TestClaudeUserPromptSubmitPowerShellEnvironmentPrecedencePort(t *testing.T) {
	powershellPath := claudeCodePowerShell(t)
	adapterPath := filepath.Join(repoRoot(t), "plugin", "claude-code", "scripts", "user-prompt-submit.ps1")
	runPortPrecedence(t, func(t *testing.T, env map[string]string) {
		cmd := exec.Command(powershellPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", adapterPath)
		cmd.Env = powershellEnv(env)
		input, _ := json.Marshal(map[string]string{"session_id": "t5b-env-" + t.Name(), "cwd": "C:/workspace", "prompt": "hello"})
		cmd.Stdin = strings.NewReader(string(input))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("run adapter: %v: %s", err, output)
		}
		t.Cleanup(func() {
			safe := strings.Map(func(r rune) rune {
				if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
					return r
				}
				return '_'
			}, "t5b-env-"+t.Name())
			_ = os.Remove(filepath.Join(os.TempDir(), "engram-claude-"+safe+"-tools-loaded"))
		})
	})
}

func TestCodexSubagentStopPowerShellEnvironmentPrecedencePort(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows PowerShell")
	}
	adapterPath := filepath.Join(repoRoot(t), "plugin", "codex", "scripts", "subagent-stop.ps1")
	runPortPrecedence(t, func(t *testing.T, env map[string]string) {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", adapterPath)
		cmd.Env = powershellEnv(env)
		cmd.Stdin = strings.NewReader(`{"session_id":"s1","cwd":"C:/workspace","last_assistant_message":"done"}`)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("run adapter: %v: %s", err, output)
		}
	})
}
