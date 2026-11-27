package main

import (
	"bytes"
	"debug/buildinfo"
	"fmt"
	"runtime"
)

func validateArtifactCoreBuild(core []byte, revision string) error {
	info, err := buildinfo.Read(bytes.NewReader(core))
	if err != nil {
		return err
	}
	if info.Path != "github.com/qbitdevz/qbit-jay/cmd/qbit-jay" || info.Main.Path != "github.com/qbitdevz/qbit-jay" {
		return fmt.Errorf("staging: Core module identity mismatch")
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	for k, v := range map[string]string{"vcs": "git", "vcs.revision": revision, "vcs.modified": "false", "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "CGO_ENABLED": "0"} {
		if settings[k] != v {
			return fmt.Errorf("staging: Core build metadata mismatch: %s", k)
		}
	}
	return nil
}
