package preferences

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vivapad/internal/datetime"
	"vivapad/internal/printing"
	"vivapad/internal/privatejson"
	"vivapad/internal/websearch"
)

const (
	maxSize = 64 << 10
	version = 9
)

type Theme string

const (
	ClassicLight Theme = "classic-light"
	ModernDark   Theme = "modern-dark"
	Monokai      Theme = "monokai"
	FollowSystem Theme = "follow-system"
)

func (theme Theme) Normalized() Theme {
	switch theme {
	case ModernDark, Monokai, FollowSystem:
		return theme
	default:
		return ClassicLight
	}
}

type State struct {
	Styles               Styles `json:"styles,omitzero"`
	Theme                Theme  `json:"theme,omitempty"`
	FontSize             int    `json:"font_size"`
	SubFontSize          int    `json:"sub_font_size"`
	ZoomSync             bool   `json:"zoom_sync,omitempty"`
	TabWidth             int    `json:"tab_width"`
	Wrap                 bool   `json:"wrap"`
	LineNumbers          bool   `json:"line_numbers"`
	AutoIndent           bool   `json:"auto_indent"`
	AutoClose            bool   `json:"auto_close"`
	IndentMode           string `json:"indent_mode"`
	DateTimeFormat       string `json:"date_time_format,omitempty"`
	DateTimeReverseOrder bool   `json:"date_time_reverse_order,omitempty"`
	ShowWhitespace       bool   `json:"show_whitespace,omitempty"`
	ShowEOL              bool   `json:"show_eol,omitempty"`
	ShowIndentGuides     bool   `json:"show_indent_guides,omitempty"`
	ShowWrapSymbol       bool   `json:"show_wrap_symbol,omitempty"`
	// ChangeHistoryView selects how the change history is drawn: "margin"
	// (default lane), "indicator" (in-text marks only), "both", or "off".
	ChangeHistoryView  string            `json:"change_history_view,omitempty"`
	ShowNonPrinting    bool              `json:"show_non_printing,omitempty"`
	ShowControls       bool              `json:"show_controls,omitempty"`
	NPCMode            string            `json:"npc_mode,omitempty"`
	NPCIncludeControls bool              `json:"npc_include_controls,omitempty"`
	Print              printing.Settings `json:"print_settings"`
	WebSearch          websearch.Config  `json:"web_search,omitzero"`
}

type diskState struct {
	Version int `json:"version"`
	State
}

func Default() State {
	return State{Theme: ClassicLight, FontSize: 13, SubFontSize: 13, TabWidth: 4, LineNumbers: true, AutoIndent: true, AutoClose: true, IndentMode: "language", DateTimeFormat: datetime.DefaultFormat, ShowIndentGuides: true, ShowControls: true, NPCMode: "abbreviation", ChangeHistoryView: "margin", Print: printing.DefaultSettings()}
}

func (state State) validate() error {
	for _, theme := range []Theme{ClassicLight, ModernDark, Monokai} {
		if err := state.Styles.For(theme).Validate(); err != nil {
			return err
		}
	}
	if state.FontSize < 9 || state.FontSize > 24 || state.SubFontSize < 9 || state.SubFontSize > 24 || state.TabWidth < 1 || state.TabWidth > 16 {
		return fmt.Errorf("invalid editor size or tab width")
	}
	if state.ZoomSync && state.FontSize != state.SubFontSize {
		return fmt.Errorf("synchronized zoom sizes differ")
	}
	if state.IndentMode != "language" && state.IndentMode != "tabs" && state.IndentMode != "spaces" {
		return fmt.Errorf("invalid indent mode %q", state.IndentMode)
	}
	if state.NPCMode != "identity" && state.NPCMode != "abbreviation" && state.NPCMode != "codepoint" {
		return fmt.Errorf("invalid non-printing representation mode %q", state.NPCMode)
	}
	switch state.ChangeHistoryView {
	case "", "margin", "indicator", "both", "off":
	default:
		return fmt.Errorf("invalid change history view %q", state.ChangeHistoryView)
	}
	if err := datetime.ValidatePattern(state.DateTimeFormat); err != nil {
		return err
	}
	if err := state.Print.Validate(); err != nil {
		return err
	}
	return state.WebSearch.Validate()
}

func Load(path string) (State, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return State{}, err
	}
	if info.Size() > maxSize {
		return State{}, fmt.Errorf("settings exceed 64 KiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxSize))
	decoder.DisallowUnknownFields()
	var stored diskState
	if err := decoder.Decode(&stored); err != nil {
		return State{}, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return State{}, fmt.Errorf("settings have trailing data")
	}
	if stored.Version < 1 || stored.Version > version {
		return State{}, fmt.Errorf("unsupported settings version %d", stored.Version)
	}
	if stored.Version == 1 {
		stored.State.SubFontSize = stored.State.FontSize
		stored.State.ZoomSync = false
	}
	if stored.Version < 3 {
		stored.State.Print = printing.DefaultSettings()
	}
	if stored.Version < 4 {
		stored.State.ShowIndentGuides = true
	}
	if stored.Version < 5 {
		stored.State.ShowControls = true
	}
	if stored.Version < 6 {
		// Older files had no setting: keep the margin on, as it was before.
		stored.State.ChangeHistoryView = "margin"
	}
	if stored.State.DateTimeFormat == "" {
		stored.State.DateTimeFormat = datetime.DefaultFormat
	}
	if stored.State.NPCMode == "" {
		stored.State.NPCMode = "abbreviation"
	}
	stored.State.Theme = stored.State.Theme.Normalized()
	if err := stored.State.validate(); err != nil {
		return State{}, err
	}
	return stored.State, nil
}

func Save(path string, state State) error {
	state.Theme = state.Theme.Normalized()
	if err := state.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(diskState{Version: version, State: state})
	if err != nil {
		return err
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
		return fmt.Errorf("settings directory is not a regular directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".vivapad-settings-*")
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
	return privatejson.Replace(file.Name(), path)
}
