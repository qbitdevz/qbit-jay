package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectFolderResolution(t *testing.T) {
	tests := []struct {
		name string
		make []string
		want string
	}{
		{name: "fresh directory gets the qbit-jay folder", want: ".qbit-jay"},
		{name: "only legacy folder keeps using it", make: []string{".engram"}, want: ".engram"},
		{name: "both present prefers qbit-jay", make: []string{".engram", ".qbit-jay"}, want: ".qbit-jay"},
		{name: "only qbit-jay", make: []string{".qbit-jay"}, want: ".qbit-jay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range tt.make {
				if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if got := ProjectFolderName(dir); got != tt.want {
				t.Fatalf("ProjectFolderName = %q, want %q", got, tt.want)
			}
			if got, want := ProjectFolder(dir), filepath.Join(dir, tt.want); got != want {
				t.Fatalf("ProjectFolder = %q, want %q", got, want)
			}
		})
	}
}

func TestProjectFolderNeverTouchesTheDisk(t *testing.T) {
	dir := t.TempDir()
	_ = ProjectFolder(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("resolver created %d entries; it must only resolve", len(entries))
	}
}

func TestNames(t *testing.T) {
	if DataDirName != ".qbit-jay" || DatabaseFile != "qbit-jay.db" {
		t.Fatalf("DataDirName=%q DatabaseFile=%q", DataDirName, DatabaseFile)
	}
}
