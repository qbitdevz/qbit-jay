package setup

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// T0.9 (S12 as changed by L4): the Pi plugin is embedded in the binary.
// `setup pi` writes it under <data dir>/pi-plugin/<version>/ and registers that
// absolute path in Pi's settings `packages`. No npm source is ever written.

// piPluginFiles is what Pi loads: everything in plugin/pi except tests,
// node_modules and the lock file.
var piPluginFiles = []string{
	"README.md",
	"assets/qbit-jay-isotype-256.png",
	"cli.js",
	"compaction-recovery.js",
	"index.ts",
	"mcp-template.json",
	"memory-tool-chrome.js",
	"package.json",
	"private-redaction.js",
	"runtime-contract.js",
	"runtime-contract.json",
}

type piEnv struct {
	agentDir string
	settings string
	dataDir  string
}

func newPiEnv(t *testing.T, settings string) piEnv {
	t.Helper()
	resetSetupSeams(t)
	useTestHome(t)
	isolateDataDir(t)
	runCommand = func(name string, args ...string) ([]byte, error) {
		t.Fatalf("setup pi must not run commands, got %s %v", name, args)
		return nil, nil
	}
	BinaryVersion = "0.1.0"
	t.Cleanup(func() { BinaryVersion = "dev" })
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	e := piEnv{agentDir: dir, settings: filepath.Join(dir, "settings.json")}
	if settings != "" {
		if err := os.WriteFile(e.settings, []byte(settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := BackupRoot()
	if err != nil {
		t.Fatal(err)
	}
	e.dataDir = filepath.Dir(root)
	return e
}

func (e piEnv) pluginDir(version string) string {
	return filepath.Join(e.dataDir, "pi-plugin", version)
}

func (e piEnv) packages(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(e.settings)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Packages []json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("settings is not JSON: %v\n%s", err, raw)
	}
	var out []string
	for _, p := range cfg.Packages {
		out = append(out, string(p))
	}
	return out
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func piSource(name string) string {
	return filepath.Join("..", "..", "plugin", "pi", filepath.FromSlash(name))
}

func TestEmbeddedPiPluginMatchesSourceByteForByte(t *testing.T) {
	embedded := filepath.Join("plugins", "pi")
	if got := listFiles(t, embedded); strings.Join(got, ",") != strings.Join(piPluginFiles, ",") {
		t.Fatalf("embedded Pi plugin files = %v, want %v (go generate ./internal/setup)", got, piPluginFiles)
	}
	for _, name := range piPluginFiles {
		want, err := os.ReadFile(piSource(name))
		if err != nil {
			t.Fatalf("read source %s: %v", name, err)
		}
		got, err := os.ReadFile(filepath.Join(embedded, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("embedded %s drifted from plugin/pi/%s; run go generate ./internal/setup", name, name)
		}
	}
}

func TestSetupPiWritesEmbeddedPackageAndRegistersLocalPath(t *testing.T) {
	e := newPiEnv(t, `{"theme":"dark","packages":["npm:other"]}`)
	res, err := InstallReplacingEngram("pi")
	if err != nil {
		t.Fatalf("setup pi: %v", err)
	}
	dir := e.pluginDir("0.1.0")
	if got := listFiles(t, dir); strings.Join(got, ",") != strings.Join(piPluginFiles, ",") {
		t.Fatalf("installed files = %v, want %v", got, piPluginFiles)
	}
	for _, name := range piPluginFiles {
		want, _ := os.ReadFile(piSource(name))
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(want, got) {
			t.Errorf("%s is not byte-identical to plugin/pi/%s (%v)", name, name, err)
		}
	}
	pkgs := e.packages(t)
	wantPath, _ := json.Marshal(dir)
	if len(pkgs) != 2 || pkgs[0] != `"npm:other"` || pkgs[1] != string(wantPath) {
		t.Fatalf("packages = %v, want npm:other kept and %s added", pkgs, wantPath)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("plugin dir %q must be absolute", dir)
	}
	raw, _ := os.ReadFile(e.settings)
	if !strings.Contains(string(raw), `"theme": "dark"`) {
		t.Errorf("other settings lost: %s", raw)
	}
	if res.Destination != dir || res.BackupDir == "" {
		t.Errorf("result = %+v, want destination %s and a backup", res, dir)
	}
	if _, err := os.Stat(filepath.Join(e.agentDir, "mcp.json")); !os.IsNotExist(err) {
		t.Errorf("setup pi must not create mcp.json (native tools): %v", err)
	}
}

func TestSetupPiCreatesSettingsWhenMissing(t *testing.T) {
	e := newPiEnv(t, "")
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	pkgs := e.packages(t)
	if len(pkgs) != 1 || !strings.Contains(pkgs[0], "pi-plugin") {
		t.Fatalf("packages = %v", pkgs)
	}
}

func TestSetupPiTwiceIsIdempotent(t *testing.T) {
	e := newPiEnv(t, `{"packages":["npm:other"]}`)
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(e.settings)
	root, _ := BackupRoot()
	backupsBefore := listFiles(t, root)
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatalf("second setup pi: %v", err)
	}
	second, _ := os.ReadFile(e.settings)
	if !bytes.Equal(first, second) {
		t.Fatalf("settings changed on the second run:\n%s\n---\n%s", first, second)
	}
	if got := listFiles(t, root); strings.Join(got, ",") != strings.Join(backupsBefore, ",") {
		t.Fatalf("second run made another backup: %v vs %v", got, backupsBefore)
	}
	if len(e.packages(t)) != 2 {
		t.Fatalf("packages duplicated: %v", e.packages(t))
	}
	// A later run also repairs a damaged plugin directory.
	if err := os.WriteFile(filepath.Join(e.pluginDir("0.1.0"), "index.ts"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(piSource("index.ts"))
	got, _ := os.ReadFile(filepath.Join(e.pluginDir("0.1.0"), "index.ts"))
	if !bytes.Equal(want, got) {
		t.Fatal("a tampered plugin file must be rewritten")
	}
}

func TestSetupPiRestoreBringsBackOriginalSettings(t *testing.T) {
	original := "{ \"packages\" : [ \"npm:other\" ],\n  \"theme\":\"dark\" }\n"
	e := newPiEnv(t, original)
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallReplacingEngram("pi"); err != nil { // a repeated run keeps the first backup
		t.Fatal(err)
	}
	res, err := RestoreAgentBackup("pi", false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(res.Refused) != 0 || len(res.Restored) != 1 {
		t.Fatalf("restore result = %+v", res)
	}
	got, _ := os.ReadFile(e.settings)
	if string(got) != original {
		t.Fatalf("settings after restore = %q, want %q", got, original)
	}
}

func TestSetupPiRestoreRefusesSettingsEditedSinceSetup(t *testing.T) {
	e := newPiEnv(t, `{"packages":[]}`)
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.settings, []byte(`{"packages":["mine"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := RestoreAgentBackup("pi", false)
	if err != nil || len(res.Refused) != 1 {
		t.Fatalf("edited settings must be refused: %+v %v", res, err)
	}
}

func TestSetupPiNewVersionReplacesOldDirectoryAndEntry(t *testing.T) {
	original := `{"packages":["npm:other"]}`
	e := newPiEnv(t, original)
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	BinaryVersion = "0.2.0"
	if _, err := InstallReplacingEngram("pi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(e.pluginDir("0.1.0")); !os.IsNotExist(err) {
		t.Errorf("old version directory must be replaced: %v", err)
	}
	if got := listFiles(t, e.pluginDir("0.2.0")); len(got) != len(piPluginFiles) {
		t.Errorf("new version files = %v", got)
	}
	pkgs := e.packages(t)
	newPath, _ := json.Marshal(e.pluginDir("0.2.0"))
	if len(pkgs) != 2 || pkgs[1] != string(newPath) {
		t.Fatalf("packages = %v, want only the 0.2.0 entry besides npm:other", pkgs)
	}
	// Restore still returns the settings from before the first setup.
	if _, err := RestoreAgentBackup("pi", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(e.settings); string(got) != original {
		t.Fatalf("restore after upgrade = %q", got)
	}
}

func TestSetupPiNeverWritesNpmSourceAndKeepsExistingGentleEngram(t *testing.T) {
	for name, original := range map[string]string{
		"string entry":  `{"packages":["npm:gentle-engram@0.8.0"]}`,
		"object entry":  `{"packages":[{"source":"npm:gentle-engram","autoUpdate":false}]}`,
		"empty":         `{}`,
		"other package": `{"packages":["npm:something","git:github.com/x/y"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPiEnv(t, original)
			res, err := InstallReplacingEngram("pi")
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(e.settings)
			var before, after struct {
				Packages []json.RawMessage `json:"packages"`
			}
			_ = json.Unmarshal([]byte(original), &before)
			_ = json.Unmarshal(raw, &after)
			same := func(a, b json.RawMessage) bool {
				var x, y any
				_ = json.Unmarshal(a, &x)
				_ = json.Unmarshal(b, &y)
				xb, _ := json.Marshal(x)
				yb, _ := json.Marshal(y)
				return bytes.Equal(xb, yb)
			}
			isOld := func(a json.RawMessage) bool {
				for _, b := range before.Packages {
					if same(a, b) {
						return true
					}
				}
				return false
			}
			for _, b := range before.Packages {
				found := false
				for _, a := range after.Packages {
					found = found || same(a, b)
				}
				if !found {
					t.Errorf("existing entry %s was removed: %s", b, raw)
				}
			}
			for _, a := range after.Packages {
				if !isOld(a) && strings.Contains(string(a), "npm:") {
					t.Errorf("setup wrote an npm source: %s", a)
				}
			}
			if strings.Contains(original, "gentle-engram") {
				if warned := strings.Join(res.Warnings, "\n"); !strings.Contains(warned, "gentle-engram") {
					t.Errorf("setup must report the existing gentle-engram package, warnings=%q", warned)
				}
			}
		})
	}
}

func TestSetupPiRejectsUnreadableSettingsWithoutChangingAnything(t *testing.T) {
	e := newPiEnv(t, `{"packages": [broken`)
	if _, err := InstallReplacingEngram("pi"); err == nil {
		t.Fatal("malformed settings must fail")
	}
	if got, _ := os.ReadFile(e.settings); string(got) != `{"packages": [broken` {
		t.Fatalf("settings changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "pi-plugin")); !os.IsNotExist(err) {
		t.Errorf("no plugin may be written when settings cannot be read: %v", err)
	}
}

func TestSetupPiLeavesGentleAIModeAlone(t *testing.T) {
	// gentle-ai registers the memory server as "engram"; when that entry already
	// runs qbit-jay, setup changes nothing (existing coexistence rule).
	e := newPiEnv(t, `{"packages":["npm:gentle-engram"]}`)
	mcp := filepath.Join(e.agentDir, "mcp.json")
	if err := os.WriteFile(mcp, []byte(`{"mcpServers":{"engram":{"command":"/usr/local/bin/qbit-jay","args":["mcp","--tools=agent"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := InstallReplacingEngram("pi")
	if err != nil || !res.GentleAIMode {
		t.Fatalf("expected gentle-ai mode no-op, got %+v %v", res, err)
	}
	if got, _ := os.ReadFile(e.settings); string(got) != `{"packages":["npm:gentle-engram"]}` {
		t.Fatalf("settings changed: %s", got)
	}
}
