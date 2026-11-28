package main

import (
	"fmt"
	"strings"

	"github.com/qbitdevz/qbit-jay/internal/daemon"
	"github.com/qbitdevz/qbit-jay/internal/diagnostic"
	"github.com/qbitdevz/qbit-jay/internal/qbit/product"
)

// isEngramListener reports a probe that answered like Engram: the shared
// health protocol (service "engram") without the qbit-jay product marker.
func isEngramListener(probe daemon.ProbeResult) bool {
	return probe.Status == daemon.StatusRunning && probe.Service == "engram" && probe.Product != product.ID
}

// engramPortAdvice explains how to proceed when Engram holds the port (spec 6.3).
func engramPortAdvice(port int, probe daemon.ProbeResult) string {
	who := "an Engram server"
	if v := strings.TrimSpace(probe.Version); v != "" {
		who += " (version " + v + ")"
	}
	return fmt.Sprintf("%s is running on port %d, the port qbit-jay uses by default. qbit-jay will not use it or send hooks and plugins to it. Stop Engram (for example `engram stop`, or its service or launcher) and start qbit-jay again, or keep both: set QBIT_JAY_PORT to a free port for qbit-jay (and the same value wherever qbit-jay hooks and plugins run)", who, port)
}

// engramPortError returns the explanation when Engram holds port, else nil.
func engramPortError(port int) error {
	if probe := daemonStatusProbe(port); isEngramListener(probe) {
		return fmt.Errorf("%s", engramPortAdvice(port, probe))
	}
	return nil
}

// serveEngramPortCheck is the seam cmdServe calls; the package tests stub it so a
// real Engram on the machine's port 7437 cannot stop them from serving.
var serveEngramPortCheck = engramPortError

// doctorEngramPortCheck is the seam doctor calls; tests that compare whole
// doctor reports stub it so a real Engram on the machine's port 7437 cannot
// change their output.
var doctorEngramPortCheck = engramPortFinding

// engramPortFinding is the doctor check for the same condition.
func engramPortFinding(port int) *diagnostic.CheckResult {
	probe := daemonStatusProbe(port)
	if !isEngramListener(probe) {
		return nil
	}
	message := engramPortAdvice(port, probe)
	evidence := fmt.Sprintf(`{"port":%d,"service":"engram","version":%q}`, port, probe.Version)
	return &diagnostic.CheckResult{
		CheckID: "engram_on_port", Result: "warning", Severity: "warning", ReasonCode: "engram_on_port",
		Message:      message,
		Why:          "Engram and qbit-jay both default to port 7437 and speak the same health protocol; qbit-jay refuses to treat an Engram server as its own.",
		Evidence:     []byte(evidence),
		SafeNextStep: "Stop Engram, or set QBIT_JAY_PORT to a free port.",
		Findings:     []diagnostic.Finding{{CheckID: "engram_on_port", Severity: "warning", ReasonCode: "engram_on_port", Message: message, Why: "The port is occupied by Engram.", Evidence: []byte(evidence), SafeNextStep: "Stop Engram, or set QBIT_JAY_PORT to a free port."}},
	}
}

// listenerConflict observes the selected port without adopting or signaling its
// owner. Health metadata and an OS-reported owner PID are diagnostic snapshots;
// only the existing pidfile/start-token checks authorize stopping a daemon.
func listenerConflict(port int) error {
	return listenerConflictForProbe(port, daemonStatusProbe(port))
}

func listenerConflictForProbe(port int, probe daemon.ProbeResult) error {
	if probe.Status == daemon.StatusNotRunning {
		return nil
	}
	if isEngramListener(probe) {
		return fmt.Errorf("%s", engramPortAdvice(port, probe))
	}
	kind := "unknown listener"
	if probe.Status == daemon.StatusRunning && probe.Service == "engram" {
		kind = "unmanaged qbit-jay listener"
	}
	owner := "owning PID unavailable"
	if pid := daemon.ListenerOwnerPID(port); pid > 0 {
		owner = fmt.Sprintf("observed owning pid %d", pid)
	}
	return fmt.Errorf("%s on port %d (%s); not managed by this pidfile; not signaling or adopting it; verify the owner locally and stop it manually if appropriate", kind, port, owner)
}

func daemonStatusPort(record daemon.PidRecord) int {
	port, _ := resolveRestartPort(nil, record)
	return port
}

func listenerObservation(port int, probe daemon.ProbeResult) string {
	if err := listenerConflictForProbe(port, probe); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("no listener answered on port %d", port)
}
