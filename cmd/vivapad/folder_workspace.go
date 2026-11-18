package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"vivapad/internal/foldertree"
	"vivapad/internal/session"
)

type folderRow struct {
	path      string
	name      string
	depth     int
	directory bool
}

func (state *model) folderPanelWidth(width float32) float32 {
	if !state.folderVisible && !state.functionListVisible && !state.documentListVisible {
		return 0
	}
	return max(0, min(250, max(140, width*0.27), width-180))
}

func (state *model) addFolderRoot(path string) error {
	root, err := foldertree.CanonicalRoot(path)
	if err != nil {
		return err
	}
	for _, existing := range state.folderRoots {
		if foldertree.Contains(existing, root) {
			current := existing
			relative, _ := filepath.Rel(existing, root)
			for _, part := range strings.Split(relative, string(filepath.Separator)) {
				state.folderExpanded[current] = true
				if part != "." {
					current = filepath.Join(current, part)
				}
			}
			state.folderSelected, state.folderVisible = root, true
			return nil
		}
		if foldertree.Contains(root, existing) {
			return fmt.Errorf("remove the nested root before adding %s", root)
		}
	}
	if len(state.folderRoots) >= 64 {
		return fmt.Errorf("too many folder roots")
	}
	state.folderRoots = append(state.folderRoots, root)
	state.folderSelected, state.folderVisible = root, true
	return nil
}

func (state *model) chooseFolderRoot() {
	defer state.requestNativeChooserFocus()
	path, err := app.OpenDirectory(app.OpenDirectoryOptions{Title: "Open Folder as Workspace"})
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err == nil && path == "" {
		return
	}
	if err == nil {
		err = state.addFolderRoot(path)
	}
	if err != nil {
		state.notice = "Folder unavailable: " + err.Error()
	}
}

func (state *model) addCurrentDocumentFolder() {
	path := state.space.Current().Path
	if path == "" {
		state.notice = "Save the document before adding its folder"
		return
	}
	if err := state.addFolderRoot(filepath.Dir(path)); err != nil {
		state.notice = "Folder unavailable: " + err.Error()
	}
}

func (state *model) restoreFolderWorkspace(stored session.State) {
	notice := state.notice
	var missing string
	for _, root := range stored.FolderRoots {
		if err := state.addFolderRoot(root); err != nil {
			missing = "Folder unavailable: " + err.Error()
		}
	}
	state.folderVisible = stored.FolderVisible
	state.notice = notice
	if missing != "" {
		state.notice = missing
	}
}

func (state *model) folderRows() []folderRow {
	var rows []folderRow
	var appendChildren func(string, int)
	appendChildren = func(path string, depth int) {
		if !state.folderExpanded[path] {
			return
		}
		state.ensureFolderWatch(path)
		children, ok := state.folderCache[path]
		if !ok {
			var err error
			children, err = foldertree.Children(path)
			if err != nil {
				state.notice = "Folder unavailable: " + err.Error()
				return
			}
			state.folderCache[path] = children
		}
		for _, child := range children {
			rows = append(rows, folderRow{path: child.Path, name: child.Name, depth: depth, directory: child.Directory})
			if child.Directory {
				appendChildren(child.Path, depth+1)
			}
		}
	}
	for _, root := range state.folderRoots {
		rows = append(rows, folderRow{path: root, name: filepath.Base(root), directory: true})
		appendChildren(root, 1)
	}
	return rows
}

func (state *model) activateFolderRow(row folderRow) {
	state.folderSelected = row.path
	if row.directory {
		state.folderExpanded[row.path] = !state.folderExpanded[row.path]
		if !state.folderExpanded[row.path] {
			state.stopFolderBranch(row.path)
		}
		return
	}
	if err := state.space.Open(row.path); err != nil {
		state.notice = "Cannot open " + row.name + ": " + err.Error()
		return
	}
	state.showInActivePane(state.space.Current())
	state.rememberRecent(state.space.Current())
}

func (state *model) refreshFolderTree() {
	clear(state.folderCache)
	state.folderWatchUnavailable = false
	state.notice = "Folder tree refreshed"
}
