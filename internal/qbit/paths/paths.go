// Package paths holds qbit-jay's on-disk names: the data directory, the
// database file and the per-repository project folder.
//
// Existing Engram data is never read or moved here; importing it is a separate
// migration step. The one exception is the project folder, where a repository
// that only has the upstream .engram/ folder keeps using it.
package paths

import (
	"os"
	"path/filepath"
)

const (
	// DataDirName is the per-user data directory, created under the home directory.
	DataDirName = ".qbit-jay"
	// DatabaseFile is the SQLite database inside the data directory.
	DatabaseFile = "qbit-jay.db"

	// ProjectFolderNameDefault is the per-repository folder (config.json and
	// the git sync chunks) that new repositories get.
	ProjectFolderNameDefault = ".qbit-jay"
	// LegacyProjectFolderName is the upstream folder name. A repository that has
	// it and no .qbit-jay/ keeps using it so existing repositories keep working.
	LegacyProjectFolderName = ".engram"
)

// ProjectFolderName resolves the project folder name for the repository or
// directory dir: .qbit-jay when it exists, otherwise .engram when only that
// exists, otherwise .qbit-jay for a new project. It only reads the disk.
func ProjectFolderName(dir string) string {
	if exists(filepath.Join(dir, ProjectFolderNameDefault)) {
		return ProjectFolderNameDefault
	}
	if exists(filepath.Join(dir, LegacyProjectFolderName)) {
		return LegacyProjectFolderName
	}
	return ProjectFolderNameDefault
}

// ProjectFolder returns dir joined with the resolved project folder name. It
// is the single place that decides between .qbit-jay and .engram.
func ProjectFolder(dir string) string {
	return filepath.Join(dir, ProjectFolderName(dir))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
