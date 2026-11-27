// Package env reads qbit-jay's production environment variables.
//
// Every variable has a qbit-jay name (QBIT_JAY_<NAME>) and an upstream Engram
// name (ENGRAM_<NAME>) kept as a fallback for compatibility. When both are set
// the qbit-jay name wins. An empty qbit-jay value does not mask a set Engram
// value (an empty templated QBIT_JAY_HTTP_TOKEN must not disable a configured
// ENGRAM_HTTP_TOKEN); when only empty values are set, the variable still
// counts as set and the caller applies its upstream empty-value rule.
package env

import (
	"os"
	"strings"
)

const (
	prefix       = "QBIT_JAY_"
	legacyPrefix = "ENGRAM_"
)

func suffix(name string) string {
	name = strings.TrimPrefix(name, prefix)
	return strings.TrimPrefix(name, legacyPrefix)
}

// NewName returns the QBIT_JAY_ name for a variable given as NAME,
// ENGRAM_NAME or QBIT_JAY_NAME.
func NewName(name string) string { return prefix + suffix(name) }

// LegacyName returns the upstream ENGRAM_ name for the same variable.
func LegacyName(name string) string { return legacyPrefix + suffix(name) }

// Lookup returns the value and whether either name is set (os.LookupEnv
// semantics). QBIT_JAY_<NAME> is consulted first, then ENGRAM_<NAME>.
func Lookup(name string) (string, bool) {
	newValue, newSet := os.LookupEnv(NewName(name))
	if newSet && newValue != "" {
		return newValue, true
	}
	if legacyValue, legacySet := os.LookupEnv(LegacyName(name)); legacySet {
		return legacyValue, true
	}
	return newValue, newSet
}

// Get returns the value of the variable, or "" when neither name is set
// (os.Getenv semantics).
func Get(name string) string {
	v, _ := Lookup(name)
	return v
}
