package env

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		newSet    *string
		legacySet *string
		want      string
		wantOK    bool
	}{
		{name: "neither set", want: "", wantOK: false},
		{name: "legacy only", legacySet: ptr("old"), want: "old", wantOK: true},
		{name: "new only", newSet: ptr("new"), want: "new", wantOK: true},
		{name: "both set new wins", newSet: ptr("new"), legacySet: ptr("old"), want: "new", wantOK: true},
		{name: "new set but empty keeps upstream empty semantics", newSet: ptr(""), legacySet: ptr("old"), want: "", wantOK: true},
		{name: "legacy set but empty", legacySet: ptr(""), want: "", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearBoth(t, "ZZ_PROBE")
			if tt.newSet != nil {
				t.Setenv("QBIT_JAY_ZZ_PROBE", *tt.newSet)
			}
			if tt.legacySet != nil {
				t.Setenv("ENGRAM_ZZ_PROBE", *tt.legacySet)
			}
			got, ok := Lookup("ZZ_PROBE")
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("Lookup = (%q,%v), want (%q,%v)", got, ok, tt.want, tt.wantOK)
			}
			if Get("ZZ_PROBE") != tt.want {
				t.Fatalf("Get = %q, want %q", Get("ZZ_PROBE"), tt.want)
			}
		})
	}
}

func TestNameAcceptsEitherPrefix(t *testing.T) {
	clearBoth(t, "ZZ_PROBE")
	t.Setenv("QBIT_JAY_ZZ_PROBE", "new")
	for _, name := range []string{"ZZ_PROBE", "ENGRAM_ZZ_PROBE", "QBIT_JAY_ZZ_PROBE"} {
		if got := Get(name); got != "new" {
			t.Fatalf("Get(%q) = %q, want new", name, got)
		}
	}
}

func TestNewName(t *testing.T) {
	if got := NewName("ENGRAM_PORT"); got != "QBIT_JAY_PORT" {
		t.Fatalf("NewName = %q", got)
	}
	if got := NewName("PORT"); got != "QBIT_JAY_PORT" {
		t.Fatalf("NewName = %q", got)
	}
}

func clearBoth(t *testing.T, name string) {
	t.Helper()
	for _, key := range []string{"QBIT_JAY_" + name, "ENGRAM_" + name} {
		if old, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { _ = os.Setenv(key, old) })
		}
		_ = os.Unsetenv(key)
	}
}

func ptr(s string) *string { return &s }

// TestNoProductionDirectEngramEnvReads fails when a production file reads an
// ENGRAM_* variable straight from the process environment instead of going
// through this package. Upstream syncs add new call sites; this catches them.
func TestNoProductionDirectEngramEnvReads(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var offenders []string
	var files []string
	for _, top := range []string{"cmd", "internal"} {
		_ = filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			slash := filepath.ToSlash(path)
			if d.IsDir() {
				if strings.HasSuffix(slash, "internal/qbit/env") || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
	}
	// Constants naming an ENGRAM_ variable may be declared in a sibling file of
	// the same package, so collect them per directory first.
	consts := map[string]map[string]bool{}
	for _, path := range files {
		dir := filepath.Dir(path)
		if consts[dir] == nil {
			consts[dir] = map[string]bool{}
		}
		collectEngramConsts(t, path, consts[dir])
	}
	for _, path := range files {
		offenders = append(offenders, directEngramEnvReads(t, path, consts[filepath.Dir(path)])...)
	}
	if len(offenders) > 0 {
		t.Fatalf("production code must read ENGRAM_* through internal/qbit/env, found %d direct reads:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}

func collectEngramConsts(t *testing.T, path string, consts map[string]bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, v := range spec.Values {
			if lit, ok := v.(*ast.BasicLit); ok && i < len(spec.Names) && isEngramLiteral(lit) {
				consts[spec.Names[i].Name] = true
			}
		}
		return true
	})
}

func directEngramEnvReads(t *testing.T, path string, consts map[string]bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Getenv" && sel.Sel.Name != "LookupEnv") {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "os" {
			return true
		}
		switch arg := call.Args[0].(type) {
		case *ast.BasicLit:
			if isEngramLiteral(arg) {
				found = append(found, fset.Position(call.Pos()).String())
			}
		case *ast.Ident:
			if consts[arg.Name] {
				found = append(found, fset.Position(call.Pos()).String())
			}
		}
		return true
	})
	return found
}

func isEngramLiteral(lit *ast.BasicLit) bool {
	return lit.Kind == token.STRING && strings.HasPrefix(strings.Trim(lit.Value, "\"`"), "ENGRAM_")
}
