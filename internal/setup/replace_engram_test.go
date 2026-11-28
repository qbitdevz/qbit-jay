package setup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Spec S3 (6.3 Convivencia, Reversibilidad): setup replaces an existing Engram
// registration after backing up every file it touches; --restore undoes it.

type replaceCase struct {
	slug string
	// prepare isolates the environment and returns path -> seed content.
	prepare func(t *testing.T) map[string]string
	// gone maps a path to text that must no longer be in it after setup.
	gone func() map[string]string
}

const engramJSONServer = `{"command":"/old/engram","args":["mcp","--tools=agent"]}`

func replaceCases() []replaceCase {
	var cases []replaceCase
	for _, agent := range []struct {
		slug, topKey string
		path         func() string
	}{
		{"cursor", "mcpServers", cursorMCPPath},
		{"vscode-copilot", "servers", vscodeMCPPath},
		{"kilocode", "mcp", kilocodeConfigPath},
		{"windsurf", "mcpServers", windsurfMCPPath},
	} {
		agent := agent
		cases = append(cases, replaceCase{
			slug: agent.slug,
			prepare: func(t *testing.T) map[string]string {
				stubRegistryEnv(t)
				return map[string]string{agent.path(): `{"` + agent.topKey + `":{"engram":` + engramJSONServer + `,"other":{"command":"keep"}},"theme":"dark"}`}
			},
			gone: func() map[string]string { return map[string]string{agent.path(): `"engram":`} },
		})
	}
	cases = append(cases,
		replaceCase{
			slug: "gemini-cli",
			prepare: func(t *testing.T) map[string]string {
				resetSetupSeams(t)
				useIsolatedProfile(t)
				runtimeGOOS = "linux"
				return map[string]string{geminiConfigPath(): `{"mcpServers":{"engram":` + engramJSONServer + `}}`}
			},
			gone: func() map[string]string { return map[string]string{geminiConfigPath(): `"engram":`} },
		},
		replaceCase{
			slug: "opencode",
			prepare: func(t *testing.T) map[string]string {
				resetSetupSeams(t)
				home := useTestHome(t)
				runtimeGOOS = "linux"
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
				return map[string]string{
					openCodeConfigPath():                            `{"mcp":{"engram":{"type":"local","command":["/old/engram","mcp"],"enabled":true},"other":{"type":"local","command":["x"]}}}`,
					filepath.Join(openCodePluginDir(), "engram.ts"): "// old engram plugin\n",
				}
			},
			gone: func() map[string]string { return map[string]string{openCodeConfigPath(): `"engram":`} },
		},
		replaceCase{
			slug: "codex",
			prepare: func(t *testing.T) map[string]string {
				resetSetupSeams(t)
				useIsolatedProfile(t)
				runtimeGOOS = "linux"
				lookPathFn = func(string) (string, error) { return "mock-codex", nil }
				runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }
				return map[string]string{codexConfigPath(): "[profile]\nname = \"dev\"\n\n[mcp_servers.engram]\ncommand = \"/old/engram\"\nargs = [\"mcp\"]\n\n[mcp_servers.engram.env]\nA = \"1\"\n\n[mcp_servers.other]\ncommand = \"keep\"\n"}
			},
			gone: func() map[string]string { return map[string]string{codexConfigPath(): "[mcp_servers.engram"} },
		},
		replaceCase{
			slug: "claude-code",
			prepare: func(t *testing.T) map[string]string {
				resetSetupSeams(t)
				useTestHome(t)
				runtimeGOOS = "linux"
				mcpPath := ClaudeCodeUserMCPPath()
				lookPathFn = func(string) (string, error) { return "claude", nil }
				runCommand = func(_ string, args ...string) ([]byte, error) {
					// The claude CLI owns ~/.claude.json: emulate `mcp remove`.
					if len(args) >= 2 && args[0] == "mcp" && args[1] == "remove" {
						return []byte("removed"), os.WriteFile(mcpPath, []byte(`{"mcpServers":{"other":{"command":"keep"}}}`), 0o644)
					}
					return []byte("ok"), nil
				}
				writeClaudeCodeUserMCPFn = func(claudeSetupRunner) error { return nil }
				return map[string]string{
					mcpPath:                  `{"mcpServers":{"engram":` + engramJSONServer + `,"other":{"command":"keep"}}}`,
					ClaudeCodeSettingsPath(): `{"permissions":{"allow":["mcp__engram__mem_search","mcp__plugin_engram_engram__mem_save","Bash(ls)"]},"enabledPlugins":{"engram@engram":true,"x@y":true}}`,
				}
			},
			gone: func() map[string]string {
				return map[string]string{ClaudeCodeSettingsPath(): "mcp__engram__", ClaudeCodeUserMCPPath(): `"engram":`}
			},
		},
	)
	return cases
}

func seedFiles(t *testing.T, seed map[string]string) {
	t.Helper()
	for p, body := range seed {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func backupDirs(t *testing.T, slug string) []string {
	t.Helper()
	root, err := BackupRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, slug))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, filepath.Join(root, slug, e.Name()))
	}
	return out
}

func isolateDataDir(t *testing.T) {
	t.Setenv("QBIT_JAY_DATA_DIR", "")
	t.Setenv("ENGRAM_DATA_DIR", "")
}

func TestSetupReplacesEngramEntryBacksUpAndRestoresExactBytes(t *testing.T) {
	for _, tc := range replaceCases() {
		t.Run(tc.slug, func(t *testing.T) {
			isolateDataDir(t)
			seed := tc.prepare(t)
			seedFiles(t, seed)
			tick := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
			backupNow = func() time.Time { tick = tick.Add(time.Second); return tick }
			t.Cleanup(func() { backupNow = time.Now })

			res, err := InstallReplacingEngram(tc.slug)
			if err != nil {
				t.Fatalf("setup: %v", err)
			}
			if len(res.Replaced) == 0 || res.BackupDir == "" {
				t.Fatalf("expected a replacement report and backup dir, got %+v", res)
			}
			for path, text := range tc.gone() {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), text) {
					t.Fatalf("%s still contains %q:\n%s", path, text, raw)
				}
			}
			// The backup holds the exact original bytes and a manifest with sha256.
			rawManifest, err := os.ReadFile(filepath.Join(res.BackupDir, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest BackupManifest
			if err := json.Unmarshal(rawManifest, &manifest); err != nil {
				t.Fatal(err)
			}
			for p, body := range seed {
				var entry *BackupEntry
				for i := range manifest.Files {
					if manifest.Files[i].Path == p {
						entry = &manifest.Files[i]
					}
				}
				if entry == nil {
					t.Fatalf("manifest has no entry for %s: %s", p, rawManifest)
				}
				sum := sha256.Sum256([]byte(body))
				if entry.SHA256 != fmt.Sprintf("%x", sum) {
					t.Fatalf("manifest sha256 for %s = %s", p, entry.SHA256)
				}
				saved, err := os.ReadFile(filepath.Join(res.BackupDir, entry.File))
				if err != nil || string(saved) != body {
					t.Fatalf("backup of %s differs: %q %v", p, saved, err)
				}
			}

			// Idempotent: a second setup finds no Engram entry and adds no backup.
			again, err := InstallReplacingEngram(tc.slug)
			if err != nil {
				t.Fatalf("second setup: %v", err)
			}
			if len(again.Replaced) != 0 || again.BackupDir != "" {
				t.Fatalf("second setup must be a no-op replacement, got %+v", again.Replaced)
			}
			if n := len(backupDirs(t, tc.slug)); n != 1 {
				t.Fatalf("want 1 backup after two setups, got %d", n)
			}

			// Restore brings back the exact original bytes (including deleted files).
			restored, err := RestoreAgentBackup(tc.slug)
			if err != nil {
				t.Fatalf("restore: %v", err)
			}
			if len(restored.Restored) != len(seed) {
				t.Fatalf("restored %v, want %d files", restored.Restored, len(seed))
			}
			for p, body := range seed {
				raw, err := os.ReadFile(p)
				if err != nil || string(raw) != body {
					t.Fatalf("%s after restore = %q (%v), want original %q", p, raw, err, body)
				}
			}

			// Setting up again over the restored files reuses the identical backup.
			third, err := InstallReplacingEngram(tc.slug)
			if err != nil {
				t.Fatalf("third setup: %v", err)
			}
			if !third.BackupReused || third.BackupDir != res.BackupDir {
				t.Fatalf("expected the identical backup to be reused, got %+v", third)
			}
			if n := len(backupDirs(t, tc.slug)); n != 1 {
				t.Fatalf("want 1 backup, got %d", n)
			}
		})
	}
}

func TestSetupWithoutEngramEntryMakesNoBackup(t *testing.T) {
	isolateDataDir(t)
	stubRegistryEnv(t)
	if _, err := InstallReplacingEngram("cursor"); err != nil {
		t.Fatal(err)
	}
	if n := len(backupDirs(t, "cursor")); n != 0 {
		t.Fatalf("no Engram entry means no backup, got %d", n)
	}
}

func TestRestoreRefusesCorruptBackupAndUnknownAgent(t *testing.T) {
	isolateDataDir(t)
	stubRegistryEnv(t)
	path := cursorMCPPath()
	seedFiles(t, map[string]string{path: `{"mcpServers":{"engram":` + engramJSONServer + `}}`})
	res, err := InstallReplacingEngram("cursor")
	if err != nil {
		t.Fatal(err)
	}
	modified, _ := os.ReadFile(path)
	entries, _ := os.ReadDir(res.BackupDir)
	for _, e := range entries {
		if e.Name() != "manifest.json" {
			if err := os.WriteFile(filepath.Join(res.BackupDir, e.Name()), []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := RestoreAgentBackup("cursor"); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("expected a sha256 mismatch error, got %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(modified) {
		t.Fatal("a corrupt backup must not be written back")
	}
	if _, err := RestoreAgentBackup("nope"); err == nil {
		t.Fatal("unknown agent must fail")
	}
	if _, err := RestoreAgentBackup("windsurf"); err == nil || !strings.Contains(err.Error(), "no backup") {
		t.Fatalf("expected no backup error, got %v", err)
	}
}

func TestInstallReplacingEngramPiRefusesAndTouchesNothing(t *testing.T) {
	resetSetupSeams(t)
	useTestHome(t)
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	seed := map[string]string{
		filepath.Join(dir, "mcp.json"):      `{"mcpServers":{"engram":{"command":"engram"}}}`,
		filepath.Join(dir, "settings.json"): `{"packages":["npm:gentle-engram@0.7.1"]}`,
	}
	seedFiles(t, seed)
	if _, err := InstallReplacingEngram("pi"); err == nil {
		t.Fatal("setup pi must refuse")
	}
	for p, want := range seed {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != want {
			t.Fatalf("%s changed: %q %v", p, got, err)
		}
	}
	if root, err := BackupRoot(); err == nil {
		if _, err := os.Stat(root); err == nil {
			t.Fatalf("no backup expected for Pi, found %s", root)
		}
	}
}
