package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gentle-ai compatibility mode (GENTLE_AI_ENGRAM_BIN=/abs/qbit-jay): gentle-ai
// writes an MCP entry named "engram" whose command is the qbit-jay binary and
// then runs `qbit-jay setup <agent>`. Setup must leave that entry alone, add
// nothing and say so; only an "engram" entry running a real Engram is replaced.

func TestSetupLeavesEngramEntryThatRunsQbitJayAlone(t *testing.T) {
	cases := map[string]string{
		"basename":           `/opt/tools/qbit-jay`,
		"windows exe":        `C:\\Tools\\qbit-jay.exe`,
		"bare name":          `qbit-jay`,
		"same as this exe":   `/custom/wrapper/gentle-bin`,
		"basename uppercase": `/opt/tools/QBIT-JAY.EXE`,
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			isolateDataDir(t)
			stubRegistryEnv(t)
			osExecutable = func() (string, error) { return filepath.FromSlash("/custom/wrapper/gentle-bin"), nil }
			path := cursorMCPPath()
			seed := `{"mcpServers":{"engram":{"command":"` + command + `","args":["mcp","--tools=agent"]}}}`
			seedFiles(t, map[string]string{path: seed})

			res, err := InstallReplacingEngram("cursor")
			if err != nil {
				t.Fatal(err)
			}
			if !res.GentleAIMode || len(res.Replaced) != 0 || res.BackupDir != "" {
				t.Fatalf("expected gentle-ai mode without replacement, got %+v", res)
			}
			if !strings.Contains(res.Notice, "gentle-ai mode") || !strings.Contains(res.Notice, "nothing was changed") || strings.Count(res.Notice, "\n") != 0 {
				t.Fatalf("expected one notice line, got %q", res.Notice)
			}
			got, _ := os.ReadFile(path)
			if string(got) != seed {
				t.Fatalf("config changed:\n%s", got)
			}
			if _, err := os.Stat(cursorMemoryProtocolPath()); !os.IsNotExist(err) {
				t.Fatalf("nothing else may be written (instruction file): %v", err)
			}
			if n := len(backupDirs(t, "cursor")); n != 0 {
				t.Fatalf("no backup expected, got %d", n)
			}
		})
	}
}

func TestSetupStillReplacesEngramEntryThatRunsRealEngram(t *testing.T) {
	for name, command := range map[string]string{
		"engram":                `/usr/local/bin/engram`,
		"engram exe":            `C:\\Engram\\engram.exe`,
		"lookalike qbit-jay-x":  `/opt/qbit-jay-old/engram-helper`,
		"qbit-jay as directory": `/opt/qbit-jay/engram`,
	} {
		t.Run(name, func(t *testing.T) {
			isolateDataDir(t)
			stubRegistryEnv(t)
			osExecutable = func() (string, error) { return filepath.FromSlash("/home/me/bin/qbit-jay"), nil }
			path := cursorMCPPath()
			seedFiles(t, map[string]string{path: `{"mcpServers":{"engram":{"command":"` + command + `","args":["mcp"]}}}`})
			res, err := InstallReplacingEngram("cursor")
			if err != nil {
				t.Fatal(err)
			}
			if res.GentleAIMode || len(res.Replaced) != 1 || res.BackupDir == "" {
				t.Fatalf("a real Engram entry must be replaced with a backup, got %+v", res)
			}
		})
	}
}

func TestGentleAIModeCoversCodexClaudeAndOpenCode(t *testing.T) {
	t.Run("codex", func(t *testing.T) {
		isolateDataDir(t)
		resetSetupSeams(t)
		useIsolatedProfile(t)
		runtimeGOOS = "linux"
		var calls []string
		lookPathFn = func(string) (string, error) { return "mock-codex", nil }
		runCommand = func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return []byte("ok"), nil
		}
		path := codexConfigPath()
		seed := "[mcp_servers.engram]\ncommand = \"/opt/bin/qbit-jay\"\nargs = [\"mcp\"]\n"
		seedFiles(t, map[string]string{path: seed})
		res, err := InstallReplacingEngram("codex")
		if err != nil || !res.GentleAIMode {
			t.Fatalf("got %+v, %v", res, err)
		}
		if got, _ := os.ReadFile(path); string(got) != seed {
			t.Fatalf("config changed:\n%s", got)
		}
		if len(calls) != 0 {
			t.Fatalf("no command may run in gentle-ai mode: %v", calls)
		}
	})
	t.Run("claude-code", func(t *testing.T) {
		isolateDataDir(t)
		resetSetupSeams(t)
		useTestHome(t)
		runtimeGOOS = "linux"
		var calls []string
		lookPathFn = func(string) (string, error) { return "claude", nil }
		runCommand = func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return []byte("ok"), nil
		}
		seed := `{"mcpServers":{"engram":{"command":"/opt/bin/qbit-jay","args":["mcp","--tools=agent"]}}}`
		settings := `{"permissions":{"allow":["mcp__engram__mem_search"]}}`
		seedFiles(t, map[string]string{ClaudeCodeUserMCPPath(): seed, ClaudeCodeSettingsPath(): settings})
		res, err := InstallReplacingEngram("claude-code")
		if err != nil || !res.GentleAIMode {
			t.Fatalf("got %+v, %v", res, err)
		}
		if got, _ := os.ReadFile(ClaudeCodeSettingsPath()); string(got) != settings {
			t.Fatalf("the mcp__engram__ permissions belong to gentle-ai mode and stay: %s", got)
		}
		if len(calls) != 0 {
			t.Fatalf("no command may run in gentle-ai mode: %v", calls)
		}
	})
	t.Run("opencode array command", func(t *testing.T) {
		isolateDataDir(t)
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
		seed := `{"mcp":{"engram":{"type":"local","command":["/opt/bin/qbit-jay","mcp","--tools=agent"],"enabled":true}}}`
		seedFiles(t, map[string]string{openCodeConfigPath(): seed})
		res, err := InstallReplacingEngram("opencode")
		if err != nil || !res.GentleAIMode {
			t.Fatalf("got %+v, %v", res, err)
		}
		if got, _ := os.ReadFile(openCodeConfigPath()); string(got) != seed {
			t.Fatalf("config changed:\n%s", got)
		}
		if _, err := os.Stat(filepath.Join(openCodePluginDir(), "qbit-jay.ts")); !os.IsNotExist(err) {
			t.Fatalf("the plugin must not be installed in gentle-ai mode: %v", err)
		}
	})
}
