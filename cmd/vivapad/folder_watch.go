package main

import (
	"path/filepath"
	"strings"

	"vivapad/internal/foldertree"
)

func (state *model) ensureFolderWatch(path string) {
	if !state.folderVisible || state.folderWatchUnavailable || state.folderWatching[path] {
		return
	}
	if state.folderWatcher == nil {
		watcher, err := foldertree.NewWatcher(state.invalidator.Invalidate)
		if err != nil {
			state.folderWatchUnavailable = true
			state.notice = "Folder updates unavailable: " + err.Error()
			return
		}
		state.folderWatcher = watcher
	}
	if err := state.folderWatcher.Add(path); err != nil {
		state.folderWatchUnavailable = true
		state.notice = "Folder updates unavailable: " + err.Error()
		state.stopFolderWatching()
		return
	}
	state.folderWatching[path] = true
}

func (state *model) stopFolderBranch(path string) {
	for watched := range state.folderWatching {
		relative, err := filepath.Rel(path, watched)
		if err != nil || relative != "." && (relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			continue
		}
		_ = state.folderWatcher.Remove(watched)
		delete(state.folderWatching, watched)
	}
	for cached := range state.folderCache {
		relative, err := filepath.Rel(path, cached)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			delete(state.folderCache, cached)
		}
	}
	if len(state.folderWatching) == 0 {
		state.stopFolderWatching()
	}
}

func (state *model) stopFolderWatching() {
	if state.folderWatcher != nil {
		_ = state.folderWatcher.Close()
		state.folderWatcher = nil
	}
	clear(state.folderWatching)
}

func (state *model) consumeFolderChanges() {
	if state.folderWatcher == nil {
		return
	}
	changed, err := state.folderWatcher.Consume()
	if err != nil {
		state.folderWatchUnavailable = true
		state.notice = "Folder updates unavailable: " + err.Error()
		state.stopFolderWatching()
		return
	}
	if !changed {
		return
	}
	clear(state.folderCache)
	for path := range state.folderWatching {
		if _, err := foldertree.CanonicalRoot(path); err != nil {
			_ = state.folderWatcher.Remove(path)
			delete(state.folderWatching, path)
			delete(state.folderExpanded, path)
		}
	}
}
