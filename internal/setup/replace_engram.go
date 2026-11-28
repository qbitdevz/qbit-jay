package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// This file implements the coexistence rule of spec 6.3: when an agent already
// has an Engram registration, `qbit-jay setup <agent>` backs up every file it
// is about to touch, installs the qbit-jay entry and removes the Engram entry so
// the agent does not see two sets of mem_* tools. `setup --restore <agent>`
// puts the backed up files back. Neither step ever touches the Engram database
// or binary.

const backupManifestName = "manifest.json"

// backupNow is a seam so tests get distinct, ordered backup directory names.
var backupNow = time.Now

type stripFunc func(data []byte) (out []byte, removed []string, err error)

// replaceTarget is one file setup may touch for an agent. strip nil means the
// file is only backed up. deleteWhat non-empty means the whole file is an
// Engram artefact (the OpenCode engram.ts plugin) that is removed.
type replaceTarget struct {
	path       string
	strip      stripFunc
	deleteWhat string
	// apply, when set, removes the entry instead of rewriting the file. It is
	// used for files another program owns (Claude Code's ~/.claude.json is
	// edited through `claude mcp remove`).
	apply func() error
	// commands lists the command of every "engram" server entry in the file,
	// used to tell an entry that runs qbit-jay from one that runs Engram.
	commands func(data []byte) []string
}

// BackupEntry is one saved file in a backup manifest.
type BackupEntry struct {
	Path   string `json:"path"`   // original absolute path
	File   string `json:"file"`   // file name inside the backup directory
	SHA256 string `json:"sha256"` // of the saved bytes
	Mode   uint32 `json:"mode"`
	// InstalledSHA256 is the sha256 of the file as setup left it (installedAbsent
	// when setup left no file). Restore refuses a file whose current bytes differ,
	// because it was edited after setup. Empty in manifests that predate it.
	InstalledSHA256 string `json:"installed_sha256,omitempty"`
}

// installedAbsent marks, in InstalledSHA256, a path that does not exist after setup.
const installedAbsent = "absent"

// BackupManifest describes one backup directory.
type BackupManifest struct {
	Agent     string        `json:"agent"`
	CreatedAt string        `json:"created_at"`
	Files     []BackupEntry `json:"files"`
}

// BackupRoot returns the directory that holds setup backups, ~/.qbit-jay/backups
// (or QBIT_JAY_DATA_DIR/backups).
func BackupRoot() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "backups"), nil
}

// InstallReplacingEngram installs the agent like Install and, when the agent
// has an Engram registration, backs up the files it touches first and removes
// the Engram entries afterwards. Result.Replaced and Result.BackupDir describe
// what happened.
func InstallReplacingEngram(agentName string) (*Result, error) {
	if !isSupportedSlug(agentName) {
		return Install(agentName)
	}
	targets := replaceTargets(agentName)

	// gentle-ai compatibility mode: an "engram" entry that already runs
	// qbit-jay is the intended registration. Touch nothing, add nothing.
	for _, t := range targets {
		if t.commands == nil {
			continue
		}
		data, err := os.ReadFile(t.path)
		if err != nil {
			continue
		}
		for _, command := range t.commands(data) {
			if runsQbitJay(command) {
				return &Result{
					Agent:        agentName,
					GentleAIMode: true,
					Notice:       fmt.Sprintf("The %q MCP entry in %s already runs qbit-jay (gentle-ai mode); nothing was changed.", engramServerKey, t.path),
				}, nil
			}
		}
	}

	if agentName == "pi" {
		return installPiReplacing()
	}

	var found []int
	for i, t := range targets {
		msgs, err := inspectTarget(t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot inspect %s for an Engram entry: %v\n", t.path, err)
			continue
		}
		if len(msgs) > 0 {
			found = append(found, i)
		}
	}
	if len(found) == 0 {
		// A repeated setup rewrites its own files; keep the latest backup's
		// recorded hashes in step for the files nobody edited since.
		refresh := trackUnchangedInstalled(agentName)
		result, err := Install(agentName)
		if err == nil {
			refresh()
		}
		return result, err
	}

	var foundPaths []string
	for _, i := range found {
		foundPaths = append(foundPaths, targets[i].path)
	}
	backupDir, reused, err := backupTargets(agentName, targets, foundPaths)
	if err != nil {
		return nil, fmt.Errorf("back up %s configuration before replacing Engram: %w", agentName, err)
	}

	result, err := Install(agentName)
	if err != nil {
		return nil, fmt.Errorf("%w (the Engram entries were not removed; backup at %s)", err, backupDir)
	}
	// Past this point the files are being rewritten: any failure puts the
	// backup back so the agent is not left half converted.
	rollback := func(cause error) error {
		rres, rerr := restoreAgentBackup(agentName, true)
		if rerr != nil {
			return fmt.Errorf("%w; automatic restore failed: %v (backup at %s; run: qbit-jay setup --restore %s)", cause, rerr, backupDir, agentName)
		}
		return fmt.Errorf("%w; restored the backup for %s automatically (%d file(s) from %s)", cause, agentName, len(rres.Restored), rres.BackupDir)
	}

	var replaced []string
	for _, i := range found {
		msgs, err := applyTarget(targets[i])
		if err != nil {
			return nil, rollback(fmt.Errorf("remove the Engram entry from %s: %w", targets[i].path, err))
		}
		replaced = append(replaced, msgs...)
	}
	if err := recordInstalled(backupDir); err != nil {
		return nil, rollback(fmt.Errorf("record the installed file hashes: %w", err))
	}
	result.Replaced = replaced
	result.BackupDir = backupDir
	result.BackupReused = reused
	return result, nil
}

func isSupportedSlug(slug string) bool {
	for _, s := range supportedSlugs() {
		if s == slug {
			return true
		}
	}
	return false
}

func inspectTarget(t replaceTarget) ([]string, error) {
	if t.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(t.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t.deleteWhat != "" {
		return []string{fmt.Sprintf("%s: %s", t.path, t.deleteWhat)}, nil
	}
	if t.strip == nil {
		return nil, nil
	}
	_, removed, err := t.strip(data)
	return prefixPath(t.path, removed), err
}

func applyTarget(t replaceTarget) ([]string, error) {
	if t.deleteWhat != "" {
		if err := os.Remove(t.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return []string{fmt.Sprintf("%s: %s", t.path, t.deleteWhat)}, nil
	}
	data, err := os.ReadFile(t.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out, removed, err := t.strip(data)
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, nil
	}
	if t.apply != nil {
		if err := t.apply(); err != nil {
			return nil, err
		}
		return prefixPath(t.path, removed), nil
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(t.path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if string(out) != string(data) {
		if err := os.WriteFile(t.path, out, mode); err != nil {
			return nil, err
		}
	}
	return prefixPath(t.path, removed), nil
}

func prefixPath(path string, msgs []string) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = path + ": " + m
	}
	return out
}

// ─── backups ─────────────────────────────────────────────────────────────────

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// backupTargets saves every existing target file. When the newest backup of the
// agent already holds byte-identical copies of the same files, it is reused.
func backupTargets(slug string, targets []replaceTarget, engramFiles []string) (dir string, reused bool, err error) {
	root, err := BackupRoot()
	if err != nil {
		return "", false, err
	}
	agentDir := filepath.Join(root, slug)

	type snap struct {
		path string
		data []byte
		mode os.FileMode
	}
	var snaps []snap
	seen := map[string]bool{}
	for _, t := range targets {
		if t.path == "" || seen[t.path] {
			continue
		}
		seen[t.path] = true
		data, readErr := os.ReadFile(t.path)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return "", false, readErr
		}
		mode := os.FileMode(0o644)
		if info, statErr := os.Stat(t.path); statErr == nil {
			mode = info.Mode().Perm()
		}
		snaps = append(snaps, snap{t.path, data, mode})
	}

	// Reuse the newest backup when it already holds byte-identical copies of
	// every file that carries an Engram entry. Files that setup itself created
	// (and that the earlier backup therefore does not hold) do not count.
	if latest, manifest, ok := latestBackup(agentDir); ok {
		byPath := map[string]string{}
		for _, f := range manifest.Files {
			byPath[f.Path] = f.SHA256
		}
		same := true
		for _, s := range snaps {
			for _, p := range engramFiles {
				if s.path == p && byPath[p] != sha256Hex(s.data) {
					same = false
				}
			}
		}
		if same {
			return latest, true, nil
		}
	}

	name := backupNow().UTC().Format("20060102T150405.000000000Z")
	dir = filepath.Join(agentDir, name)
	for n := 1; ; n++ {
		if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
			break
		}
		dir = filepath.Join(agentDir, fmt.Sprintf("%s-%d", name, n))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	manifest := BackupManifest{Agent: slug, CreatedAt: backupNow().UTC().Format(time.RFC3339)}
	for i, s := range snaps {
		file := fmt.Sprintf("%02d-%s", i, filepath.Base(s.path))
		if err := os.WriteFile(filepath.Join(dir, file), s.data, 0o600); err != nil {
			return "", false, err
		}
		manifest.Files = append(manifest.Files, BackupEntry{Path: s.path, File: file, SHA256: sha256Hex(s.data), Mode: uint32(s.mode)})
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", false, err
	}
	// The manifest is written last: a directory without one is an unfinished
	// backup and is ignored by restore.
	if err := os.WriteFile(filepath.Join(dir, backupManifestName), append(raw, '\n'), 0o600); err != nil {
		return "", false, err
	}
	return dir, false, nil
}

func latestBackup(agentDir string) (string, *BackupManifest, bool) {
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return "", nil, false
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		dir := filepath.Join(agentDir, name)
		raw, err := os.ReadFile(filepath.Join(dir, backupManifestName))
		if err != nil {
			continue
		}
		var m BackupManifest
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		return dir, &m, true
	}
	return "", nil, false
}

// installedHash is the sha256 of the file at path, or installedAbsent.
func installedHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return installedAbsent, nil
	}
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

// recordInstalled stores, in the backup manifest, the hash of every backed up
// path as setup left it, so a later restore can tell edits made since.
func recordInstalled(dir string) error {
	return recordInstalledFor(dir, func(string) bool { return true })
}

func recordInstalledFor(dir string, include func(path string) bool) error {
	manifestPath := filepath.Join(dir, backupManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m BackupManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for i := range m.Files {
		if !include(m.Files[i].Path) {
			continue
		}
		h, err := installedHash(m.Files[i].Path)
		if err != nil {
			return err
		}
		m.Files[i].InstalledSHA256 = h
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, append(out, '\n'), 0o600)
}

// trackUnchangedInstalled notes which files of the agent's latest backup still
// match the hash setup recorded; the returned func records their new hashes.
func trackUnchangedInstalled(slug string) func() {
	root, err := BackupRoot()
	if err != nil {
		return func() {}
	}
	dir, manifest, ok := latestBackup(filepath.Join(root, slug))
	if !ok {
		return func() {}
	}
	unchanged := map[string]bool{}
	for _, f := range manifest.Files {
		if f.InstalledSHA256 == "" {
			continue
		}
		if h, err := installedHash(f.Path); err == nil && h == f.InstalledSHA256 {
			unchanged[f.Path] = true
		}
	}
	return func() {
		_ = recordInstalledFor(dir, func(path string) bool { return unchanged[path] })
	}
}

// RestoreResult reports what RestoreAgentBackup put back.
type RestoreResult struct {
	Agent     string
	BackupDir string
	Restored  []string
	// Refused lists files left as they are because they changed after setup
	// wrote them; force restores them anyway.
	Refused []string
}

// RestoreAgentBackup restores the files saved by the newest backup of the
// agent. Every saved file is verified against the manifest sha256 before any
// file is written. A file edited since setup wrote it (its current sha256
// differs from the recorded one) is not restored unless force is set; it is
// listed in RestoreResult.Refused.
func RestoreAgentBackup(slug string, force bool) (*RestoreResult, error) {
	return restoreAgentBackup(slug, force)
}

func restoreAgentBackup(slug string, force bool) (*RestoreResult, error) {
	if !isSupportedSlug(slug) {
		return nil, fmt.Errorf("unknown agent: %q (supported: %s)", slug, strings.Join(supportedSlugs(), ", "))
	}
	root, err := BackupRoot()
	if err != nil {
		return nil, err
	}
	dir, manifest, ok := latestBackup(filepath.Join(root, slug))
	if !ok {
		return nil, fmt.Errorf("no backup found for %s under %s", slug, filepath.Join(root, slug))
	}
	if manifest.Agent != slug {
		return nil, fmt.Errorf("backup %s belongs to agent %q, not %q", dir, manifest.Agent, slug)
	}
	type item struct {
		entry BackupEntry
		data  []byte
	}
	var items []item
	for _, f := range manifest.Files {
		if f.File != filepath.Base(f.File) || f.File == "" || f.File == "." || f.File == ".." || !filepath.IsAbs(f.Path) {
			return nil, fmt.Errorf("backup manifest %s has an invalid entry for %q", dir, f.Path)
		}
		data, err := os.ReadFile(filepath.Join(dir, f.File))
		if err != nil {
			return nil, fmt.Errorf("read backup file %s: %w", f.File, err)
		}
		if got := sha256Hex(data); got != f.SHA256 {
			return nil, fmt.Errorf("backup file %s is corrupt (sha256 %s, manifest says %s); nothing was restored", f.File, got, f.SHA256)
		}
		items = append(items, item{f, data})
	}
	res := &RestoreResult{Agent: slug, BackupDir: dir}
	for _, it := range items {
		if !force && it.entry.InstalledSHA256 != "" {
			current, err := installedHash(it.entry.Path)
			if err != nil {
				return nil, err
			}
			if current != it.entry.InstalledSHA256 {
				res.Refused = append(res.Refused, it.entry.Path)
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(it.entry.Path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(it.entry.Path, it.data, os.FileMode(it.entry.Mode)); err != nil {
			return nil, fmt.Errorf("restore %s: %w", it.entry.Path, err)
		}
		res.Restored = append(res.Restored, it.entry.Path)
	}
	return res, nil
}

// ─── targets per agent ───────────────────────────────────────────────────────

const engramServerKey = legacyMCPServerName

func replaceTargets(slug string) []replaceTarget {
	jsonServer := func(path string, f mcpFormat) replaceTarget {
		jsonc := f == opencodeObject
		return replaceTarget{path: path, strip: stripJSONServers(jsonc, mcpTopKey(f)), commands: jsonServerCommands(jsonc, mcpTopKey(f))}
	}
	switch slug {
	case "opencode":
		return []replaceTarget{
			{path: openCodeConfigPath(), strip: stripJSONServers(true, "mcp"), commands: jsonServerCommands(true, "mcp")},
			{path: openCodeTUIConfigPath()},
			{path: filepath.Join(openCodePluginDir(), "engram.ts"), deleteWhat: "removed the Engram OpenCode plugin file"},
			{path: filepath.Join(openCodePluginDir(), "qbit-jay.ts")},
		}
	case "pi":
		// Only used to spot a gentle-ai "engram" entry that already runs
		// qbit-jay; settings.json is backed up by installPiReplacing.
		return []replaceTarget{{path: filepath.Join(piAgentDir(), "mcp.json"), commands: jsonServerCommands(false, "mcpServers")}}
	case "claude-code":
		return []replaceTarget{
			{path: ClaudeCodeUserMCPPath(), strip: stripJSONServers(false, "mcpServers"), apply: removeClaudeUserEngramServer, commands: jsonServerCommands(false, "mcpServers")},
			{path: ClaudeCodeSettingsPath(), strip: stripClaudeEngramSettings},
		}
	case "gemini-cli":
		return []replaceTarget{
			jsonServer(geminiConfigPath(), mcpServersObject),
			{path: geminiSystemPromptPath()},
			{path: geminiEnvPath()},
		}
	case "codex":
		cfg := codexConfigPath()
		if cfg == "" {
			return nil
		}
		return []replaceTarget{
			{path: cfg, strip: stripCodexEngram, commands: codexEngramCommands},
			{path: codexInstructionsPath()},
			{path: codexCompactPromptPath()},
		}
	}
	for _, a := range agentAdapters() {
		if a.slug != slug || a.custom != nil || a.mcpPath == nil {
			continue
		}
		targets := []replaceTarget{jsonServer(a.mcpPath(), a.mcpFormat)}
		for _, ins := range a.instructions {
			targets = append(targets, replaceTarget{path: ins.path()})
		}
		return targets
	}
	return nil
}

// ─── strip functions ─────────────────────────────────────────────────────────

func parseObject(data []byte, jsonc bool) (map[string]json.RawMessage, error) {
	if jsonc {
		data = stripJSONC(data)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return cfg, nil
}

func renderObject(cfg map[string]json.RawMessage) ([]byte, error) {
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// deleteKey removes key from the object at path inside cfg.
func deleteKey(cfg map[string]json.RawMessage, path []string, key string) bool {
	if len(path) == 0 {
		if _, ok := cfg[key]; ok {
			delete(cfg, key)
			return true
		}
		return false
	}
	raw, ok := cfg[path[0]]
	if !ok {
		return false
	}
	var child map[string]json.RawMessage
	if json.Unmarshal(raw, &child) != nil || child == nil {
		return false
	}
	if !deleteKey(child, path[1:], key) {
		return false
	}
	b, err := json.Marshal(child)
	if err != nil {
		return false
	}
	cfg[path[0]] = b
	return true
}

// dropStrings removes the strings of the array at path.key that match drop.
func dropStrings(cfg map[string]json.RawMessage, path []string, key string, drop func(string) bool) []string {
	if len(path) > 0 {
		raw, ok := cfg[path[0]]
		if !ok {
			return nil
		}
		var child map[string]json.RawMessage
		if json.Unmarshal(raw, &child) != nil || child == nil {
			return nil
		}
		removed := dropStrings(child, path[1:], key, drop)
		if len(removed) > 0 {
			if b, err := json.Marshal(child); err == nil {
				cfg[path[0]] = b
			}
		}
		return removed
	}
	raw, ok := cfg[key]
	if !ok {
		return nil
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	var removed []string
	kept := make([]json.RawMessage, 0, len(values))
	for _, v := range values {
		var s string
		if json.Unmarshal(v, &s) == nil && drop(s) {
			removed = append(removed, s)
			continue
		}
		kept = append(kept, v)
	}
	if len(removed) > 0 {
		b, _ := json.Marshal(kept)
		cfg[key] = b
	}
	return removed
}

// stripJSONServers removes the "engram" MCP server under topKey (and under
// topKey.servers for OpenCode's V2 layout).
func stripJSONServers(jsonc bool, topKey string) stripFunc {
	return func(data []byte) ([]byte, []string, error) {
		cfg, err := parseObject(data, jsonc)
		if err != nil {
			return nil, nil, err
		}
		var removed []string
		if deleteKey(cfg, []string{topKey}, engramServerKey) {
			removed = append(removed, fmt.Sprintf("removed MCP server %q from %q", engramServerKey, topKey))
		}
		if jsonc && deleteKey(cfg, []string{topKey, "servers"}, engramServerKey) {
			removed = append(removed, fmt.Sprintf("removed MCP server %q from %q", engramServerKey, topKey+".servers"))
		}
		if len(removed) == 0 {
			return data, nil, nil
		}
		out, err := renderObject(cfg)
		return out, removed, err
	}
}

func stripClaudeEngramSettings(data []byte) ([]byte, []string, error) {
	cfg, err := parseObject(data, false)
	if err != nil {
		return nil, nil, err
	}
	var removed []string
	for _, rule := range dropStrings(cfg, []string{"permissions"}, "allow", func(s string) bool {
		return strings.HasPrefix(s, "mcp__engram__") || strings.HasPrefix(s, "mcp__plugin_engram_engram__")
	}) {
		removed = append(removed, fmt.Sprintf("removed permission %q", rule))
	}
	if deleteKey(cfg, []string{"enabledPlugins"}, "engram@engram") {
		removed = append(removed, `removed the enabled plugin "engram@engram"`)
	}
	if len(removed) == 0 {
		return data, nil, nil
	}
	out, err := renderObject(cfg)
	return out, removed, err
}

// stripCodexEngram removes the [mcp_servers.engram] table (and its subtables)
// and the engram@engram plugin table from a Codex config.toml.
func stripCodexEngram(data []byte) ([]byte, []string, error) {
	content := string(data)
	spans, err := scanCodexTOML(content)
	if err != nil {
		return nil, nil, err
	}
	var kept strings.Builder
	var removed []string
	removing := false
	for _, span := range spans {
		code := strings.TrimSpace(content[span.start:span.codeEnd])
		if strings.HasPrefix(code, "[") && !strings.HasPrefix(code, "[[") {
			keys, err := codexTOMLHeader(code)
			if err != nil {
				return nil, nil, err
			}
			removing = false
			switch {
			case len(keys) >= 2 && keys[0] == "mcp_servers" && keys[1] == engramServerKey:
				removing = true
				if len(keys) == 2 {
					removed = append(removed, `removed MCP server table [mcp_servers.engram]`)
				}
			case len(keys) >= 2 && keys[0] == "plugins" && keys[1] == "engram@engram":
				removing = true
				if len(keys) == 2 {
					removed = append(removed, `removed plugin table [plugins."engram@engram"]`)
				}
			}
		} else if strings.HasPrefix(code, "[[") {
			removing = false
		}
		if !removing {
			kept.WriteString(content[span.start:span.end])
		}
	}
	if len(removed) == 0 {
		return data, nil, nil
	}
	return []byte(kept.String()), removed, nil
}

// removeClaudeUserEngramServer lets the claude CLI edit its own user config.
func removeClaudeUserEngramServer() error {
	claudeBin, err := lookPathFn("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	if out, err := runCommand(claudeBin, "mcp", "remove", "--scope", "user", engramServerKey); err != nil {
		return fmt.Errorf("claude mcp remove %s: %v: %s", engramServerKey, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ─── gentle-ai mode ──────────────────────────────────────────────────────────

// runsQbitJay reports whether command names the qbit-jay binary: the file name
// is qbit-jay (or qbit-jay.exe, any case, either path separator), or it is the
// very executable running this setup.
func runsQbitJay(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	base := command
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.ToLower(base)
	if base == "qbit-jay" || base == "qbit-jay.exe" {
		return true
	}
	exe, err := osExecutable()
	if err != nil || exe == "" {
		return false
	}
	if filepath.Clean(command) == filepath.Clean(exe) {
		return true
	}
	a, errA := os.Stat(command)
	b, errB := os.Stat(exe)
	return errA == nil && errB == nil && os.SameFile(a, b)
}

// jsonServerCommands returns the commands of the "engram" entries under topKey
// (and topKey.servers for OpenCode's V2 layout).
func jsonServerCommands(jsonc bool, topKey string) func([]byte) []string {
	return func(data []byte) []string {
		cfg, err := parseObject(data, jsonc)
		if err != nil {
			return nil
		}
		var out []string
		collect := func(servers map[string]json.RawMessage) {
			if entry, ok := servers[engramServerKey]; ok {
				out = append(out, entryCommand(entry)...)
			}
		}
		var servers map[string]json.RawMessage
		if json.Unmarshal(cfg[topKey], &servers) == nil {
			collect(servers)
			if jsonc {
				var v2 map[string]json.RawMessage
				if json.Unmarshal(servers["servers"], &v2) == nil {
					collect(v2)
				}
			}
		}
		return out
	}
}

// entryCommand reads the executable of an MCP entry whose "command" is a string
// or an array whose first element is the executable.
func entryCommand(entry json.RawMessage) []string {
	var obj struct {
		Command json.RawMessage `json:"command"`
	}
	if json.Unmarshal(entry, &obj) != nil || len(obj.Command) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(obj.Command, &s) == nil {
		return []string{s}
	}
	var list []string
	if json.Unmarshal(obj.Command, &list) == nil && len(list) > 0 {
		return []string{list[0]}
	}
	return nil
}

// codexEngramCommands returns the command of [mcp_servers.engram].
func codexEngramCommands(data []byte) []string {
	content := string(data)
	spans, err := scanCodexTOML(content)
	if err != nil {
		return nil
	}
	var out []string
	inEngram := false
	for _, span := range spans {
		code := strings.TrimSpace(content[span.start:span.codeEnd])
		if strings.HasPrefix(code, "[") {
			keys, err := codexTOMLHeader(code)
			inEngram = err == nil && !strings.HasPrefix(code, "[[") && len(keys) == 2 && keys[0] == "mcp_servers" && keys[1] == engramServerKey
			continue
		}
		if !inEngram || span.equal < 0 {
			continue
		}
		keys, err := codexTOMLKey(content[span.start:span.equal])
		if err != nil || len(keys) != 1 || keys[0] != "command" {
			continue
		}
		value := strings.TrimSpace(content[span.equal+1 : span.codeEnd])
		if len(value) >= 2 && value[0] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				out = append(out, unquoted)
			}
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			out = append(out, value[1:len(value)-1])
		}
	}
	return out
}
