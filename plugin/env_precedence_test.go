package plugin_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Spec S5 (T5b): the hook scripts read QBIT_JAY_* first and ENGRAM_* as a
// fallback. The new name wins when both are set, and an empty new value falls
// back to a set legacy one. These tests observe what a script actually does
// (the URL it calls through curl, the flags passed, the files it writes, the
// output it prints) with stub curl and qbit-jay executables on PATH.

type precedenceCase struct {
	name string
	env  map[string]string
	want string
}

// precedenceMatrix covers new only, legacy only, both, new empty with legacy
// set, new set with legacy empty, both empty and neither. want is the value the
// script is expected to act on.
func precedenceMatrix(name, newValue, legacyValue, defaultValue string) []precedenceCase {
	n, l := "QBIT_JAY_"+name, "ENGRAM_"+name
	return []precedenceCase{
		{"new only", map[string]string{n: newValue}, newValue},
		{"legacy only", map[string]string{l: legacyValue}, legacyValue},
		{"both, new wins", map[string]string{n: newValue, l: legacyValue}, newValue},
		{"new empty, legacy set", map[string]string{n: "", l: legacyValue}, legacyValue},
		{"new set, legacy empty", map[string]string{n: newValue, l: ""}, newValue},
		{"both empty", map[string]string{n: "", l: ""}, defaultValue},
		{"neither", map[string]string{}, defaultValue},
	}
}

type stubFixture struct {
	bin  string
	home string
	tmp  string
	log  string
}

const precedenceStubQbitJay = `#!/usr/bin/env bash
printf 'qbit-jay %s\n' "$*" >> "$T5B_STUB_LOG"
case "$*" in
  "instance-id") printf '00000000000000000000000000000000\n' ;;
  "hook codex-resolve"*) printf '{"session_id":"sess1"}' ;;
esac
exit 0
`

// The curl stub resolves the project, reports one very old observation and
// fails every other request, which is all the prompt and subagent hooks need.
const precedenceStubCurl = `#!/usr/bin/env bash
printf 'curl %s\n' "$*" >> "$T5B_STUB_LOG"
case "$*" in
  */project/current*) printf '{"project":"p","project_source":"config"}' ;;
  */observations\?*) printf '[{"created_at":"2000-01-01T00:00:00Z"}]' ;;
  *) exit 22 ;;
esac
`

func newStubFixture(t *testing.T) stubFixture {
	t.Helper()
	root := t.TempDir()
	f := stubFixture{
		bin:  filepath.Join(root, "bin"),
		home: filepath.Join(root, "home"),
		tmp:  filepath.Join(root, "tmp"),
		log:  filepath.Join(root, "stub.log"),
	}
	for _, dir := range []string{f.bin, f.home, f.tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"qbit-jay": precedenceStubQbitJay, "curl": precedenceStubCurl} {
		if err := os.WriteFile(filepath.Join(f.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f stubFixture) readLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func (f stubFixture) resetLog(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.log); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

type runOptions struct {
	// keepSafeMode leaves the Claude Windows-bash safe mode to the test; by
	// default it is forced off so the full POSIX path is exercised everywhere.
	keepSafeMode bool
	extraEnv     []string
}

// runPrecedenceScript runs a hook under bash with every QBIT_JAY_* and
// ENGRAM_* variable removed from the inherited environment, so only the test's
// env map decides the outcome.
func runPrecedenceScript(t *testing.T, f stubFixture, script, stdin string, env map[string]string, opts runOptions) string {
	t.Helper()
	requireHookBinaries(t)
	path := bashScriptPath(t, script)
	cmd := exec.Command("bash", path)
	cmd.Dir = f.home
	cmd.Stdin = strings.NewReader(stdin)
	for _, entry := range os.Environ() {
		upper := strings.ToUpper(entry)
		switch {
		case strings.HasPrefix(upper, "ENGRAM_"), strings.HasPrefix(upper, "QBIT_JAY_"),
			strings.HasPrefix(upper, "PI_CODING_AGENT="), strings.HasPrefix(upper, "CLAUDE_CONFIG_DIR="),
			strings.HasPrefix(upper, "MSYSTEM="), strings.HasPrefix(upper, "MINGW_PREFIX="),
			strings.HasPrefix(upper, "PATH="), strings.HasPrefix(upper, "HOME="), strings.HasPrefix(upper, "TMPDIR="),
			strings.HasPrefix(upper, "T5B_STUB_LOG="):
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env,
		"PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+f.home,
		"TMPDIR="+filepath.ToSlash(f.tmp),
		"T5B_STUB_LOG="+f.log,
	)
	_, hasNew := env["QBIT_JAY_CLAUDE_WINDOWS_BASH_SAFE_MODE"]
	_, hasOld := env["ENGRAM_CLAUDE_WINDOWS_BASH_SAFE_MODE"]
	if !opts.keepSafeMode && !hasNew && !hasOld {
		cmd.Env = append(cmd.Env, "ENGRAM_CLAUDE_WINDOWS_BASH_SAFE_MODE=0")
	}
	cmd.Env = append(cmd.Env, opts.extraEnv...)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s must exit 0: %v\n%s", filepath.Base(script), err, out)
	}
	return string(out)
}

func precClaudeScript(name string) func(*testing.T) string {
	return func(t *testing.T) string { return filepath.Join(repoRoot(t), "plugin", "claude-code", "scripts", name) }
}

func precCodexScript(name string) func(*testing.T) string {
	return func(t *testing.T) string { return filepath.Join(repoRoot(t), "plugin", "codex", "scripts", name) }
}

// secondPrompt runs a prompt hook twice. The first run consumes the
// first-message bootstrap; the second takes the subsequent-message path, which
// calls the server. With nudgeAgo >= 0 the last-nudge marker is set that many
// seconds in the past before the second run.
func secondPrompt(t *testing.T, script string, f stubFixture, env map[string]string, opts runOptions, nudgeAgo int) string {
	t.Helper()
	stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q}`, filepath.ToSlash(f.home))
	runPrecedenceScript(t, f, script, stdin, env, opts)
	if nudgeAgo >= 0 {
		markers, _ := filepath.Glob(filepath.Join(f.tmp, "*-tools-loaded"))
		if len(markers) != 1 {
			t.Fatalf("session markers = %v, want exactly one", markers)
		}
		nudge := strings.TrimSuffix(markers[0], "-tools-loaded") + "-last-nudge"
		if err := os.WriteFile(nudge, []byte(fmt.Sprintf("%d\n", time.Now().Unix()-int64(nudgeAgo))), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f.resetLog(t)
	return runPrecedenceScript(t, f, script, stdin, env, opts)
}

func assertLogUses(t *testing.T, log, want string, others ...string) {
	t.Helper()
	if !strings.Contains(log, want) {
		t.Fatalf("stub log lacks %q:\n%s", want, log)
	}
	for _, other := range others {
		if other != want && strings.Contains(log, other) {
			t.Fatalf("stub log must not contain %q:\n%s", other, log)
		}
	}
}

func portURL(port string) string { return "http://127.0.0.1:" + port + "/" }

func TestClaudeHookEnvironmentPrecedencePortAndURL(t *testing.T) {
	script := precClaudeScript("user-prompt-submit.sh")
	for _, tt := range precedenceMatrix("PORT", "18101", "18102", "7437") {
		t.Run("PORT/"+tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			assertLogUses(t, log, portURL(tt.want), portURL("18101"), portURL("18102"), portURL("7437"))
		})
	}
	for _, tt := range precedenceMatrix("URL", "http://127.0.0.1:19001", "http://127.0.0.1:19002", "http://127.0.0.1:7437") {
		t.Run("URL/"+tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			assertLogUses(t, log, tt.want+"/", "http://127.0.0.1:19001/", "http://127.0.0.1:19002/", "http://127.0.0.1:7437/")
		})
	}
}

func TestClaudeHookEnvironmentPrecedenceSocket(t *testing.T) {
	script := precClaudeScript("user-prompt-submit.sh")
	for _, tt := range precedenceMatrix("SOCKET", "/tmp/new.sock", "/tmp/legacy.sock", "") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			if tt.want == "" {
				if strings.Contains(log, "--unix-socket") || !strings.Contains(log, portURL("7437")) {
					t.Fatalf("no socket configured: expected the default TCP endpoint, got:\n%s", log)
				}
				return
			}
			assertLogUses(t, log, "--unix-socket "+tt.want, "--unix-socket /tmp/new.sock", "--unix-socket /tmp/legacy.sock")
		})
	}
}

func TestClaudeHookEnvironmentPrecedenceHookMaxTime(t *testing.T) {
	script := precClaudeScript("user-prompt-submit.sh")
	for _, tt := range precedenceMatrix("HOOK_MAX_TIME", "1.5", "2.5", "0.2") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			var observations string
			for _, line := range strings.Split(log, "\n") {
				if strings.Contains(line, "/observations?") {
					observations = line
				}
			}
			if !strings.HasSuffix(observations, "--max-time "+tt.want) {
				t.Fatalf("observations call = %q, want --max-time %s", observations, tt.want)
			}
		})
	}
}

// With the nudge marker 100s old, a cooldown of 50 fires the reminder and a
// cooldown of 500 (or the 900 default) stays quiet. Both orientations make
// every source distinguishable from the default.
func TestClaudeHookEnvironmentPrecedenceNudgeCooldown(t *testing.T) {
	script := precClaudeScript("user-prompt-submit.sh")
	for _, values := range [][2]string{{"50", "500"}, {"500", "50"}} {
		for _, tt := range precedenceMatrix("NUDGE_COOLDOWN_SECS", values[0], values[1], "900") {
			t.Run(values[0]+"-"+values[1]+"/"+tt.name, func(t *testing.T) {
				f := newStubFixture(t)
				out := secondPrompt(t, script(t), f, tt.env, runOptions{}, 100)
				wantFire := tt.want == "50"
				if got := strings.Contains(out, "MEMORY REMINDER"); got != wantFire {
					t.Fatalf("nudge fired = %t, want %t (cooldown %s); output: %s", got, wantFire, tt.want, out)
				}
			})
		}
	}
}

// Safe mode only applies under a Windows bash (MSYSTEM). "0" selects the full
// path, whose nudge is additionalContext; any other value, or none, keeps the
// builtin-only path, which never emits additionalContext.
func TestClaudeHookEnvironmentPrecedenceWindowsSafeMode(t *testing.T) {
	script := precClaudeScript("user-prompt-submit.sh")
	for _, tt := range precedenceMatrix("CLAUDE_WINDOWS_BASH_SAFE_MODE", "0", "1", "auto") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			env := map[string]string{"QBIT_JAY_NUDGE_COOLDOWN_SECS": "50"}
			for k, v := range tt.env {
				env[k] = v
			}
			out := secondPrompt(t, script(t), f, env, runOptions{keepSafeMode: true, extraEnv: []string{"MSYSTEM=MINGW64"}}, 100)
			full := strings.Contains(out, "additionalContext")
			if wantFull := tt.want == "0"; full != wantFull {
				t.Fatalf("full path = %t, want %t (safe mode %s); output: %s", full, wantFull, tt.want, out)
			}
		})
	}
}

func TestClaudeHookEnvironmentPrecedenceDataDir(t *testing.T) {
	script := precClaudeScript("session-start.sh")
	for _, tt := range precedenceMatrix("DATA_DIR", "new-data", "legacy-data", ".qbit-jay") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			env := map[string]string{}
			for k, v := range tt.env {
				if v != "" {
					v = filepath.ToSlash(filepath.Join(f.home, v))
				}
				env[k] = v
			}
			stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q}`, filepath.ToSlash(f.home))
			runPrecedenceScript(t, f, script(t), stdin, env, runOptions{})
			for _, dir := range []string{"new-data", "legacy-data", ".qbit-jay"} {
				_, err := os.Stat(filepath.Join(f.home, dir, "serve.err.log"))
				if exists := err == nil; exists != (dir == tt.want) {
					t.Fatalf("serve.err.log in %s exists = %t, want %t (data dir %s)", dir, exists, dir == tt.want, tt.want)
				}
			}
		})
	}
}

func TestCodexHookEnvironmentPrecedencePortAndURL(t *testing.T) {
	script := precCodexScript("user-prompt-submit.sh")
	for _, tt := range precedenceMatrix("PORT", "18101", "18102", "7437") {
		t.Run("PORT/"+tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			assertLogUses(t, log, portURL(tt.want), portURL("18101"), portURL("18102"), portURL("7437"))
		})
	}
	for _, tt := range precedenceMatrix("URL", "http://127.0.0.1:19001", "http://127.0.0.1:19002", "http://127.0.0.1:7437") {
		t.Run("URL/"+tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			secondPrompt(t, script(t), f, tt.env, runOptions{}, -1)
			log := f.readLog(t)
			assertLogUses(t, log, tt.want+"/", "http://127.0.0.1:19001/", "http://127.0.0.1:19002/", "http://127.0.0.1:7437/")
		})
	}
}

func TestCodexHookEnvironmentPrecedenceNudgeCooldown(t *testing.T) {
	script := precCodexScript("user-prompt-submit.sh")
	for _, values := range [][2]string{{"50", "500"}, {"500", "50"}} {
		for _, tt := range precedenceMatrix("NUDGE_COOLDOWN_SECS", values[0], values[1], "900") {
			t.Run(values[0]+"-"+values[1]+"/"+tt.name, func(t *testing.T) {
				f := newStubFixture(t)
				out := secondPrompt(t, script(t), f, tt.env, runOptions{}, 100)
				wantFire := tt.want == "50"
				if got := strings.Contains(out, "MEMORY REMINDER"); got != wantFire {
					t.Fatalf("nudge fired = %t, want %t (cooldown %s); output: %s", got, wantFire, tt.want, out)
				}
			})
		}
	}
}

func TestCodexSubagentStopShellEnvironmentPrecedencePort(t *testing.T) {
	script := precCodexScript("subagent-stop.sh")
	for _, tt := range precedenceMatrix("PORT", "18101", "18102", "7437") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q,"last_assistant_message":"done"}`, filepath.ToSlash(f.home))
			runPrecedenceScript(t, f, script(t), stdin, tt.env, runOptions{})
			log := f.readLog(t)
			assertLogUses(t, log, portURL(tt.want)+"observations/passive")
			assertLogUses(t, log, portURL(tt.want), portURL("18101"), portURL("18102"), portURL("7437"))
		})
	}
}

// Codex session-start is the owner of the local server: an explicit URL or
// socket hands ownership to an external server, so the script never asks the
// binary for the local instance id.
func TestCodexSessionStartEnvironmentPrecedenceExternalOwnership(t *testing.T) {
	script := precCodexScript("session-start.sh")
	for _, name := range []string{"URL"} {
		for _, tt := range precedenceMatrix(name, "http://127.0.0.1:19001", "http://127.0.0.1:19002", "") {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				f := newStubFixture(t)
				stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q}`, filepath.ToSlash(f.home))
				runPrecedenceScript(t, f, script(t), stdin, tt.env, runOptions{})
				log := f.readLog(t)
				if external := tt.want != ""; strings.Contains(log, "qbit-jay instance-id") == external {
					t.Fatalf("external = %t but instance-id lookup = %t:\n%s", external, !external, log)
				}
			})
		}
	}
}

func TestCodexSessionStartEnvironmentPrecedenceDataDir(t *testing.T) {
	script := precCodexScript("session-start.sh")
	for _, tt := range precedenceMatrix("DATA_DIR", "new-data", "legacy-data", ".qbit-jay") {
		t.Run(tt.name, func(t *testing.T) {
			f := newStubFixture(t)
			env := map[string]string{}
			for k, v := range tt.env {
				if v != "" {
					v = filepath.ToSlash(filepath.Join(f.home, v))
				}
				env[k] = v
			}
			stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q}`, filepath.ToSlash(f.home))
			runPrecedenceScript(t, f, script(t), stdin, env, runOptions{})
			for _, dir := range []string{"new-data", "legacy-data", ".qbit-jay"} {
				_, err := os.Stat(filepath.Join(f.home, dir, "serve.err.log"))
				if exists := err == nil; exists != (dir == tt.want) {
					t.Fatalf("serve.err.log in %s exists = %t, want %t (data dir %s)", dir, exists, dir == tt.want, tt.want)
				}
			}
		})
	}
}

// post-compaction negotiates local compaction through the binary only when
// neither an explicit URL nor a socket is configured.
func TestCodexPostCompactionEnvironmentPrecedenceExternalOwnership(t *testing.T) {
	script := precCodexScript("post-compaction.sh")
	for _, name := range []string{"URL", "SOCKET"} {
		for _, tt := range precedenceMatrix(name, "http://127.0.0.1:19001", "http://127.0.0.1:19002", "") {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				f := newStubFixture(t)
				stdin := fmt.Sprintf(`{"session_id":"sess1","cwd":%q}`, filepath.ToSlash(f.home))
				runPrecedenceScript(t, f, script(t), stdin, tt.env, runOptions{})
				native := strings.Contains(f.readLog(t), "hook codex-resolve --local-compaction")
				if wantNative := tt.want == ""; native != wantNative {
					t.Fatalf("local compaction negotiated = %t, want %t", native, wantNative)
				}
			})
		}
	}
}
