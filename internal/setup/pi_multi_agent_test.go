package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Follow-up from T0.4 L18: a flow that sets up several agents must report a Pi
// failure for Pi alone, leave Pi's files as they were, and still install the
// agents around it. The CLI installs one agent per call, so the flow here is
// the loop a caller (or the interactive menu used repeatedly) runs.
func setupAgents(slugs []string) (map[string]error, map[string]*Result) {
	errs := map[string]error{}
	results := map[string]*Result{}
	for _, slug := range slugs {
		res, err := InstallReplacingEngram(slug)
		errs[slug] = err
		results[slug] = res
	}
	return errs, results
}

func TestSetupSeveralAgentsKeepsGoingWhenPiFails(t *testing.T) {
	home := stubRegistryEnv(t)
	isolateDataDir(t)
	BinaryVersion = "0.1.0"
	t.Cleanup(func() { BinaryVersion = "dev" })
	piDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", piDir)
	broken := `{"packages": [oops`
	settings := filepath.Join(piDir, "settings.json")
	if err := os.WriteFile(settings, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	// Pi sits in the middle on purpose: agents before and after it must install.
	errs, results := setupAgents([]string{"cursor", "pi", "windsurf", "gemini-cli"})

	if err := errs["pi"]; err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Fatalf("pi must fail naming its settings file, got %v", err)
	}
	for _, slug := range []string{"cursor", "windsurf", "gemini-cli"} {
		if errs[slug] != nil {
			t.Fatalf("%s must install even though pi failed: %v", slug, errs[slug])
		}
		if results[slug] == nil || results[slug].Agent != slug {
			t.Fatalf("%s result = %+v", slug, results[slug])
		}
	}
	for _, p := range []string{cursorMCPPath(), windsurfMCPPath(), geminiConfigPath()} {
		raw, err := os.ReadFile(p)
		if err != nil || !strings.Contains(string(raw), `"qbit-jay"`) {
			t.Fatalf("%s must hold the qbit-jay entry: %q %v", p, raw, err)
		}
	}
	if got, _ := os.ReadFile(settings); string(got) != broken {
		t.Fatalf("a failed pi setup must leave settings.json as it was: %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".qbit-jay", "pi-plugin")); !os.IsNotExist(err) {
		t.Fatalf("a failed pi setup must not write the plugin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".qbit-jay", "backups", "pi")); !os.IsNotExist(err) {
		t.Fatalf("a failed pi setup must not leave a backup: %v", err)
	}

	// Fixing Pi's settings and running the same flow again completes all of
	// them and changes nothing already done.
	if err := os.WriteFile(settings, []byte(`{"packages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cursorBefore, _ := os.ReadFile(cursorMCPPath())
	errs, _ = setupAgents([]string{"cursor", "pi", "windsurf", "gemini-cli"})
	for slug, err := range errs {
		if err != nil {
			t.Fatalf("second flow: %s: %v", slug, err)
		}
	}
	if cursorAfter, _ := os.ReadFile(cursorMCPPath()); string(cursorAfter) != string(cursorBefore) {
		t.Fatalf("a repeated agent setup must not rewrite its config:\n%s\n%s", cursorBefore, cursorAfter)
	}
	raw, _ := os.ReadFile(settings)
	if !strings.Contains(string(raw), "pi-plugin") || strings.Contains(string(raw), "npm:") {
		t.Fatalf("pi settings after the second flow: %s", raw)
	}
}

func TestSetupSeveralAgentsWithPiRestoresEachIndependently(t *testing.T) {
	stubRegistryEnv(t)
	isolateDataDir(t)
	piDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", piDir)
	settings := filepath.Join(piDir, "settings.json")
	piOriginal := `{"packages":["npm:other"]}`
	cursorOriginal := `{"mcpServers":{"engram":` + engramJSONServer + `}}`
	seedFiles(t, map[string]string{settings: piOriginal, cursorMCPPath(): cursorOriginal})

	errs, _ := setupAgents([]string{"cursor", "pi"})
	for slug, err := range errs {
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
	}
	if _, err := RestoreAgentBackup("pi", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(settings); string(got) != piOriginal {
		t.Fatalf("pi restore = %q", got)
	}
	if got, _ := os.ReadFile(cursorMCPPath()); string(got) == cursorOriginal {
		t.Fatal("restoring pi must not restore cursor")
	}
	if _, err := RestoreAgentBackup("cursor", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(cursorMCPPath()); string(got) != cursorOriginal {
		t.Fatalf("cursor restore = %q", got)
	}
}
