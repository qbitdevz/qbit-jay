package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Specs S7, S9: the Claude Code plugin and its marketplace entry carry the
// qbit-jay identity and point at the qbitdevz repository.
func TestClaudePluginAndMarketplaceCarryQbitJayIdentity(t *testing.T) {
	root := repoRoot(t)
	const repo = "https://github.com/qbitdevz/qbit-jay"

	var market struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			Homepage string `json:"homepage"`
			Source   string `json:"source"`
		} `json:"plugins"`
	}
	var plug struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Homepage   string `json:"homepage"`
		Repository string `json:"repository"`
	}
	readJSON := func(path string, into any) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	readJSON(".claude-plugin/marketplace.json", &market)
	readJSON("plugin/claude-code/.claude-plugin/plugin.json", &plug)

	if market.Name != "qbit-jay" || market.Owner.Name != "qbitdevz" {
		t.Errorf("marketplace identity = %q owned by %q, want qbit-jay owned by qbitdevz", market.Name, market.Owner.Name)
	}
	if len(market.Plugins) != 1 || market.Plugins[0].Name != "qbit-jay" || market.Plugins[0].Homepage != repo || market.Plugins[0].Source != "./plugin/claude-code" {
		t.Errorf("marketplace plugin entry = %+v", market.Plugins)
	}
	if plug.Name != "qbit-jay" || plug.Homepage != repo || plug.Repository != repo {
		t.Errorf("plugin.json = %+v, want name qbit-jay and %s", plug, repo)
	}
	// The version rule (CONTRIBUTING, Claude Plugin Version Rule): same version in both
	// files and above the upstream base 0.1.28; CI's claude-plugin-version guard owns the
	// comparison against the real base commit.
	if len(market.Plugins) == 1 && market.Plugins[0].Version != plug.Version {
		t.Errorf("versions differ: marketplace %q, plugin %q", market.Plugins[0].Version, plug.Version)
	}
	if plug.Version == "0.1.28" {
		t.Error("plugin version was not bumped past the upstream 0.1.28")
	}
}

// The hooks that call the binary must name qbit-jay, never an engram executable (S9).
func TestClaudePluginHooksCallQbitJay(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "plugin", "claude-code", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for event, groups := range cfg.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Command == "engram" {
					t.Errorf("%s hook still calls the engram executable", event)
				}
			}
		}
	}
	pre := cfg.Hooks["PreToolUse"]
	if len(pre) != 1 || pre[0].Hooks[0].Command != "qbit-jay" {
		t.Fatalf("PreToolUse hook = %+v, want command qbit-jay", pre)
	}
	for _, prefix := range []string{"mcp__qbit-jay__", "mcp__plugin_qbit-jay_qbit-jay__"} {
		if !strings.Contains(pre[0].Matcher, prefix) {
			t.Errorf("PreToolUse matcher misses %s: %s", prefix, pre[0].Matcher)
		}
	}
}
