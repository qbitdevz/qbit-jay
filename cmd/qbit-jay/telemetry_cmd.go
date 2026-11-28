package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/qbitdevz/qbit-jay/internal/qbit/telemetry"
	"github.com/qbitdevz/qbit-jay/internal/store"
)

// newTelemetryRecorder returns the local telemetry recorder for cfg, or nil
// when telemetry is off. The recorder is a mcp.ToolObserver.
func newTelemetryRecorder(cfg store.Config) *telemetry.Recorder {
	return telemetry.New(telemetry.Options{DataDir: cfg.DataDir})
}

// cmdStatsTelemetry implements `qbit-jay stats --telemetry`. It only reads
// telemetry.db and never creates it.
func cmdStatsTelemetry(cfg store.Config) {
	sum, err := telemetry.Load(cfg.DataDir)
	if err != nil {
		fatal(fmt.Errorf("read telemetry: %w", err))
		return
	}
	sum.Render(os.Stdout)
}

// cmdTelemetry implements `qbit-jay telemetry clear [--yes]`.
func cmdTelemetry(cfg store.Config) {
	if len(os.Args) < 3 || os.Args[2] != "clear" {
		fatal(fmt.Errorf("usage: qbit-jay telemetry clear [--yes]"))
		return
	}
	yes := false
	for _, arg := range os.Args[3:] {
		switch arg {
		case "--yes", "-y":
			yes = true
		default:
			fatal(fmt.Errorf("unknown flag for telemetry clear: %s", arg))
			return
		}
	}
	path := telemetry.Path(cfg.DataDir)
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("No telemetry file at %s: nothing to delete.\n", path)
		return
	}
	if !yes {
		fmt.Printf("Delete all local telemetry (%s)? [y/N] ", path)
		scanner := stdinScanner()
		answer := ""
		if scanner.Scan() {
			answer = strings.ToLower(strings.TrimSpace(scanner.Text()))
		}
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled; telemetry kept. Use --yes to delete without a prompt.")
			return
		}
	}
	if _, err := telemetry.Clear(cfg.DataDir); err != nil {
		fatal(fmt.Errorf("delete telemetry: %w", err))
		return
	}
	fmt.Printf("Local telemetry deleted (%s).\n", path)
}
