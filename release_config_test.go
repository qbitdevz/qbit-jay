package engram_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const expectedGoreleaserJob = `  goreleaser:
    # Forks never publish: the release belongs to qbitdevz/qbit-jay.
    if: github.repository == 'qbitdevz/qbit-jay'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6
        with:
          go-version: "1.26.11"

      - name: Verify module metadata is tidy
        run: go mod tidy -diff

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      # The workflow token cannot push to the tap and bucket repositories, so
      # GoReleaser only writes the files (skip_upload) and they ship as release
      # assets. The maintainer commits them (docs/qbit/RELEASING.md).
      - name: Attach Homebrew formula and Scoop manifest
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAG: ${{ github.ref_name }}
        run: gh release upload "$TAG" dist/homebrew/Formula/qbit-jay.rb dist/scoop/qbit-jay.json --clobber
`

func TestReleaseChecksModuleMetadataWithoutMutatingTaggedSources(t *testing.T) {
	root := releaseConfigRepoRoot(t)
	goreleaser := releaseConfigFile(t, filepath.Join(root, ".goreleaser.yaml"))
	if strings.Contains(goreleaser, "go mod tidy") {
		t.Fatal(".goreleaser.yaml must not run go mod tidy during a release")
	}

	workflow := releaseConfigFile(t, filepath.Join(root, ".github", "workflows", "release.yml"))
	job, ok := releaseWorkflowGoreleaserJob(workflow)
	if !ok {
		t.Fatal("release workflow must contain exactly one top-level goreleaser job")
	}
	if job != expectedGoreleaserJob {
		t.Errorf("unexpected goreleaser job; it must set up Go, run blocking `go mod tidy -diff`, run GoReleaser with the workflow token only, then attach the formula and manifest\nwant:\n%s\ngot:\n%s", expectedGoreleaserJob, job)
	}
}

// modulePath must carry the major-version suffix of the release line so that
// `go install <modulePath>/cmd/qbit-jay@latest` resolves the tagged release.
const modulePath = "github.com/qbitdevz/qbit-jay"

func TestModulePathMatchesReleaseMajorVersion(t *testing.T) {
	root := releaseConfigRepoRoot(t)
	goMod := strings.ReplaceAll(releaseConfigFile(t, filepath.Join(root, "go.mod")), "\r\n", "\n")
	firstLine, _, _ := strings.Cut(goMod, "\n")
	if want := "module " + modulePath; firstLine != want {
		t.Fatalf("go.mod module line = %q, want %q", firstLine, want)
	}

	install := "go install " + modulePath + "/cmd/qbit-jay@latest"
	installation := releaseConfigFile(t, filepath.Join(root, "docs", "INSTALLATION.md"))
	if strings.Count(installation, install) < 2 {
		t.Fatalf("docs/INSTALLATION.md must document %q for both Windows and macOS/Linux", install)
	}
}

// releaseWorkflowGoreleaserJob extracts this repository's one top-level job.
// It deliberately is not a YAML parser: the exact job text is the contract.
func releaseWorkflowGoreleaserJob(workflow string) (string, bool) {
	const jobMarker = "\n  goreleaser:\n"
	workflow = strings.ReplaceAll(workflow, "\r\n", "\n")
	if strings.Count(workflow, jobMarker) != 1 {
		return "", false
	}

	start := strings.Index(workflow, jobMarker) + 1
	job := workflow[start:]
	for offset := len(jobMarker) - 1; ; {
		nextJob := strings.Index(job[offset:], "\n  ")
		if nextJob < 0 {
			break
		}
		nextJob += offset
		if nextJob+3 == len(job) || job[nextJob+3] != ' ' {
			job = job[:nextJob]
			break
		}
		offset = nextJob + 3
	}
	return job, true
}

func releaseConfigRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func releaseConfigFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// TestReleaseNeedsNoPersonalAccessToken pins the credentials rule (T0.9): the
// workflow GITHUB_TOKEN cannot push to other repositories and no PAT exists, so
// GoReleaser writes the Homebrew formula and the Scoop manifest without
// uploading them and the workflow attaches both to the release.
func TestReleaseNeedsNoPersonalAccessToken(t *testing.T) {
	root := releaseConfigRepoRoot(t)
	goreleaser := strings.ReplaceAll(releaseConfigFile(t, filepath.Join(root, ".goreleaser.yaml")), "\r\n", "\n")
	for _, want := range []string{
		"project_name: qbit-jay",
		"owner: qbitdevz\n    name: qbit-jay",
		"binary: qbit-jay",
		"main: ./cmd/qbit-jay",
		"-X main.version={{.Version}}",
		"owner: qbitdevz\n      name: homebrew-tap\n    skip_upload: true",
		"owner: qbitdevz\n      name: scoop-bucket\n    skip_upload: true",
		`name_template: "checksums.txt"`,
	} {
		if !strings.Contains(goreleaser, want) {
			t.Errorf(".goreleaser.yaml must contain %q", want)
		}
	}
	// No token of any kind may be read from the environment by the config.
	for _, forbidden := range []string{"HOMEBREW_TAP_TOKEN", "SCOOP_BUCKET_TOKEN", "{{ ." + "Env", "skip_upload: auto", "Gentleman-Programming", "project_name: engram"} {
		if strings.Contains(goreleaser, forbidden) {
			t.Errorf(".goreleaser.yaml must not contain %q", forbidden)
		}
	}

	workflow := strings.ReplaceAll(releaseConfigFile(t, filepath.Join(root, ".github", "workflows", "release.yml")), "\r\n", "\n")
	for _, want := range []string{"permissions:\n  contents: write", "dist/homebrew/Formula/qbit-jay.rb", "dist/scoop/qbit-jay.json", "if: github.repository == 'qbitdevz/qbit-jay'"} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release.yml must contain %q", want)
		}
	}
	if got := strings.Count(workflow, "secrets."); got != strings.Count(workflow, "secrets.GITHUB_TOKEN") {
		t.Errorf("release.yml may use only secrets.GITHUB_TOKEN (%d secret references in total)", got)
	}
	for _, forbidden := range []string{"npm ", "registry.npmjs.org", "signpath", "SignPath", "id-token:", "HOMEBREW_TAP_TOKEN"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release.yml must not contain %q", forbidden)
		}
	}
}

// TestNoUpstreamBrandAssetsAreShipped pins spec S6 and S12: Engram's logos and
// screenshots are gone from the tree (TRADEMARKS.md), and the README header is
// the qbit-jay banner with the attribution line and no Engram logo.
func TestNoUpstreamBrandAssetsAreShipped(t *testing.T) {
	root := releaseConfigRepoRoot(t)

	for _, glob := range []string{
		"assets/branding/engram-*",
		"plugin/pi/assets/engram-*",
		"assets/agent-save.png",
		"assets/tui-*.png",
	} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(glob)))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("upstream brand asset(s) still shipped for %s: %v", glob, matches)
		}
	}

	readme := releaseConfigFile(t, filepath.Join(root, "README.md"))
	for _, forbidden := range []string{"engram-banner", "engram-logo", "engram-cloud-", "<h1>Engram"} {
		if strings.Contains(readme, forbidden) {
			t.Errorf("README.md still contains %q", forbidden)
		}
	}
	for _, want := range []string{
		"assets/branding/qbit-jay-banner.gif",
		"[Engram](https://github.com/Gentleman-Programming/engram) by Gentleman Programming",
		"go install github.com/qbitdevz/qbit-jay/cmd/qbit-jay@latest",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md must contain %q", want)
		}
	}

	license := releaseConfigFile(t, filepath.Join(root, "LICENSE"))
	for _, want := range []string{"Copyright (c) 2026 Alan Buscaglia", "Copyright (c) 2026 qbitdev (Leandro Alvarez)"} {
		if !strings.Contains(license, want) {
			t.Errorf("LICENSE must contain %q", want)
		}
	}
}
