package main

import (
	"strings"
	"testing"
)

// Spec S2: usage and help name the qbit-jay command and document the
// QBIT_JAY_* variables with their ENGRAM_* fallback.
func TestUsageNamesQbitJayAndDocumentsEnv(t *testing.T) {
	out, _ := captureOutput(t, func() { printUsage() })
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "qbit-jay v") {
		t.Fatalf("first help line = %q, want it to start with %q", lines[0], "qbit-jay v")
	}
	for _, want := range []string{
		"qbit-jay <command> [arguments]",
		"QBIT_JAY_DATA_DIR",
		"QBIT_JAY_PORT",
		"QBIT_JAY_PROJECT",
		"ENGRAM_",
		"fallback",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("usage does not contain %q", want)
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "engram ") {
			t.Errorf("usage still tells the user to run the old command: %q", line)
		}
	}
}
