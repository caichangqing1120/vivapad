package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vivapad/internal/foldertree"
)

func (state *model) ensureFolderWatch(path string) {
	if !state.folderVisible || state.folderWatchUnavailable || state.folderWatching[path] != nil {
		return
	}
	info, err := folderWatchIdentity(path)
	if err != nil {
		state.notice = "Folder unavailable: " + err.Error()
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
	state.folderWatching[path] = info
}

func folderWatchIdentity(path string) (os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// On Windows a path-based stat can defer looking up the file ID until
	// SameFile is called, by which time the name can refer to another directory.
	info, err := file.Stat()
	if err == nil && !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}
	return info, err
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
	clear(state.folderCache)
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
	active := make(map[string]struct{}, len(state.folderWatching))
	for _, path := range state.folderWatcher.WatchList() {
		active[path] = struct{}{}
	}
	reset := false
	for path, previous := range state.folderWatching {
		err := foldertree.CheckCanonicalPath(path)
		var current os.FileInfo
		if err == nil {
			current, err = folderWatchIdentity(path)
		}
		if err != nil {
			delete(state.folderExpanded, path)
			state.notice = "Folder unavailable: " + err.Error()
			reset = true
		} else if !os.SameFile(previous, current) {
			// A backend may keep the old handle and report the original name.
			// Closing the watcher releases handles that Remove(path) cannot find.
			reset = true
		} else if _, watched := active[path]; !watched {
			// The OS dropped the old directory watch. Paint can attach to its replacement.
			delete(state.folderWatching, path)
		}
	}
	if reset {
		state.stopFolderWatching()
	}
}
