package recovery

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

const (
	version  = 9
	maxBytes = 32 << 20
)

type Tab struct {
	Path             string              `json:"path"`
	Name             string              `json:"name"`
	Content          string              `json:"content"`
	Encoding         document.Encoding   `json:"encoding"`
	Language         string              `json:"language,omitempty"`
	LanguageSelected bool                `json:"language_selected,omitempty"`
	FoldedHeaders    []int               `json:"folded_headers,omitempty"`
	DiskEncoding     document.Encoding   `json:"disk_encoding,omitempty"`
	LineEnding       document.LineEnding `json:"line_ending"`
	LineEndings      string              `json:"line_endings,omitempty"`
	DiskDigest       string              `json:"disk_digest"`
	SelectionStart   int                 `json:"selection_start"`
	SelectionEnd     int                 `json:"selection_end"`
	Pane             int                 `json:"pane"`
	ReadOnly         bool                `json:"read_only,omitempty"`
	Pinned           bool                `json:"pinned,omitempty"`
	TabColor         uint8               `json:"tab_color,omitempty"`
}

type State struct {
	Tabs       []Tab   `json:"tabs"`
	Active     int     `json:"active"`
	Split      bool    `json:"split"`
	ActivePane int     `json:"active_pane"`
	Divider    float32 `json:"divider"`
}

type diskState struct {
	Version int `json:"version"`
	State
}

func Save(path string, state State) error {
	if err := validate(state); err != nil {
		return err
	}
	if !encodedContentsFit(state.Tabs, maxBytes) {
		return fmt.Errorf("recovery backup exceeds 32 MiB")
	}
	data, err := json.Marshal(diskState{Version: version, State: state})
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return fmt.Errorf("recovery backup exceeds 32 MiB")
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
		return fmt.Errorf("recovery directory is not a regular directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-recovery-*")
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

// Check the JSON string sizes before encoding; Marshal still checks the full
// backup, including metadata. validate has already checked UTF-8 and raw sizes.
func encodedContentsFit(tabs []Tab, limit int) bool {
	size := 0
	for _, tab := range tabs {
		size += len(tab.Content) + 2
	}
	if size <= limit/6 {
		return true
	}
	if size > limit {
		return false
	}
	for _, tab := range tabs {
		for _, r := range tab.Content {
			switch r {
			case '"', '\\', '\b', '\f', '\n', '\r', '\t':
				size++
			case '<', '>', '&':
				size += 5
			case '\u2028', '\u2029':
				size += 3
			default:
				if r < ' ' {
					size += 5
				}
			}
			if size > limit {
				return false
			}
		}
	}
	return true
}

func Load(path string) (State, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Active: -1}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return State{}, err
	}
	if info.Size() > maxBytes {
		return State{}, fmt.Errorf("recovery backup exceeds 32 MiB")
	}
	return decodeRecovery(file, maxBytes)
}

func decodeRecovery(source io.Reader, limit int64) (State, error) {
	// Read one extra byte so the limit cannot masquerade as the file's EOF.
	limited := &io.LimitedReader{R: source, N: limit + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var stored diskState
	err := decoder.Decode(&stored)
	if err == nil {
		var extra json.RawMessage
		if trailing := decoder.Decode(&extra); !errors.Is(trailing, io.EOF) {
			err = fmt.Errorf("recovery backup has trailing data")
		}
	}
	if limited.N == 0 {
		return State{}, fmt.Errorf("recovery backup exceeds 32 MiB")
	}
	if err != nil {
		return State{}, err
	}
	if stored.Version < 1 || stored.Version > version {
		return State{}, fmt.Errorf("unsupported recovery version %d", stored.Version)
	}
	if stored.Version < 3 {
		for index := range stored.Tabs {
			stored.Tabs[index].DiskEncoding = stored.Tabs[index].Encoding
		}
	}
	if stored.Version < 5 {
		for index := range stored.Tabs {
			stored.Tabs[index].Pinned = false
		}
	}
	if stored.Version < 7 {
		for index := range stored.Tabs {
			stored.Tabs[index].Language, stored.Tabs[index].LanguageSelected = "", false
		}
	}
	if stored.Version < 8 {
		for index := range stored.Tabs {
			stored.Tabs[index].FoldedHeaders = nil
		}
	}
	if stored.Version < 9 {
		for index := range stored.Tabs {
			stored.Tabs[index].TabColor = 0
		}
	}
	if err := validate(stored.State); err != nil {
		return State{}, err
	}
	return stored.State, nil
}

func Clear(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func validate(state State) error {
	if state.Split {
		if state.ActivePane < 0 || state.ActivePane > 1 || state.Divider <= 0 || state.Divider >= 1 {
			return fmt.Errorf("invalid recovery split view")
		}
	} else if state.ActivePane != 0 || state.Divider != 0 {
		return fmt.Errorf("inactive recovery has split view state")
	}
	if len(state.Tabs) == 0 {
		if state.Active != -1 {
			return fmt.Errorf("empty recovery has an active tab")
		}
	} else if state.Active < 0 || state.Active >= len(state.Tabs) {
		return fmt.Errorf("recovery active tab is out of range")
	}
	total := 0
	for _, tab := range state.Tabs {
		if tab.TabColor > 5 {
			return fmt.Errorf("recovery tab has invalid color %d", tab.TabColor)
		}
		if tab.Pane < 0 || tab.Pane > 1 || !state.Split && tab.Pane != 0 {
			return fmt.Errorf("invalid recovery pane assignment")
		}
		if tab.Name == "" || strings.IndexByte(tab.Name, 0) >= 0 || !utf8.ValidString(tab.Content) {
			return fmt.Errorf("recovery tab has invalid text")
		}
		if tab.Language != "" && !tab.LanguageSelected {
			return fmt.Errorf("recovery tab has unselected language")
		}
		if tab.LanguageSelected {
			if _, ok := syntax.LanguageByMode(tab.Language); !ok {
				return fmt.Errorf("recovery tab has unsupported language %q", tab.Language)
			}
		}
		if len(tab.FoldedHeaders) > 4096 {
			return fmt.Errorf("recovery tab has too many folds")
		}
		previous, lines := -1, strings.Count(tab.Content, "\n")+1
		for _, line := range tab.FoldedHeaders {
			if line <= previous || line >= lines {
				return fmt.Errorf("recovery tab has invalid fold header %d", line)
			}
			previous = line
		}
		if tab.Path != "" {
			if !filepath.IsAbs(tab.Path) || strings.IndexByte(tab.Path, 0) >= 0 || len(tab.DiskDigest) != 64 {
				return fmt.Errorf("recovery tab has an invalid path or digest")
			}
			if _, err := hex.DecodeString(tab.DiskDigest); err != nil {
				return fmt.Errorf("recovery digest is not hexadecimal: %w", err)
			}
		} else if tab.DiskDigest != "" {
			return fmt.Errorf("untitled recovery tab has a disk digest")
		}
		if !tab.Encoding.Valid() || !tab.DiskEncoding.Valid() || tab.LineEnding > document.CR || tab.SelectionStart < 0 || tab.SelectionEnd < 0 ||
			tab.SelectionStart > utf8.RuneCountInString(tab.Content) || tab.SelectionEnd > utf8.RuneCountInString(tab.Content) {
			return fmt.Errorf("recovery tab has invalid editor state")
		}
		if tab.LineEndings != "" {
			if len(tab.LineEndings) != strings.Count(tab.Content, "\n") || strings.IndexFunc(tab.LineEndings, func(code rune) bool { return code != 'W' && code != 'L' && code != 'M' }) >= 0 {
				return fmt.Errorf("recovery tab has invalid EOL sequence")
			}
		}
		total += len(tab.Content) + len(tab.Path) + len(tab.Name) + len(tab.LineEndings)
		if total > maxBytes {
			return fmt.Errorf("recovery backup exceeds 32 MiB")
		}
	}
	return nil
}
