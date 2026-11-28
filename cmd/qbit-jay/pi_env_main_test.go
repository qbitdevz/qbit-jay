package main

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/qbitdevz/qbit-jay/internal/diagnostic"
)

// Pi exports PI_CODING_AGENT to every process it starts, and the Claude hook
// is a no-op when it is set. Clear it so the hook tests behave the same from a
// Pi shell as from CI. The Pi guard itself is covered in package plugin_test.
// Hook tests also publish host bindings; keep those out of the developer's real
// Engram data directory.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("PI_CODING_AGENT"); err != nil {
		fmt.Fprintf(os.Stderr, "clear PI_CODING_AGENT: %v\n", err)
		os.Exit(1)
	}
	// Commands that open the store would import the developer's real
	// ~/.engram database on first start. Tests of that import opt back in.
	if err := os.Setenv("QBIT_JAY_NO_IMPORT", "1"); err != nil {
		fmt.Fprintf(os.Stderr, "set QBIT_JAY_NO_IMPORT: %v\n", err)
		os.Exit(1)
	}
	// Doctor reports must not depend on what listens on this machine's port
	// 7437 (a developer may run Engram there); the port tests opt back in.
	doctorEngramPortCheck = func(int) *diagnostic.CheckResult { return nil }
	serveEngramPortCheck = func(int) error { return nil }
	// Test subprocesses must never recover a developer's drive-root database.
	// Migration dispatch assertions install their own seam.
	migrateOrphanedDatabase = func(string) {}

	// Unrelated CLI subprocesses may deliberately have no writable temp root.
	// Retain the suite's temp root before individual tests change their env.
	tempRoot := os.TempDir()
	compatibilityBinary = newCompatibilityBinaryFixture(tempRoot)
	var dir string
	var once sync.Once
	hostBindingDataDir = func() string {
		once.Do(func() {
			var err error
			dir, err = os.MkdirTemp(tempRoot, "engram-hostbind-test-")
			if err != nil {
				fmt.Fprintf(os.Stderr, "create host binding dir: %v\n", err)
				os.Exit(1)
			}
		})
		return dir
	}
	code := m.Run()
	if err := compatibilityBinary.close(); err != nil {
		fmt.Fprintf(os.Stderr, "remove CLI fixture: %v\n", err)
		code = 1
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}
