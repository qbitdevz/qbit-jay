package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/qbitdevz/qbit-jay/internal/qbit/engramimport"
	"github.com/qbitdevz/qbit-jay/internal/qbit/paths"
)

const importFromEngramUsage = "usage: qbit-jay import --from-engram [path] [--force]\n" +
	"  path    an Engram data directory or engram.db (default: ENGRAM_DATA_DIR, else ~/.engram)\n" +
	"  --force set the existing qbit-jay database aside as a timestamped backup and import over it\n"

// autoImportFromEngram runs the first-start import (spec 6.3). A failure is
// reported on stderr and never stops the command: qbit-jay still starts, and
// `qbit-jay import --from-engram --force` retries explicitly.
func autoImportFromEngram(dataDir string, stderr io.Writer) {
	if _, err := engramimport.AutoImport(dataDir, engramimport.Options{Stderr: stderr}); err != nil {
		_, _ = fmt.Fprintf(stderr, "qbit-jay: warning: %v\n  Continuing with an empty database. Retry with: qbit-jay import --from-engram --force\n", err)
	}
}

// runImportFromEngram implements `qbit-jay import --from-engram [path] [--force]`
// and returns the process exit code.
func runImportFromEngram(args []string, dataDir string, stdout, stderr io.Writer) int {
	var source string
	var force bool
	for _, arg := range args {
		switch {
		case arg == "--force":
			force = true
		case strings.HasPrefix(arg, "-"):
			_, _ = fmt.Fprintf(stderr, "qbit-jay import --from-engram: unknown flag %s\n%s", arg, importFromEngramUsage)
			return 2
		case source == "":
			source = arg
		default:
			_, _ = fmt.Fprint(stderr, importFromEngramUsage)
			return 2
		}
	}
	res, err := engramimport.Explicit(source, dataDir, force, engramimport.Options{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qbit-jay import --from-engram: %v\n", err)
		return 1
	}
	if res.BackupOfExisting != "" {
		_, _ = fmt.Fprintf(stdout, "Set aside the existing database as %s\n", res.BackupOfExisting)
	}
	_, _ = fmt.Fprintf(stdout, "Imported Engram data from %s into %s (%d KB). The Engram database was not modified.\n", res.Source, res.Destination, res.Bytes/1024)
	return 0
}

// stageEngramSource lets migrate-copy accept a directory that holds Engram's
// engram.db instead of qbit-jay.db: it copies the database with the backup API
// into a staging directory next to the identity file, so MigrateCopy verifies
// and copies it like a qbit-jay source. The caller removes the staging directory.
func stageEngramSource(source string) (staging string, err error) {
	if _, e := os.Stat(filepath.Join(source, paths.DatabaseFile)); e == nil {
		return "", nil
	}
	engramDB := filepath.Join(source, engramimport.EngramDatabaseFile)
	if _, e := os.Stat(engramDB); e != nil {
		return "", nil
	}
	identity, err := os.ReadFile(filepath.Join(source, ".instance-id"))
	if err != nil {
		return "", fmt.Errorf("source instance identity: %w", err)
	}
	staging, err = os.MkdirTemp("", "qbit-jay-engram-stage-")
	if err != nil {
		return "", err
	}
	if err = engramimport.Copy(engramDB, staging); err == nil {
		err = os.WriteFile(filepath.Join(staging, ".instance-id"), identity, 0o600)
	}
	if err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}
