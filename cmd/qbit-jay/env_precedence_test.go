package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The helper is unit-tested in internal/qbit/env; these tests prove the real
// call paths read QBIT_JAY_* first and ENGRAM_* as fallback (spec S5).

func TestServeOptionsPortPrefersQbitJayOverEngram(t *testing.T) {
	tests := []struct {
		name       string
		qbit, engr string
		want       int
	}{
		{name: "neither uses upstream default", want: 7437},
		{name: "only legacy is used", engr: "9101", want: 9101},
		{name: "only new is used", qbit: "9202", want: 9202},
		{name: "new wins when both set", qbit: "9202", engr: "9101", want: 9202},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setOrUnset(t, "QBIT_JAY_PORT", tt.qbit)
			setOrUnset(t, "ENGRAM_PORT", tt.engr)
			setOrUnset(t, "QBIT_JAY_SOCKET", "")
			setOrUnset(t, "ENGRAM_SOCKET", "")
			opts, err := resolveServeOptions(nil)
			if err != nil {
				t.Fatalf("resolveServeOptions: %v", err)
			}
			if opts.port != tt.want {
				t.Fatalf("port = %d, want %d", opts.port, tt.want)
			}
		})
	}
}

func TestHostBindingDataDirPrefersQbitJayOverEngram(t *testing.T) {
	newDir := filepath.Join(t.TempDir(), "new")
	oldDir := filepath.Join(t.TempDir(), "old")
	t.Run("only legacy", func(t *testing.T) {
		setOrUnset(t, "QBIT_JAY_DATA_DIR", "")
		setOrUnset(t, "ENGRAM_DATA_DIR", oldDir)
		if got := hostBindingDataDirFromEnvOrConfig(); got != oldDir {
			t.Fatalf("got %q, want %q", got, oldDir)
		}
	})
	t.Run("only new", func(t *testing.T) {
		setOrUnset(t, "QBIT_JAY_DATA_DIR", newDir)
		setOrUnset(t, "ENGRAM_DATA_DIR", "")
		if got := hostBindingDataDirFromEnvOrConfig(); got != newDir {
			t.Fatalf("got %q, want %q", got, newDir)
		}
	})
	t.Run("both set new wins", func(t *testing.T) {
		setOrUnset(t, "QBIT_JAY_DATA_DIR", newDir)
		setOrUnset(t, "ENGRAM_DATA_DIR", oldDir)
		if got := hostBindingDataDirFromEnvOrConfig(); got != newDir {
			t.Fatalf("got %q, want %q", got, newDir)
		}
	})
	t.Run("neither falls back to the default config dir", func(t *testing.T) {
		setOrUnset(t, "QBIT_JAY_DATA_DIR", "")
		setOrUnset(t, "ENGRAM_DATA_DIR", "")
		cfg, err := storeDefaultConfig()
		if err != nil {
			t.Skip(err)
		}
		if got := hostBindingDataDirFromEnvOrConfig(); got != cfg.DataDir {
			t.Fatalf("got %q, want %q", got, cfg.DataDir)
		}
	})
}

// setOrUnset sets key to value, or removes it entirely when value is empty, so
// "not set" and "set but empty" stay distinguishable. It restores the old state.
func setOrUnset(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, "x") // registers restoration of the original state
	if value == "" {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, value)
}
