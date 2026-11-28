package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec D1: new repos get .qbit-jay/; a repo that only has .engram/ keeps using it.
func TestWriteInitConfigFolderChoice(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     string
	}{
		{name: "fresh directory", want: ".qbit-jay"},
		{name: "only .engram", existing: []string{".engram"}, want: ".engram"},
		{name: "both", existing: []string{".engram", ".qbit-jay"}, want: ".qbit-jay"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tc.existing {
				if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeInitConfig(dir, []byte(`{"project_name":"demo"}`), false); err != nil {
				t.Fatalf("writeInitConfig: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, tc.want, "config.json")); err != nil {
				t.Fatalf("config.json not in %s: %v", tc.want, err)
			}
			other := ".engram"
			if tc.want == ".engram" {
				other = ".qbit-jay"
			}
			if _, err := os.Stat(filepath.Join(dir, other, "config.json")); err == nil && len(tc.existing) < 2 {
				t.Fatalf("config.json was also written to %s", other)
			}
		})
	}
}
