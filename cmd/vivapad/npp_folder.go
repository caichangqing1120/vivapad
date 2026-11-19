package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vivapad/internal/foldertree"
	"vivapad/internal/session"
)

func (state *model) exportNPPFolders(result *session.NPPState) error {
	if !state.folderVisible {
		return nil
	}
	result.FileBrowserIncluded = true
	for _, path := range state.folderRoots {
		root, err := foldertree.CanonicalRoot(path)
		if err != nil {
			return fmt.Errorf("folder unavailable for session export: %w", err)
		}
		if root != path {
			return fmt.Errorf("folder unavailable for session export: path now points elsewhere: %s", path)
		}
		folder := session.NPPFolderRoot{Path: root}
		for expanded, enabled := range state.folderExpanded {
			if !enabled {
				continue
			}
			relative, err := filepath.Rel(root, expanded)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
			canonical, err := foldertree.CanonicalRoot(expanded)
			if err != nil {
				return fmt.Errorf("expanded folder unavailable for session export: %w", err)
			}
			if canonical != expanded || !foldertree.Contains(root, canonical) {
				return fmt.Errorf("expanded folder is outside or hidden from root: %s", expanded)
			}
			folder.Expanded = append(folder.Expanded, canonical)
		}
		sort.Strings(folder.Expanded)
		result.FolderRoots = append(result.FolderRoots, folder)
	}
	if state.folderSelected != "" {
		selected, err := filepath.EvalSymlinks(state.folderSelected)
		if err != nil {
			return fmt.Errorf("selected folder item unavailable for session export: %w", err)
		}
		for _, root := range result.FolderRoots {
			if foldertree.Contains(root.Path, selected) {
				result.FolderSelected = selected
				break
			}
		}
		if result.FolderSelected == "" {
			return fmt.Errorf("selected folder item is outside workspace roots")
		}
	}
	return nil
}

func (state *model) prepareNPPFolders(stored session.NPPState) ([]session.NPPFolderRoot, string, error) {
	if !stored.FileBrowserIncluded {
		return nil, "", nil
	}
	known := append([]string(nil), state.folderRoots...)
	prepared := make([]session.NPPFolderRoot, 0, len(stored.FolderRoots))
	for _, item := range stored.FolderRoots {
		root, err := foldertree.CanonicalRoot(item.Path)
		if err != nil {
			return nil, "", fmt.Errorf("folder unavailable: %w", err)
		}
		covered := false
		for _, existing := range known {
			if foldertree.Contains(existing, root) {
				covered = true
				break
			}
			if foldertree.Contains(root, existing) {
				return nil, "", fmt.Errorf("folder root conflicts with %s", existing)
			}
		}
		if !covered {
			if len(known) >= 64 {
				return nil, "", fmt.Errorf("too many folder roots")
			}
			known = append(known, root)
		}
		folder := session.NPPFolderRoot{Path: root}
		for _, path := range item.Expanded {
			expanded, err := foldertree.CanonicalRoot(path)
			if err != nil {
				return nil, "", fmt.Errorf("expanded folder unavailable: %w", err)
			}
			if !foldertree.Contains(root, expanded) || expanded != path {
				return nil, "", fmt.Errorf("expanded folder is outside or hidden from root: %s", path)
			}
			folder.Expanded = append(folder.Expanded, expanded)
		}
		prepared = append(prepared, folder)
	}
	selected := ""
	if stored.FolderSelected != "" {
		var err error
		selected, err = filepath.EvalSymlinks(stored.FolderSelected)
		if err != nil {
			return nil, "", fmt.Errorf("selected folder item unavailable: %w", err)
		}
		info, err := os.Stat(selected)
		if err != nil || !info.IsDir() && !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("selected folder item is not a file or directory: %s", selected)
		}
		inside := false
		for _, root := range prepared {
			inside = inside || foldertree.Contains(root.Path, selected)
		}
		if !inside {
			return nil, "", fmt.Errorf("selected folder item resolves outside workspace roots: %s", selected)
		}
	}
	return prepared, selected, nil
}

func (state *model) applyNPPFolders(folders []session.NPPFolderRoot, selected string) error {
	for _, folder := range folders {
		if err := state.addFolderRoot(folder.Path); err != nil {
			return err
		}
		for _, path := range folder.Expanded {
			state.folderExpanded[path] = true
		}
	}
	if selected != "" {
		state.folderSelected = selected
		for _, folder := range folders {
			if !foldertree.Contains(folder.Path, selected) {
				continue
			}
			parent := selected
			if info, err := os.Stat(selected); err == nil && !info.IsDir() {
				parent = filepath.Dir(selected)
			}
			for current := parent; foldertree.Contains(folder.Path, current); current = filepath.Dir(current) {
				state.folderExpanded[current] = true
				if current == folder.Path {
					break
				}
			}
			break
		}
	}
	if len(folders) > 0 {
		state.folderVisible = true
	}
	return nil
}
