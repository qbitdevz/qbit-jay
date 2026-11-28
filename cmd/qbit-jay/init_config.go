package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qbitdevz/qbit-jay/internal/qbit/paths"
)

const initConfigName = "config.json"

// initConfigLabel names a config file as <project folder>/config.json for messages.
func initConfigLabel(path string) string {
	return filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
}

// writeInitConfig creates or replaces the cwd-local project configuration.
func writeInitConfig(cwd string, data []byte, force bool) error {
	// New projects get .qbit-jay/; a repository that only has the upstream
	// .engram/ keeps using it (spec D1).
	directory := paths.ProjectFolder(cwd)
	if err := ensureInitConfigDirectory(directory); err != nil {
		return err
	}

	configPath := filepath.Join(directory, initConfigName)
	if err := rejectInitConfigSymlink(configPath); err != nil {
		return err
	}
	if force {
		return replaceInitConfig(configPath, data)
	}
	return createInitConfig(configPath, data)
}

func ensureInitConfigDirectory(path string) error {
	if err := os.Mkdir(path, 0o755); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create %s directory: %w", filepath.Base(path), err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s directory: %w", filepath.Base(path), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must be a real directory, not a symlink", filepath.Base(path))
	}
	if !info.IsDir() {
		return fmt.Errorf("%s must be a directory", filepath.Base(path))
	}
	return nil
}

func rejectInitConfigSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", initConfigLabel(path), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", initConfigLabel(path))
	}
	return nil
}

func createInitConfig(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s already exists (use --force to overwrite)", initConfigLabel(path))
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", initConfigLabel(path), err)
	}
	if err := writeAndCloseInitConfig(file, data); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("write %s: %w (remove partial config: %v)", initConfigLabel(path), err, removeErr)
		}
		return fmt.Errorf("write %s: %w", initConfigLabel(path), err)
	}
	return nil
}

func replaceInitConfig(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config.json-*")
	if err != nil {
		return fmt.Errorf("create replacement config: %w", err)
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set replacement config permissions: %w", err)
	}
	if err := writeAndCloseInitConfig(temporary, data); err != nil {
		return fmt.Errorf("write replacement config: %w", err)
	}
	if err := replaceInitConfigFile(temporaryPath, path); err != nil {
		return fmt.Errorf("publish replacement config: %w", err)
	}
	published = true
	return nil
}

func writeAndCloseInitConfig(file *os.File, data []byte) error {
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = fmt.Errorf("short write: wrote %d of %d bytes", written, len(data))
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
