package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Pi plugin is not distributed through npm (it will be embedded in the
// binary, T0.9), so setup must never install or pin a registry package for it:
// an unpublished name could be registered by anyone (dependency confusion).
func TestInstallPiRefusesNpmDistributionAndLeavesSettingsUntouched(t *testing.T) {
	cases := map[string]string{
		"gentle-engram pin":  `{"packages":["npm:gentle-engram@0.8.0","npm:existing"],"theme":"dark"}`,
		"old gentle pin":     `{"packages":["npm:gentle-engram@0.7.1"]}`,
		"object entry":       `{"packages":[{"source":"npm:gentle-engram@9.9.9","autoUpdate":false}]}`,
		"no qbit-jay entry":  `{"packages":["./local"]}`,
		"existing settings ": "{ \"keep\" :  true }\n",
	}
	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			resetSetupSeams(t)
			dir := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			var commands []string
			runCommand = func(name string, args ...string) ([]byte, error) {
				commands = append(commands, name+" "+strings.Join(args, " "))
				return nil, nil
			}
			_, err := Install("pi")
			if err == nil {
				t.Fatal("setup pi must fail until the plugin is embedded")
			}
			for _, want := range []string{"not distributed through npm", "pi install <repo>/plugin/pi"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != original {
				t.Errorf("settings changed: %q (%v)", got, readErr)
			}
			if len(commands) != 0 {
				t.Errorf("no command may run, got %v", commands)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "mcp.json")); !os.IsNotExist(statErr) {
				t.Errorf("mcp.json must not be created: %v", statErr)
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Errorf("setup left extra files in the Pi dir: %v", entries)
			}
		})
	}
}

func TestInstallPiWithoutSettingsCreatesNothing(t *testing.T) {
	resetSetupSeams(t)
	dir := filepath.Join(t.TempDir(), "agent")
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	runCommand = func(string, ...string) ([]byte, error) {
		t.Fatal("no command may run")
		return nil, nil
	}
	if _, err := Install("pi"); err == nil {
		t.Fatal("setup pi must fail")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Pi dir must not be created: %v", err)
	}
}

func TestInstallPiNeverProducesQbitJayNpmSource(t *testing.T) {
	resetSetupSeams(t)
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	var seen strings.Builder
	runCommand = func(name string, args ...string) ([]byte, error) {
		seen.WriteString(name + " " + strings.Join(args, " ") + "\n")
		return nil, nil
	}
	_, err := Install("pi")
	if err != nil {
		seen.WriteString(err.Error())
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, file := range files {
		data, _ := os.ReadFile(file)
		seen.Write(data)
	}
	if strings.Contains(seen.String(), "npm:qbit-jay-pi") || strings.Contains(seen.String(), "npm:gentle-engram") {
		t.Fatalf("setup produced an npm source for qbit-jay: %s", seen.String())
	}
}
