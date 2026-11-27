package main

import (
	"os"

	"vivapad/internal/document"
)

func destinationMatchesFile(path string, target os.FileInfo, existing string) bool {
	if path == existing {
		return true
	}
	if target == nil {
		return false
	}
	info, err := os.Stat(existing)
	return err == nil && os.SameFile(target, info)
}

func (state *model) isPrivateDataPath(path string) bool {
	target, _ := os.Stat(path)
	for _, internal := range []string{
		state.sessionPath, state.preferencesPath, state.recoveryPath, state.recentPath,
		state.closedPath, state.macroPath, state.namedMacroPath, state.runPath,
		state.udlPath, state.projectsPath,
	} {
		if internal == "" {
			continue
		}
		if reserved, err := document.CanonicalSavePath(internal); err == nil && destinationMatchesFile(path, target, reserved) {
			return true
		}
	}
	return false
}
