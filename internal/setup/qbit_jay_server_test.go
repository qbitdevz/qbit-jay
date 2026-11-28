package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Spec S3 / decision D4: setup registers the MCP server as "qbit-jay" and
// leaves any existing "engram" entry alone (replacing those is T0.5).

const foreignEngramEntry = `{"command":"/old/engram","args":["mcp","--tools=agent"],"marker":"keep-me"}`

func serversAt(t *testing.T, path, topKey string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(cfg[topKey], &servers); err != nil {
		t.Fatalf("parse %s block of %s: %v", topKey, path, err)
	}
	return servers
}

func sameJSON(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(av, bv)
}

func TestDeclarativeAgentsWriteQbitJayServerAndKeepEngramEntry(t *testing.T) {
	for _, agent := range declarativeAgents() {
		t.Run(agent.slug, func(t *testing.T) {
			stubRegistryEnv(t)
			path := agent.mcpPath()
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			seed := `{"` + agent.topKey + `":{"engram":` + foreignEngramEntry + `}}`
			if err := os.WriteFile(path, []byte(seed), 0644); err != nil {
				t.Fatal(err)
			}
			for run := 0; run < 2; run++ {
				if _, err := Install(agent.slug); err != nil {
					t.Fatalf("install run %d: %v", run, err)
				}
			}
			servers := serversAt(t, path, agent.topKey)
			if _, ok := servers["qbit-jay"]; !ok {
				t.Fatalf("%s: no qbit-jay server in %v", agent.slug, servers)
			}
			if !sameJSON(t, servers["engram"], json.RawMessage(foreignEngramEntry)) {
				t.Fatalf("%s: existing engram entry was modified: %s", agent.slug, servers["engram"])
			}
			if len(servers) != 2 {
				t.Fatalf("%s: expected exactly engram and qbit-jay, got %v", agent.slug, servers)
			}
		})
	}
}

func TestGeminiWritesQbitJayServerAndKeepsEngramEntry(t *testing.T) {
	resetSetupSeams(t)
	useIsolatedProfile(t)
	path := geminiConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	seed := `{"mcpServers":{"engram":` + foreignEngramEntry + `}}`
	if err := os.WriteFile(path, []byte(seed), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("gemini-cli"); err != nil {
		t.Fatalf("install: %v", err)
	}
	servers := serversAt(t, path, "mcpServers")
	if _, ok := servers["qbit-jay"]; !ok {
		t.Fatalf("no qbit-jay server: %v", servers)
	}
	if !sameJSON(t, servers["engram"], json.RawMessage(foreignEngramEntry)) {
		t.Fatalf("engram entry modified: %s", servers["engram"])
	}
}

func TestOpenCodeWritesQbitJayServerAndKeepsEngramEntry(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	path := filepath.Join(home, "xdg", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	engram := `{"type":"local","command":["/old/engram","mcp","--tools=agent"],"enabled":true}`
	if err := os.WriteFile(path, []byte(`{"mcp":{"engram":`+engram+`}}`), 0644); err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		if err := injectOpenCodeMCP(); err != nil {
			t.Fatalf("inject run %d: %v", run, err)
		}
	}
	servers := serversAt(t, path, "mcp")
	if _, ok := servers["qbit-jay"]; !ok {
		t.Fatalf("no qbit-jay server: %v", servers)
	}
	if !sameJSON(t, servers["engram"], json.RawMessage(engram)) {
		t.Fatalf("engram entry modified (command must not be rewritten): %s", servers["engram"])
	}
}

func TestCodexWritesQbitJayBlockAndKeepsEngramBlock(t *testing.T) {
	resetSetupSeams(t)
	useIsolatedProfile(t)
	runtimeGOOS = "linux"
	lookPathFn = func(string) (string, error) { return "mock-codex", nil }
	runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }
	path := codexConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	engramBlock := "[mcp_servers.engram]\ncommand = \"/old/engram\"\nargs = [\"mcp\", \"--tools=agent\"]\n"
	seed := "[profile]\nname = \"dev\"\n\n" + engramBlock + "\n[mcp_servers.qbit-jay]\ncommand = \"stale\"\nargs = [\"stale\"]\n"
	if err := os.WriteFile(path, []byte(seed), 0644); err != nil {
		t.Fatal(err)
	}
	var first string
	for run := 0; run < 2; run++ {
		if _, err := Install("codex"); err != nil {
			t.Fatalf("install run %d: %v", run, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = string(raw)
		} else if string(raw) != first {
			t.Fatalf("second run changed config:\n%s\n---\n%s", first, raw)
		}
	}
	if strings.Count(first, "[mcp_servers.qbit-jay]") != 1 {
		t.Fatalf("want exactly one qbit-jay block:\n%s", first)
	}
	if strings.Contains(first, "stale") {
		t.Fatalf("stale qbit-jay block not replaced:\n%s", first)
	}
	if !strings.Contains(first, engramBlock) {
		t.Fatalf("engram block was rewritten or removed:\n%s", first)
	}
	if !strings.Contains(first, "[profile]\nname = \"dev\"") {
		t.Fatalf("unrelated config lost:\n%s", first)
	}
}

func TestCodexRejectsDottedQbitJayRegistration(t *testing.T) {
	_, err := upsertCodexEngramBlock("mcp_servers.qbit-jay.command = \"x\"\n", "qbit-jay")
	if err == nil || !strings.Contains(err.Error(), "[mcp_servers.qbit-jay]") {
		t.Fatalf("expected dotted registration error naming qbit-jay, got %v", err)
	}
	// A dotted engram registration is not ours to judge: it must not block setup.
	if _, err := upsertCodexEngramBlock("mcp_servers.engram.command = \"x\"\n", "qbit-jay"); err != nil {
		t.Fatalf("dotted engram entry must be ignored: %v", err)
	}
}

func TestClaudeCodePermissionsUseQbitJayPrefixesAndKeepEngramRules(t *testing.T) {
	resetSetupSeams(t)
	useTestHome(t)
	path := ClaudeCodeSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"permissions":{"allow":["mcp__engram__mem_search"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := AddClaudeCodeAllowlist(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Permissions struct{ Allow []string } `json:"permissions"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	has := func(rule string) bool {
		for _, a := range got.Permissions.Allow {
			if a == rule {
				return true
			}
		}
		return false
	}
	for _, rule := range []string{"mcp__qbit-jay__mem_search", "mcp__plugin_qbit-jay_qbit-jay__mem_search", "mcp__engram__mem_search"} {
		if !has(rule) {
			t.Fatalf("missing %q in %v", rule, got.Permissions.Allow)
		}
	}
	for _, rule := range got.Permissions.Allow {
		if strings.HasPrefix(rule, "mcp__plugin_engram_") {
			t.Fatalf("setup must not add engram plugin rules: %q", rule)
		}
	}
}

func TestClaudeCodeUserMCPRegistersQbitJayNextToEngram(t *testing.T) {
	resetSetupSeams(t)
	useTestHome(t)
	command := filepath.Join(t.TempDir(), "qbit-jay")
	osExecutable = func() (string, error) { return command, nil }
	lookPathFn = func(string) (string, error) { return "claude", nil }
	engram := `{"type":"stdio","command":"/old/engram","args":["mcp","--tools=agent"]}`
	cfgPath := ClaudeCodeUserMCPPath()
	if err := os.WriteFile(cfgPath, []byte(`{"mcpServers":{"engram":`+engram+`}}`), 0644); err != nil {
		t.Fatal(err)
	}
	var added []string
	run := func(_ string, args ...string) ([]byte, error) {
		added = args
		body := `{"mcpServers":{"engram":` + engram + `,"qbit-jay":{"type":"stdio","command":"` + filepath.ToSlash(command) + `","args":["mcp","--tools=agent"]}}}`
		return nil, os.WriteFile(cfgPath, []byte(body), 0644)
	}
	if err := ensureClaudeCodeUserMCP(run); err != nil {
		t.Fatalf("an existing engram entry must not conflict: %v", err)
	}
	want := []string{"mcp", "add", "--transport", "stdio", "--scope", "user", "qbit-jay", "--", command, "mcp", "--tools=agent"}
	if !reflect.DeepEqual(added, want) {
		t.Fatalf("claude mcp add args:\nwant %v\n got %v", want, added)
	}
}
