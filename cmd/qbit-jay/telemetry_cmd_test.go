package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/qbitdevz/qbit-jay/internal/mcp"
	"github.com/qbitdevz/qbit-jay/internal/qbit/telemetry"
	"github.com/qbitdevz/qbit-jay/internal/store"
)

func seedTelemetry(t *testing.T, dir string) {
	t.Helper()
	rec := telemetry.New(telemetry.Options{DataDir: dir})
	if rec == nil {
		t.Fatal("telemetry should be on by default")
	}
	rec.ObserveTool("mem_search", map[string]any{"query": "alpha"}, `{"project":"p","results":[{"id":7}]}`, false, 3*time.Millisecond)
	rec.ObserveTool("mem_search", map[string]any{"query": "nothing"}, `{"project":"p"}`, false, time.Millisecond)
	rec.ObserveTool("mem_get_observation", map[string]any{"id": float64(7)}, `{"project":"p"}`, false, 0)
	rec.ObserveTool("mem_save", map[string]any{"content": "abc"}, "Failed to save: boom", true, time.Millisecond)
	rec.Close()
}

func TestStatsTelemetryPrintsSummary(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("QBIT_JAY_TELEMETRY", "")
	seedTelemetry(t, cfg.DataDir)

	withArgs(t, "qbit-jay", "stats", "--telemetry")
	out, _ := captureOutput(t, func() { cmdStats(cfg) })
	for _, want := range []string{"Searches: 2", "zero-result rate:     50.0%", "reformulation rate:   50.0%", "opens after search:   1", "#7 x1", "Saves: 1", "errors: 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stats --telemetry missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Sessions:") {
		t.Fatalf("--telemetry must not print the memory stats:\n%s", out)
	}
}

func TestStatsTelemetryWithoutFileCreatesNothing(t *testing.T) {
	cfg := testConfig(t)
	withArgs(t, "qbit-jay", "stats", "--telemetry")
	out, _ := captureOutput(t, func() { cmdStats(cfg) })
	if !strings.Contains(out, "No telemetry recorded yet") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if _, err := os.Stat(telemetry.Path(cfg.DataDir)); !os.IsNotExist(err) {
		t.Fatalf("stats must not create telemetry.db: %v", err)
	}
}

func TestTelemetryClear(t *testing.T) {
	cfg := testConfig(t)
	seedTelemetry(t, cfg.DataDir)

	oldScanner := stdinScanner
	t.Cleanup(func() { stdinScanner = oldScanner })
	stdinScanner = func() *bufio.Scanner { return bufio.NewScanner(strings.NewReader("n\n")) }
	withArgs(t, "qbit-jay", "telemetry", "clear")
	out, _ := captureOutput(t, func() { cmdTelemetry(cfg) })
	if _, err := os.Stat(telemetry.Path(cfg.DataDir)); err != nil {
		t.Fatalf("declining the prompt must keep the file: %v\n%s", err, out)
	}

	withArgs(t, "qbit-jay", "telemetry", "clear", "--yes")
	out, _ = captureOutput(t, func() { cmdTelemetry(cfg) })
	if _, err := os.Stat(telemetry.Path(cfg.DataDir)); !os.IsNotExist(err) {
		t.Fatalf("--yes must delete the file: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Fatalf("expected a confirmation, got:\n%s", out)
	}
	// A second clear is a no-op, not an error.
	out, _ = captureOutput(t, func() { cmdTelemetry(cfg) })
	if !strings.Contains(out, "nothing to delete") {
		t.Fatalf("expected nothing-to-delete, got:\n%s", out)
	}
}

func runMCPCapture(t *testing.T, cfg store.Config) mcp.MCPConfig {
	t.Helper()
	var got mcp.MCPConfig
	oldNew, oldServe := newMCPServerWithConfig, serveMCP
	t.Cleanup(func() { newMCPServerWithConfig, serveMCP = oldNew, oldServe })
	newMCPServerWithConfig = func(s *store.Store, c mcp.MCPConfig, a map[string]bool) *mcpserver.MCPServer {
		got = c
		return oldNew(s, c, a)
	}
	serveMCP = func(*mcpserver.MCPServer, ...mcpserver.StdioOption) error { return nil }
	withArgs(t, "qbit-jay", "mcp")
	captureOutput(t, func() { cmdMCP(cfg) })
	return got
}

func TestCmdMCPInstallsTelemetryObserverUnlessOff(t *testing.T) {
	t.Run("default is on", func(t *testing.T) {
		cfg := testConfig(t)
		t.Setenv("QBIT_JAY_TELEMETRY", "")
		if runMCPCapture(t, cfg).Observer == nil {
			t.Fatal("telemetry observer should be installed by default")
		}
	})
	t.Run("off installs nothing and creates no file", func(t *testing.T) {
		cfg := testConfig(t)
		t.Setenv("QBIT_JAY_TELEMETRY", "off")
		if runMCPCapture(t, cfg).Observer != nil {
			t.Fatal("no observer expected when telemetry is off")
		}
		if _, err := os.Stat(telemetry.Path(cfg.DataDir)); !os.IsNotExist(err) {
			t.Fatalf("telemetry.db must not exist when off: %v", err)
		}
	})
}
