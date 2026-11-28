package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/daemon"
	"github.com/qbitdevz/qbit-jay/internal/store"
)

// Spec S4 (6.3 Puerto y socket): when Engram holds the port, doctor and the
// server start say so instead of failing silently or talking to it.

// fakeHealthServer answers /health like the given product on 127.0.0.1.
func fakeHealthServer(t *testing.T, body map[string]any) int {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on loopback: %v", err)
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = listener
	srv.Start()
	t.Cleanup(srv.Close)
	return listener.Addr().(*net.TCPAddr).Port
}

var engramHealth = map[string]any{"status": "ok", "service": "engram", "version": "3.7.2", "instance_id": "deadbeef"}

func qbitJayHealth() map[string]any {
	return map[string]any{"status": "ok", "service": "engram", "product": "qbit-jay", "version": "0.1.0", "instance_id": "cafebabe"}
}

func TestProbeIdentifiesProduct(t *testing.T) {
	res := daemon.Probe(context.Background(), fakeHealthServer(t, engramHealth), time.Second)
	if res.Status != daemon.StatusRunning || res.Product != "" || res.Version != "3.7.2" {
		t.Fatalf("Engram probe = %+v", res)
	}
	res = daemon.Probe(context.Background(), fakeHealthServer(t, qbitJayHealth()), time.Second)
	if res.Product != "qbit-jay" {
		t.Fatalf("qbit-jay probe = %+v", res)
	}
}

func TestListenerConflictNamesEngram(t *testing.T) {
	port := fakeHealthServer(t, engramHealth)
	err := listenerConflict(port)
	if err == nil {
		t.Fatal("an Engram listener must be a conflict")
	}
	for _, want := range []string{"Engram", "3.7.2", strconv.Itoa(port), "QBIT_JAY_PORT"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "unmanaged qbit-jay listener") {
		t.Fatalf("Engram must not be reported as a qbit-jay listener: %v", err)
	}
	// A real qbit-jay listener keeps the existing wording.
	err = listenerConflict(fakeHealthServer(t, qbitJayHealth()))
	if err == nil || !strings.Contains(err.Error(), "unmanaged qbit-jay listener") {
		t.Fatalf("qbit-jay listener message changed: %v", err)
	}
}

func TestServeRefusesWhenEngramHoldsThePort(t *testing.T) {
	port := fakeHealthServer(t, engramHealth)
	opened := false
	old := storeNew
	storeNew = func(store.Config) (*store.Store, error) { opened = true; return nil, os.ErrInvalid }
	t.Cleanup(func() { storeNew = old })

	err := serveWith(store.Config{DataDir: t.TempDir()}, serveOptions{port: port})
	if err == nil || !strings.Contains(err.Error(), "Engram") || !strings.Contains(err.Error(), "QBIT_JAY_PORT") {
		t.Fatalf("serve must explain that Engram holds the port, got %v", err)
	}
	if opened {
		t.Fatal("serve must not open the store before the port check")
	}
}

func TestDoctorReportsEngramOnThePort(t *testing.T) {
	port := fakeHealthServer(t, engramHealth)
	t.Setenv("QBIT_JAY_PORT", strconv.Itoa(port))
	t.Setenv("ENGRAM_PORT", "")
	cfg := store.Config{DataDir: t.TempDir(), MaxObservationLength: 50000, MaxContextResults: 20, MaxSearchResults: 20, DedupeWindow: time.Hour}
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"qbit-jay", "doctor", "--json"}
	stdout, _ := captureOutput(t, func() { cmdDoctor(cfg) })
	var report struct {
		Checks []struct {
			CheckID string `json:"check_id"`
			Result  string `json:"result"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("doctor output: %v\n%s", err, stdout)
	}
	for _, c := range report.Checks {
		if c.CheckID == "engram_on_port" {
			if c.Result != "warning" || !strings.Contains(c.Message, "Engram") || !strings.Contains(c.Message, "QBIT_JAY_PORT") {
				t.Fatalf("engram_on_port = %+v", c)
			}
			return
		}
	}
	t.Fatalf("doctor has no engram_on_port check:\n%s", stdout)
}

func TestDoctorSilentWhenPortIsFreeOrQbitJay(t *testing.T) {
	for name, port := range map[string]func() int{
		"qbit-jay": func() int { return fakeHealthServer(t, qbitJayHealth()) },
		"free": func() int {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Skip(err)
			}
			p := l.Addr().(*net.TCPAddr).Port
			_ = l.Close()
			return p
		},
	} {
		t.Run(name, func(t *testing.T) {
			if finding := engramPortFinding(port()); finding != nil {
				t.Fatalf("unexpected finding: %+v", finding)
			}
		})
	}
}

func TestHookIdentityDenialNamesEngram(t *testing.T) {
	msg := hookLocalIdentityMismatch(hookLocalHealth{Status: "ok", Service: "engram", InstanceID: "x", Version: json.RawMessage(`"3.7.2"`)}).Error()
	if !strings.Contains(msg, "Engram") || !strings.Contains(msg, "QBIT_JAY_PORT") {
		t.Fatalf("an Engram listener must be named in the hook denial: %q", msg)
	}
	msg = hookLocalIdentityMismatch(hookLocalHealth{Status: "ok", Service: "engram", Product: "qbit-jay", InstanceID: "x"}).Error()
	if strings.Contains(msg, "Engram") {
		t.Fatalf("a qbit-jay listener must not be called Engram: %q", msg)
	}
}
