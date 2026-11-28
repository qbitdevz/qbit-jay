package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Specs S7, S9: the Codex plugin carries the qbit-jay identity, its hooks and
// scripts call the qbit-jay binary, and no script invokes an engram executable.
func TestCodexPluginCarriesQbitJayIdentity(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "plugin", "codex", ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plug struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Homepage    string `json:"homepage"`
		Repository  string `json:"repository"`
		Author      struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &plug); err != nil {
		t.Fatal(err)
	}
	const repo = "https://github.com/qbitdevz/qbit-jay"
	if plug.Name != "qbit-jay" || plug.Homepage != repo || plug.Repository != repo || plug.Author.Name != "qbitdevz" {
		t.Errorf("plugin.json = %+v, want name qbit-jay, author qbitdevz and %s", plug, repo)
	}
	if !strings.Contains(plug.DisplayName, "qbit-jay") {
		t.Errorf("displayName %q must name qbit-jay", plug.DisplayName)
	}
}

func TestCodexPluginHooksCallQbitJay(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "plugin", "codex", "hooks", "hooks.json"))
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
	pre := cfg.Hooks["PreToolUse"]
	if len(pre) != 1 || pre[0].Hooks[0].Command != "qbit-jay hook codex-pre-tool-use" {
		t.Fatalf("PreToolUse hook = %+v, want command qbit-jay hook codex-pre-tool-use", pre)
	}
	for _, prefix := range []string{"mcp__qbit-jay__mem_*", "mcp__plugin_qbit-jay_qbit-jay__mem_*", "mcp__engram__mem_*", "mcp__plugin_engram_engram__mem_*"} {
		if !strings.Contains(pre[0].Matcher, prefix) {
			t.Errorf("PreToolUse matcher misses %s: %s", prefix, pre[0].Matcher)
		}
	}
}

// No Codex script may execute an engram binary: the brand policy protects that
// name as a CLI identifier too (spec S9).
func TestCodexPluginScriptsNeverExecuteEngramBinary(t *testing.T) {
	root := repoRoot(t)
	call := regexp.MustCompile(`(^|[|;&(\x60]|\$\(|\s)(& )?engram (hook|serve|serve-background|instance-id|sync|mcp|setup|run)\b`)
	files, err := filepath.Glob(filepath.Join(root, "plugin", "codex", "scripts", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if call.MatchString(line) && !strings.Contains(line, "usage: engram hook") {
				t.Errorf("%s:%d still executes engram: %s", filepath.Base(file), i+1, trimmed)
			}
		}
	}
}
