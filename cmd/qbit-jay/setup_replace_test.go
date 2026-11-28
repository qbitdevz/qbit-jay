package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qbitdevz/qbit-jay/internal/store"
)

// Spec S3 through the CLI, with a temporary HOME.

func setupHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("QBIT_JAY_DATA_DIR", "")
	t.Setenv("ENGRAM_DATA_DIR", "")
	return home
}

func runSetupCLI(t *testing.T, args ...string) string {
	t.Helper()
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = append([]string{"qbit-jay", "setup"}, args...)
	stdout, _ := captureOutput(t, func() { cmdSetup(store.Config{}) })
	return stdout
}

func TestSetupCLIReplacesEngramAndRestores(t *testing.T) {
	home := setupHome(t)
	cfg := filepath.Join(home, ".cursor", "mcp.json")
	original := `{"mcpServers":{"engram":{"command":"/usr/local/bin/engram","args":["mcp"]},"other":{"command":"keep"}}}`
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runSetupCLI(t, "cursor")
	for _, want := range []string{"Replaced the existing Engram registration", `removed MCP server "engram"`, "Backup: " + filepath.Join(home, ".qbit-jay", "backups", "cursor"), "qbit-jay setup --restore cursor"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
	after, _ := os.ReadFile(cfg)
	if strings.Contains(string(after), `"engram"`) || !strings.Contains(string(after), `"qbit-jay"`) || !strings.Contains(string(after), `"other"`) {
		t.Fatalf("config after setup: %s", after)
	}

	out = runSetupCLI(t, "--restore", "cursor")
	if !strings.Contains(out, "Restored 1 file(s) for cursor") || !strings.Contains(out, cfg) {
		t.Fatalf("restore output: %s", out)
	}
	restored, _ := os.ReadFile(cfg)
	if string(restored) != original {
		t.Fatalf("restore must bring back the exact bytes:\n%s", restored)
	}
}

func TestSetupCLIGentleAIModePrintsOneLineAndChangesNothing(t *testing.T) {
	home := setupHome(t)
	cfg := filepath.Join(home, ".cursor", "mcp.json")
	seed := `{"mcpServers":{"engram":{"command":"/opt/gentle/qbit-jay","args":["mcp","--tools=agent"]}}}`
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runSetupCLI(t, "cursor")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "already runs qbit-jay (gentle-ai mode); nothing was changed") {
		t.Fatalf("expected exactly one notice line, got:\n%s", out)
	}
	if got, _ := os.ReadFile(cfg); string(got) != seed {
		t.Fatalf("config changed: %s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".qbit-jay", "backups")); !os.IsNotExist(err) {
		t.Fatalf("no backup expected: %v", err)
	}
}

func TestSetupRestoreUsageErrors(t *testing.T) {
	setupHome(t)
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"qbit-jay", "setup", "--restore"}
	_, stderr, code := captureExitPanic(t, func() { cmdSetup(store.Config{}) })
	if code == 0 || !strings.Contains(stderr, "usage: qbit-jay setup --restore <agent>") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}
