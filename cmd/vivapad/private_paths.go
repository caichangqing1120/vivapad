package main

import "vivapad/internal/document"

func (state *model) isPrivateDataPath(path string) bool {
	for _, internal := range []string{state.sessionPath, state.preferencesPath, state.recoveryPath, state.recentPath, state.closedPath, state.macroPath} {
		if internal == "" {
			continue
		}
		if reserved, err := document.CanonicalSavePath(internal); err == nil && reserved == path {
			return true
		}
	}
	return false
}
