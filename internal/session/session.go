package session

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

const version = 13
const maxJSONSessionBytes = 1 << 20

type FoldState struct {
	Digest  string `json:"digest"`
	Headers []int  `json:"headers"`
}

type State struct {
	Paths               []string
	Active              string
	View                ViewState
	Encodings           map[string]string
	Languages           map[string]string
	Folds               map[string]FoldState
	ReadOnly            map[string]bool
	Pinned              map[string]bool
	RTL                 map[string]bool
	TabColors           map[string]uint8
	FolderRoots         []string
	FolderVisible       bool
	DocumentMapVisible  bool
	FunctionListVisible bool
	DocumentListVisible bool
	ActiveLeftDock      string
}

type ViewState struct {
	Split      bool     `json:"split"`
	Left       string   `json:"left"`
	Right      string   `json:"right"`
	RightPaths []string `json:"right_paths"`
	ActivePane int      `json:"active_pane"`
	Divider    float32  `json:"divider"`
}

type diskState struct {
	Version             int                  `json:"version"`
	Paths               []string             `json:"paths"`
	Active              string               `json:"active"`
	View                ViewState            `json:"view"`
	Encodings           map[string]string    `json:"encodings,omitempty"`
	Languages           map[string]string    `json:"languages,omitempty"`
	Folds               map[string]FoldState `json:"folds,omitempty"`
	ReadOnly            map[string]bool      `json:"read_only,omitempty"`
	Pinned              map[string]bool      `json:"pinned,omitempty"`
	RTL                 map[string]bool      `json:"rtl,omitempty"`
	TabColors           map[string]uint8     `json:"tab_colors,omitempty"`
	FolderRoots         []string             `json:"folder_roots,omitempty"`
	FolderVisible       bool                 `json:"folder_visible,omitempty"`
	DocumentMapVisible  bool                 `json:"document_map_visible,omitempty"`
	FunctionListVisible bool                 `json:"function_list_visible,omitempty"`
	DocumentListVisible bool                 `json:"document_list_visible,omitempty"`
	ActiveLeftDock      string               `json:"active_left_dock,omitempty"`
}

func Save(path string, state State) error {
	if err := validate(state); err != nil {
		return err
	}
	data, err := json.Marshal(diskState{Version: version, Paths: state.Paths, Active: state.Active, View: state.View, Encodings: state.Encodings, Languages: state.Languages, Folds: state.Folds, ReadOnly: state.ReadOnly, Pinned: state.Pinned, RTL: state.RTL, TabColors: state.TabColors, FolderRoots: state.FolderRoots, FolderVisible: state.FolderVisible, DocumentMapVisible: state.DocumentMapVisible, FunctionListVisible: state.FunctionListVisible, DocumentListVisible: state.DocumentListVisible, ActiveLeftDock: state.ActiveLeftDock})
	if err != nil {
		return err
	}
	if len(data) > maxJSONSessionBytes {
		return fmt.Errorf("session exceeds 1 MiB")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("session directory is not a regular directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-session-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func Load(path string) (State, error) {
	file, size, err := openRegularSession(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	if size > maxJSONSessionBytes {
		return State{}, fmt.Errorf("session exceeds 1 MiB")
	}
	return decodeSession(file)
}

func decodeSession(source io.Reader) (State, error) {
	// Read one extra byte so reaching the limit cannot look like EOF.
	limited := &io.LimitedReader{R: source, N: maxJSONSessionBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var stored diskState
	err := decoder.Decode(&stored)
	if err == nil {
		var extra json.RawMessage
		if trailing := decoder.Decode(&extra); !errors.Is(trailing, io.EOF) {
			err = fmt.Errorf("session has trailing data")
		}
	}
	if limited.N == 0 {
		return State{}, fmt.Errorf("session exceeds 1 MiB")
	}
	if err != nil {
		return State{}, err
	}
	if stored.Version < 1 || stored.Version > version {
		return State{}, fmt.Errorf("unsupported session version %d", stored.Version)
	}
	state := State{Paths: stored.Paths, Active: stored.Active, View: stored.View, Encodings: stored.Encodings, Languages: stored.Languages, Folds: stored.Folds, ReadOnly: stored.ReadOnly, Pinned: stored.Pinned, RTL: stored.RTL, TabColors: stored.TabColors, FolderRoots: stored.FolderRoots, FolderVisible: stored.FolderVisible, DocumentMapVisible: stored.DocumentMapVisible, FunctionListVisible: stored.FunctionListVisible, DocumentListVisible: stored.DocumentListVisible, ActiveLeftDock: stored.ActiveLeftDock}
	if stored.Version == 1 {
		state.View = ViewState{}
	}
	if stored.Version < 3 {
		state.Encodings = nil
	}
	if stored.Version < 4 {
		state.ReadOnly = nil
	}
	if stored.Version < 5 {
		state.Pinned = nil
	}
	if stored.Version < 6 {
		state.FolderRoots, state.FolderVisible = nil, false
	}
	if stored.Version < 7 {
		state.Languages = nil
	}
	if stored.Version < 8 {
		state.Folds = nil
	}
	if stored.Version < 9 {
		state.DocumentMapVisible = false
	}
	if stored.Version < 10 {
		state.FunctionListVisible, state.ActiveLeftDock = false, ""
	}
	if stored.Version < 11 {
		state.DocumentListVisible = false
	}
	if stored.Version < 12 {
		state.TabColors = nil
	}
	if stored.Version < 13 {
		state.RTL = nil
	}
	if err := validate(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func validate(state State) error {
	if state.ActiveLeftDock != "" && state.ActiveLeftDock != "workspace" && state.ActiveLeftDock != "functions" && state.ActiveLeftDock != "documents" ||
		state.ActiveLeftDock == "workspace" && !state.FolderVisible || state.ActiveLeftDock == "functions" && !state.FunctionListVisible || state.ActiveLeftDock == "documents" && !state.DocumentListVisible {
		return fmt.Errorf("invalid active left dock")
	}
	activeFound := state.Active == ""
	paths := make(map[string]bool, len(state.Paths))
	for _, path := range state.Paths {
		if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
			return fmt.Errorf("session path is not absolute text: %q", path)
		}
		activeFound = activeFound || state.Active == path
		paths[path] = true
	}
	if !activeFound {
		return fmt.Errorf("active session path is not open")
	}
	for path, encoding := range state.Encodings {
		if _, ok := document.EncodingFromLabel(encoding); !paths[path] || !ok {
			return fmt.Errorf("invalid session encoding for %s", path)
		}
	}
	for path, mode := range state.Languages {
		if !paths[path] {
			return fmt.Errorf("invalid session language path %s", path)
		}
		if _, ok := syntax.LanguageByMode(mode); !ok {
			return fmt.Errorf("unsupported session language %q", mode)
		}
	}
	for path, fold := range state.Folds {
		if !paths[path] || len(fold.Digest) != 64 || len(fold.Headers) == 0 || len(fold.Headers) > 4096 {
			return fmt.Errorf("invalid session fold state for %s", path)
		}
		if _, err := hex.DecodeString(fold.Digest); err != nil {
			return fmt.Errorf("invalid session fold digest: %w", err)
		}
		previous := -1
		for _, line := range fold.Headers {
			if line <= previous {
				return fmt.Errorf("invalid session fold header %d", line)
			}
			previous = line
		}
	}
	for path, enabled := range state.ReadOnly {
		if !paths[path] || !enabled {
			return fmt.Errorf("invalid session read-only state for %s", path)
		}
	}
	for path, enabled := range state.Pinned {
		if !paths[path] || !enabled {
			return fmt.Errorf("invalid session pinned state for %s", path)
		}
	}
	for path, color := range state.TabColors {
		if !paths[path] || color < 1 || color > 5 {
			return fmt.Errorf("invalid session tab color for %s", path)
		}
	}
	for path, enabled := range state.RTL {
		if !paths[path] || !enabled {
			return fmt.Errorf("invalid session RTL state for %s", path)
		}
	}
	if len(state.FolderRoots) > 64 {
		return fmt.Errorf("too many folder roots")
	}
	folders := make(map[string]bool, len(state.FolderRoots))
	for _, folder := range state.FolderRoots {
		if !filepath.IsAbs(folder) || strings.IndexByte(folder, 0) >= 0 || folders[folder] {
			return fmt.Errorf("invalid folder root %q", folder)
		}
		folders[folder] = true
	}
	view := state.View
	if !view.Split {
		if view.Left != "" || view.Right != "" || len(view.RightPaths) != 0 || view.ActivePane != 0 || view.Divider != 0 {
			return fmt.Errorf("inactive view has split state")
		}
		return nil
	}
	if view.ActivePane < 0 || view.ActivePane > 1 || view.Divider <= 0 || view.Divider >= 1 {
		return fmt.Errorf("invalid split view position")
	}
	for _, path := range append(append([]string{view.Left, view.Right}, view.RightPaths...), "") {
		if path != "" && !paths[path] {
			return fmt.Errorf("split view path is not open: %s", path)
		}
	}
	return nil
}
