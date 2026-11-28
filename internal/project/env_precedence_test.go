package project

import (
	"os"
	"testing"
)

func TestProcessOverridePrefersQbitJayOverEngram(t *testing.T) {
	tests := []struct {
		name       string
		qbit, engr string
		want       string
		wantOK     bool
	}{
		{name: "neither", want: "", wantOK: false},
		{name: "only legacy", engr: "legacy-proj", want: "legacy-proj", wantOK: true},
		{name: "only new", qbit: "new-proj", want: "new-proj", wantOK: true},
		{name: "both set new wins", qbit: "new-proj", engr: "legacy-proj", want: "new-proj", wantOK: true},
		{name: "blank new keeps upstream blank semantics", qbit: "  ", engr: "legacy-proj", want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setOrUnset(t, "QBIT_JAY_PROJECT", tt.qbit)
			setOrUnset(t, "ENGRAM_PROJECT", tt.engr)
			got, ok := ProcessOverride("")
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("ProcessOverride = (%q,%v), want (%q,%v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
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
