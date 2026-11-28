package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProjectConfig(t *testing.T, dir, folder, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"project_name":"` + name + `"}`
	if err := os.WriteFile(filepath.Join(dir, folder, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Spec D1: .qbit-jay/ is the project folder; a repo that only has the upstream
// .engram/ keeps working; when both exist .qbit-jay/ wins.
func TestConfigReadProjectFolderChoice(t *testing.T) {
	tests := []struct {
		name    string
		folders map[string]string
		want    string
		found   bool
	}{
		{name: "no folder", found: false},
		{name: "only qbit-jay", folders: map[string]string{".qbit-jay": "new-name"}, want: "new-name", found: true},
		{name: "only legacy", folders: map[string]string{".engram": "old-name"}, want: "old-name", found: true},
		{name: "both prefers qbit-jay", folders: map[string]string{".qbit-jay": "new-name", ".engram": "old-name"}, want: "new-name", found: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for folder, name := range tt.folders {
				writeProjectConfig(t, dir, folder, name)
			}
			res, found := readConfigAt(dir)
			if found != tt.found || res.Project != tt.want {
				t.Fatalf("readConfigAt = (%q, %v), want (%q, %v)", res.Project, found, tt.want, tt.found)
			}
		})
	}
}
