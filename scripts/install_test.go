package scripts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run the real installers against a local fixture release (a temp
// directory served through QBIT_JAY_INSTALL_BASE_URL=file://...). They never
// touch the network, the user's PATH or the real install directories.

const fixtureVersion = "9.9.9"

// buildFakeBinary compiles a tiny program that prints "qbit-jay 9.9.9" for the
// host platform, so the installers can run the installed file.
func buildFakeBinary(t *testing.T) []byte {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("requires the go tool on PATH to build the fixture binary")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"qbit-jay "+fixtureVersion+"\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "fake-bin")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command(goTool, "build", "-o", out, src)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake binary: %v\n%s", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func tarGz(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range members {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipFile(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeRelease writes one archive plus a checksums.txt listing it with the given
// recorded digest ("" means the real digest) and returns the release directory.
func writeRelease(t *testing.T, archiveName string, archive []byte, recorded string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, archiveName), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if recorded == "" {
		recorded = sha256Hex(archive)
	}
	other := strings.Repeat("0", 64) + "  qbit-jay_" + fixtureVersion + "_plan9_riscv.tar.gz\n"
	sums := other + recorded + "  " + archiveName + "\n"
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sums), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func fileURL(dir string) string {
	slash := filepath.ToSlash(dir)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return "file://" + slash
}

func installedFile(dir, name string) string { return filepath.Join(dir, name) }

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func unixArchiveName(t *testing.T) string {
	t.Helper()
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("installer supports amd64 and arm64 only, not %s", runtime.GOARCH)
	}
	return "qbit-jay_" + fixtureVersion + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
}

func runInstallSh(t *testing.T, env ...string) (string, error) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires a POSIX shell on Linux or macOS")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("requires a POSIX shell: sh not found")
	}
	cmd := exec.Command(sh, filepath.Join(".", "install.sh"))
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallShInstallsAfterVerifyingChecksum(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"qbit-jay": buildFakeBinary(t), "tools/cloud-sync-projects.sh": []byte("#!/bin/sh\n")})
	release := writeRelease(t, unixArchiveName(t), archive, "")
	target := filepath.Join(t.TempDir(), "bin")

	out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Checksum verified", "Installed " + installedFile(target, "qbit-jay"), "qbit-jay " + fixtureVersion, "is not on your PATH"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	info, err := os.Stat(installedFile(target, "qbit-jay"))
	if err != nil {
		t.Fatalf("binary not installed: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed binary is not executable: %v", info.Mode())
	}
	leftovers, _ := filepath.Glob(filepath.Join(target, ".qbit-jay.new.*"))
	if len(leftovers) != 0 {
		t.Errorf("staging files left behind: %v", leftovers)
	}
	if _, err := os.Stat(installedFile(target, "cloud-sync-projects.sh")); err == nil {
		t.Error("only the binary may be installed, found extra archive members")
	}
}

func TestInstallShAbortsOnChecksumMismatch(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"qbit-jay": buildFakeBinary(t)})
	release := writeRelease(t, unixArchiveName(t), archive, strings.Repeat("a", 64))
	target := filepath.Join(t.TempDir(), "bin")

	out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
	if err == nil {
		t.Fatalf("a bad checksum must exit non-zero:\n%s", out)
	}
	if !strings.Contains(out, "checksum mismatch") {
		t.Errorf("output must explain the mismatch:\n%s", out)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		entries, _ := os.ReadDir(target)
		t.Errorf("nothing may be installed, but %s exists with %d entries", target, len(entries))
	}
}

func TestInstallShKeepsExistingBinaryOnChecksumMismatch(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"qbit-jay": buildFakeBinary(t)})
	release := writeRelease(t, unixArchiveName(t), archive, strings.Repeat("b", 64))
	target := t.TempDir()
	existing := installedFile(target, "qbit-jay")
	if err := os.WriteFile(existing, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target); err == nil {
		t.Fatalf("a bad checksum must exit non-zero:\n%s", out)
	}
	if data, _ := os.ReadFile(existing); string(data) != "old" {
		t.Errorf("existing binary was modified: %q", data)
	}
}

func TestInstallShRejectsReleaseWithoutThisPlatform(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"qbit-jay": buildFakeBinary(t)})
	release := writeRelease(t, "qbit-jay_"+fixtureVersion+"_plan9_riscv.tar.gz", archive, "")
	target := filepath.Join(t.TempDir(), "bin")

	out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
	if err == nil || !strings.Contains(out, "archive listed in checksums.txt") {
		t.Fatalf("a release without this platform must fail clearly (err=%v):\n%s", err, out)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Error("nothing may be installed")
	}
}

func TestInstallShRejectsArchiveWithoutBinary(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"other": []byte("x")})
	release := writeRelease(t, unixArchiveName(t), archive, "")
	target := filepath.Join(t.TempDir(), "bin")

	out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
	if err == nil || !strings.Contains(out, "does not contain qbit-jay") {
		t.Fatalf("an archive without the binary must fail (err=%v):\n%s", err, out)
	}
	if _, statErr := os.Stat(installedFile(target, "qbit-jay")); statErr == nil {
		t.Error("nothing may be installed")
	}
}

func TestInstallShPrintsNoPathHintWhenDirIsOnPath(t *testing.T) {
	archive := tarGz(t, map[string][]byte{"qbit-jay": buildFakeBinary(t)})
	release := writeRelease(t, unixArchiveName(t), archive, "")
	target := filepath.Join(t.TempDir(), "bin")

	out, err := runInstallSh(t, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target, "PATH="+target+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "is not on your PATH") {
		t.Errorf("no PATH hint expected when the directory is on PATH:\n%s", out)
	}
}

func powerShellExecutables(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows PowerShell")
	}
	var found []string
	for _, name := range []string{"powershell", "pwsh"} {
		if path, err := exec.LookPath(name); err == nil {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		t.Skip("requires Windows PowerShell")
	}
	return found
}

func windowsArchiveName(t *testing.T) string {
	t.Helper()
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("installer supports amd64 and arm64 only, not %s", runtime.GOARCH)
	}
	return "qbit-jay_" + fixtureVersion + "_windows_" + runtime.GOARCH + ".zip"
}

func runInstallPs1(t *testing.T, shell string, env ...string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	// A PSModulePath inherited from PowerShell 7 hides the built-in modules
	// (Get-FileHash) from Windows PowerShell 5.1, so let each shell compute its own.
	var base []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "PSMODULEPATH=") {
			base = append(base, kv)
		}
	}
	cmd.Env = append(base, append([]string{"QBIT_JAY_NO_MODIFY_PATH=1", "QBIT_JAY_VERSION="}, env...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Users who start `irm ... | iex` from a host whose PSModulePath points at the
// other PowerShell edition (Windows Terminal profiles, Git Bash, editors) do
// not get Get-FileHash; the installer must still verify the checksum.
func TestInstallPs1VerifiesChecksumWithoutUtilityModule(t *testing.T) {
	archive := zipFile(t, map[string][]byte{"qbit-jay.exe": buildFakeBinary(t)})
	release := writeRelease(t, windowsArchiveName(t), archive, "")
	for _, shell := range powerShellExecutables(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "bin")
			// PowerShell 7's modules first, as a pwsh-launched host passes them on:
			// Windows PowerShell 5.1 then cannot load Microsoft.PowerShell.Utility.
			polluted := t.TempDir()
			if pwsh7 := filepath.Join(os.Getenv("ProgramFiles"), "PowerShell", "7", "Modules"); dirExists(pwsh7) {
				polluted = pwsh7
			}
			out, err := runInstallPs1(t, shell, "PSModulePath="+polluted, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
			if err != nil {
				t.Fatalf("install failed without the utility module: %v\n%s", err, out)
			}
			if !strings.Contains(out, "Checksum verified") {
				t.Errorf("output missing checksum confirmation:\n%s", out)
			}
		})
	}
}

func TestInstallPs1InstallsAfterVerifyingChecksum(t *testing.T) {
	archive := zipFile(t, map[string][]byte{"qbit-jay.exe": buildFakeBinary(t), "tools/cloud-sync-projects.ps1": []byte("# x\n")})
	release := writeRelease(t, windowsArchiveName(t), archive, "")
	for _, shell := range powerShellExecutables(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "bin")
			out, err := runInstallPs1(t, shell, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
			if err != nil {
				t.Fatalf("install failed: %v\n%s", err, out)
			}
			for _, want := range []string{"Checksum verified", "qbit-jay " + fixtureVersion, "PATH left unchanged"} {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
			if _, err := os.Stat(installedFile(target, "qbit-jay.exe")); err != nil {
				t.Fatalf("binary not installed: %v", err)
			}
			if entries, _ := os.ReadDir(target); len(entries) != 1 {
				t.Errorf("only qbit-jay.exe may be installed, found %d entries", len(entries))
			}
		})
	}
}

func TestInstallPs1AbortsOnChecksumMismatch(t *testing.T) {
	archive := zipFile(t, map[string][]byte{"qbit-jay.exe": buildFakeBinary(t)})
	release := writeRelease(t, windowsArchiveName(t), archive, strings.Repeat("c", 64))
	for _, shell := range powerShellExecutables(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "bin")
			out, err := runInstallPs1(t, shell, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
			if err == nil {
				t.Fatalf("a bad checksum must exit non-zero:\n%s", out)
			}
			if !strings.Contains(out, "checksum mismatch") {
				t.Errorf("output must explain the mismatch:\n%s", out)
			}
			if _, statErr := os.Stat(target); statErr == nil {
				t.Errorf("nothing may be installed, but %s exists", target)
			}
		})
	}
}

func TestInstallPs1RejectsArchiveWithoutBinary(t *testing.T) {
	archive := zipFile(t, map[string][]byte{"other.exe": []byte("x")})
	release := writeRelease(t, windowsArchiveName(t), archive, "")
	for _, shell := range powerShellExecutables(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "bin")
			out, err := runInstallPs1(t, shell, "QBIT_JAY_INSTALL_BASE_URL="+fileURL(release), "QBIT_JAY_INSTALL_DIR="+target)
			if err == nil || !strings.Contains(out, "does not contain qbit-jay.exe") {
				t.Fatalf("an archive without the binary must fail (err=%v):\n%s", err, out)
			}
			if _, statErr := os.Stat(installedFile(target, "qbit-jay.exe")); statErr == nil {
				t.Error("nothing may be installed")
			}
		})
	}
}
