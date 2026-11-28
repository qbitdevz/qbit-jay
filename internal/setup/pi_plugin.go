package setup

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qbitdevz/qbit-jay/internal/qbit/env"
	"github.com/qbitdevz/qbit-jay/internal/qbit/paths"
)

// The Pi plugin ships inside the binary (S12, changed by L4): no npm. `setup pi`
// writes it to <data dir>/pi-plugin/<version>/ and lists that absolute path in
// Pi's settings `packages`, which Pi loads as a local package. The embedded
// copy under plugins/pi is generated from plugin/pi (go generate ./internal/setup)
// and a test keeps it byte-identical.
//
//go:embed plugins/pi
var piPluginFS embed.FS

const (
	piPluginEmbedRoot = "plugins/pi"
	piPluginDirName   = "pi-plugin"
)

// dataDir is the qbit-jay data directory: QBIT_JAY_DATA_DIR or ~/.qbit-jay.
func dataDir() (string, error) {
	if dir := strings.TrimSpace(env.Get("DATA_DIR")); dir != "" {
		return dir, nil
	}
	home, err := userHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, paths.DataDirName), nil
}

// piVersionDir turns the binary version into one safe directory name.
func piVersionDir() string {
	v := strings.TrimPrefix(strings.TrimSpace(BinaryVersion), "v")
	if v == "" || strings.Trim(v, ".") == "" {
		return "dev"
	}
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '.', r == '-', r == '_', r == '+':
		default:
			return "dev"
		}
	}
	return v
}

// piEmbeddedFiles returns every embedded plugin file, relative and slash separated.
func piEmbeddedFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(piPluginFS, piPluginEmbedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := piPluginFS.ReadFile(p)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(p, piPluginEmbedRoot+"/")] = data
		return nil
	})
	if err == nil && len(files) == 0 {
		err = fmt.Errorf("the embedded Pi plugin is empty")
	}
	return files, err
}

// piPluginCurrent reports whether dir holds exactly the embedded files.
func piPluginCurrent(dir string, files map[string][]byte) bool {
	seen := 0
	ok := true
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		want, found := files[filepath.ToSlash(rel)]
		if !found {
			ok = false
			return nil
		}
		got, readErr := os.ReadFile(p)
		if readErr != nil || !bytes.Equal(got, want) {
			ok = false
			return nil
		}
		seen++
		return nil
	})
	return err == nil && ok && seen == len(files)
}

// writePiPlugin installs the embedded files into dest atomically: it stages a
// sibling directory, then swaps it in, so Pi never sees a half written package.
// Directories are 0700 and files 0600.
func writePiPlugin(root, dest string, files map[string][]byte) error {
	if piPluginCurrent(dest, files) {
		return nil
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}
	stage, err := os.MkdirTemp(root, ".stage-")
	if err != nil {
		return fmt.Errorf("stage the Pi plugin in %s: %w", root, err)
	}
	cleanStage := true
	defer func() {
		if cleanStage {
			_ = os.RemoveAll(stage)
		}
	}()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, files[name], 0o600); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	var old string
	if _, statErr := os.Lstat(dest); statErr == nil {
		old = stage + ".old"
		if err := os.Rename(dest, old); err != nil {
			return fmt.Errorf("replace %s: %w", dest, err)
		}
	}
	if err := os.Rename(stage, dest); err != nil {
		if old != "" {
			_ = os.Rename(old, dest)
		}
		return fmt.Errorf("install %s: %w", dest, err)
	}
	cleanStage = false
	if old != "" {
		_ = os.RemoveAll(old)
	}
	return nil
}

// prunePiPlugins removes every other version directory (and staging leftovers)
// under root, so a newer binary replaces the old package.
func prunePiPlugins(root, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() != keep {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

func piSamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtimeGOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// piEntrySource returns the package source of a settings `packages` entry,
// either a plain string or an object with a "source" field.
func piEntrySource(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Source string `json:"source"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Source
	}
	return ""
}

// isOurPiPlugin reports whether source is a qbit-jay plugin directory of some
// version under root.
func isOurPiPlugin(source, root string) bool {
	return source != "" && filepath.IsAbs(source) && piSamePath(filepath.Dir(source), root)
}

func piPackages(config map[string]json.RawMessage, settingsPath string) ([]json.RawMessage, error) {
	raw, ok := config["packages"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var packages []json.RawMessage
	if err := json.Unmarshal(raw, &packages); err != nil {
		return nil, fmt.Errorf("%s: packages must be an array: %w", settingsPath, err)
	}
	return packages, nil
}

// piHasOurEntry reports whether settings already register a qbit-jay plugin
// directory, which means a pre-install backup already exists.
func piHasOurEntry(settingsPath, root string) (bool, error) {
	config, err := readJSONConfig(settingsPath)
	if err != nil {
		return false, fmt.Errorf("read Pi settings %s: %w", settingsPath, err)
	}
	packages, err := piPackages(config, settingsPath)
	if err != nil {
		return false, err
	}
	for _, p := range packages {
		if isOurPiPlugin(piEntrySource(p), root) {
			return true, nil
		}
	}
	return false, nil
}

// installPi writes the embedded Pi plugin and registers it as a local package
// in Pi's settings. It never writes an npm source and never touches mcp.json:
// the plugin has native mem_* tools, so a second MCP entry would duplicate them.
func installPi() (*Result, error) {
	agentDir := piAgentDir()
	settingsPath := filepath.Join(agentDir, "settings.json")
	data, err := dataDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(data, piPluginDirName)
	dest := filepath.Join(root, piVersionDir())

	// Read settings first: a file setup cannot parse stops it before any write.
	config, err := readJSONConfig(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("read Pi settings %s: %w", settingsPath, err)
	}
	packages, err := piPackages(config, settingsPath)
	if err != nil {
		return nil, err
	}
	files, err := piEmbeddedFiles()
	if err != nil {
		return nil, err
	}
	if err := writePiPlugin(root, dest, files); err != nil {
		return nil, err
	}

	destJSON, err := json.Marshal(dest)
	if err != nil {
		return nil, err
	}
	var updated []json.RawMessage
	hasDest := false
	var warnings []string
	for _, p := range packages {
		source := piEntrySource(p)
		if isOurPiPlugin(source, root) {
			if piSamePath(source, dest) && !hasDest {
				hasDest = true
				updated = append(updated, p)
			}
			continue // a duplicate, or an older version replaced below
		}
		if strings.Contains(strings.ToLower(source), "gentle-engram") {
			warnings = append(warnings, fmt.Sprintf("Pi settings still list %q (left as it was). Pi tool names are exclusive and both packages register mem_* tools: if Pi reports a tool conflict, remove one of them from packages in %s (or run qbit-jay setup --restore pi).", source, settingsPath))
		}
		updated = append(updated, p)
	}
	if !hasDest {
		updated = append(updated, destJSON)
	}
	changed := len(updated) != len(packages)
	for i := 0; !changed && i < len(updated); i++ {
		changed = !bytes.Equal(updated[i], packages[i])
	}
	if changed {
		raw, err := json.Marshal(updated)
		if err != nil {
			return nil, err
		}
		config["packages"] = raw
		if err := writeJSONConfig(settingsPath, config); err != nil {
			return nil, fmt.Errorf("write Pi settings %s: %w", settingsPath, err)
		}
	}
	prunePiPlugins(root, filepath.Base(dest))

	warnings = append(warnings, piMCPWarnings(filepath.Join(agentDir, "mcp.json"))...)
	return &Result{Agent: "pi", Destination: dest, Files: len(files), Warnings: warnings}, nil
}

// piMCPWarnings reports, without changing anything, MCP entries that would give
// Pi a second set of mem_* tools next to the plugin's native ones.
func piMCPWarnings(mcpPath string) []string {
	config, err := readJSONConfig(mcpPath)
	if err != nil {
		return nil
	}
	var servers map[string]json.RawMessage
	if raw, ok := config["mcpServers"]; !ok || json.Unmarshal(raw, &servers) != nil {
		return nil
	}
	var out []string
	for _, name := range []string{mcpServerName, legacyMCPServerName} {
		if _, ok := servers[name]; ok {
			out = append(out, fmt.Sprintf("%s has mcpServers.%s (left as it was). The plugin has native mem_* tools; for native-only writes remove that key and reload Pi.", mcpPath, name))
		}
	}
	return out
}

// installPiReplacing is `setup pi` with a backup of settings.json: the first run
// saves the original so `setup --restore pi` can put it back; later runs and
// upgrades keep that backup and only refresh the hash of the file setup wrote.
func installPiReplacing() (*Result, error) {
	settingsPath := filepath.Join(piAgentDir(), "settings.json")
	data, err := dataDir()
	if err != nil {
		return nil, err
	}
	registered, err := piHasOurEntry(settingsPath, filepath.Join(data, piPluginDirName))
	if err != nil {
		return nil, err
	}
	if registered {
		refresh := trackUnchangedInstalled("pi")
		result, err := Install("pi")
		if err == nil {
			refresh()
		}
		return result, err
	}
	backupDir, reused, err := backupTargets("pi", []replaceTarget{{path: settingsPath}}, []string{settingsPath})
	if err != nil {
		return nil, fmt.Errorf("back up Pi settings before registering the plugin: %w", err)
	}
	result, err := Install("pi")
	if err != nil {
		return nil, fmt.Errorf("%w (Pi settings were not changed; backup at %s)", err, backupDir)
	}
	if err := recordInstalled(backupDir); err != nil {
		if _, rerr := restoreAgentBackup("pi", true); rerr != nil {
			return nil, fmt.Errorf("record the installed file hashes: %w; automatic restore failed: %v (backup at %s)", err, rerr, backupDir)
		}
		return nil, fmt.Errorf("record the installed file hashes: %w; restored the Pi settings from %s", err, backupDir)
	}
	result.BackupDir = backupDir
	result.BackupReused = reused
	return result, nil
}
