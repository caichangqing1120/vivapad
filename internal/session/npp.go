package session

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vivapad/internal/document"
)

const maxNPPBytes = 1 << 20

type NPPFile struct {
	Path     string
	Encoding int
	Language string
	StartPos int64
	EndPos   int64
	Marks    []int
	Folds    []int
	ReadOnly bool
	Pinned   bool
	Color    uint8
}

type NPPState struct {
	Main                []NPPFile
	Sub                 []NPPFile
	ActiveView          int
	ActiveMain          int
	ActiveSub           int
	FileBrowserIncluded bool
	FolderSelected      string
	FolderRoots         []NPPFolderRoot
}

type NPPFolderRoot struct {
	Path     string
	Expanded []string
}

type nppXMLRoot struct {
	XMLName xml.Name       `xml:"NotepadPlus"`
	Session *nppXMLSession `xml:"Session"`
}

type nppXMLSession struct {
	ActiveView  int            `xml:"activeView,attr"`
	Main        nppXMLView     `xml:"mainView"`
	Sub         nppXMLView     `xml:"subView"`
	FileBrowser *nppXMLBrowser `xml:"FileBrowser"`
}

type nppXMLBrowser struct {
	Selected string             `xml:"latestSelectedItem,attr"`
	Roots    []nppXMLFolderRoot `xml:"root"`
	Unknown  []nppXMLUnknown    `xml:",any"`
}

type nppXMLFolderRoot struct {
	Path     string          `xml:"foldername,attr"`
	Expanded []nppXMLPath    `xml:"expanded"`
	Unknown  []nppXMLUnknown `xml:",any"`
}

type nppXMLPath struct {
	Path    string          `xml:"path,attr"`
	Unknown []nppXMLUnknown `xml:",any"`
}

type nppXMLUnknown struct {
	XMLName xml.Name
}

type nppXMLView struct {
	ActiveIndex int          `xml:"activeIndex,attr"`
	Files       []nppXMLFile `xml:"File"`
}

type nppXMLFile struct {
	Filename       string       `xml:"filename,attr"`
	Encoding       *int         `xml:"encoding,attr"`
	Language       string       `xml:"lang,attr,omitempty"`
	StartPos       int64        `xml:"startPos,attr"`
	EndPos         int64        `xml:"endPos,attr"`
	BackupFilePath string       `xml:"backupFilePath,attr,omitempty"`
	UserReadOnly   string       `xml:"userReadOnly,attr,omitempty"`
	TabPinned      string       `xml:"tabPinned,attr,omitempty"`
	TabColourID    *int         `xml:"tabColourId,attr"`
	Marks          []nppXMLMark `xml:"Mark"`
	Folds          []nppXMLFold `xml:"Fold"`
}

type nppXMLMark struct {
	Line *int `xml:"line,attr"`
}

type nppXMLFold struct {
	Line *int `xml:"line,attr"`
}

func (state NPPState) validate() error {
	if state.ActiveView < 0 || state.ActiveView > 1 || state.ActiveView == 1 && len(state.Sub) == 0 {
		return fmt.Errorf("invalid active NPP view")
	}
	for pane, view := range [][]NPPFile{state.Main, state.Sub} {
		active := state.ActiveMain
		if pane == 1 {
			active = state.ActiveSub
		}
		if active < 0 || len(view) == 0 && active != 0 || len(view) != 0 && active >= len(view) {
			return fmt.Errorf("invalid active tab in NPP view %d", pane)
		}
		seen := make(map[string]bool, len(view))
		for _, file := range view {
			if !filepath.IsAbs(file.Path) || strings.IndexByte(file.Path, 0) >= 0 || seen[file.Path] ||
				file.StartPos < 0 || file.EndPos < 0 || !supportedNPPEncoding(file.Encoding) || file.Color > 5 {
				return fmt.Errorf("invalid or unsupported NPP file entry %q", file.Path)
			}
			seen[file.Path] = true
			marked := make(map[int]bool, len(file.Marks))
			for _, line := range file.Marks {
				if line < 0 || marked[line] {
					return fmt.Errorf("invalid NPP bookmark line %d in %q", line, file.Path)
				}
				marked[line] = true
			}
			folded := make(map[int]bool, len(file.Folds))
			for _, line := range file.Folds {
				if line < 0 || folded[line] {
					return fmt.Errorf("invalid NPP fold line %d in %q", line, file.Path)
				}
				folded[line] = true
			}
		}
	}
	if !state.FileBrowserIncluded && (state.FolderSelected != "" || len(state.FolderRoots) != 0) || len(state.FolderRoots) > 64 {
		return fmt.Errorf("invalid NPP FileBrowser state")
	}
	roots := make(map[string]bool, len(state.FolderRoots))
	for _, root := range state.FolderRoots {
		if !validNPPFolderPath(root.Path) || roots[root.Path] || len(root.Expanded) > 4096 {
			return fmt.Errorf("invalid NPP folder root %q", root.Path)
		}
		roots[root.Path] = true
		seen := make(map[string]bool, len(root.Expanded))
		for _, path := range root.Expanded {
			if !validNPPFolderPath(path) || !nppPathWithin(root.Path, path) || seen[path] {
				return fmt.Errorf("invalid NPP expanded folder %q", path)
			}
			seen[path] = true
		}
	}
	if state.FolderSelected != "" {
		if !validNPPFolderPath(state.FolderSelected) {
			return fmt.Errorf("invalid NPP selected folder item %q", state.FolderSelected)
		}
		found := false
		for _, root := range state.FolderRoots {
			found = found || nppPathWithin(root.Path, state.FolderSelected)
		}
		if !found {
			return fmt.Errorf("selected NPP folder item is outside roots")
		}
	}
	return nil
}

func validNPPFolderPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && strings.IndexByte(path, 0) < 0
}

func nppPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func supportedNPPEncoding(value int) bool {
	switch value {
	case -1, 65001, 1200, 1201:
		return true
	}
	_, ok := document.EncodingFromCodePage(value)
	return ok
}

func LoadNPP(path string) (NPPState, error) {
	file, size, err := openRegularSession(path)
	if err != nil {
		return NPPState{}, err
	}
	defer file.Close()
	if size > maxNPPBytes {
		return NPPState{}, fmt.Errorf("NPP session exceeds 1 MiB")
	}
	decoder := xml.NewDecoder(io.LimitReader(file, maxNPPBytes))
	var stored nppXMLRoot
	if err := decoder.Decode(&stored); err != nil {
		return NPPState{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return NPPState{}, fmt.Errorf("NPP session has trailing data")
	}
	if stored.Session == nil {
		return NPPState{}, fmt.Errorf("NPP session element is missing")
	}
	result := NPPState{ActiveView: stored.Session.ActiveView, ActiveMain: stored.Session.Main.ActiveIndex, ActiveSub: stored.Session.Sub.ActiveIndex}
	if browser := stored.Session.FileBrowser; browser != nil {
		if len(browser.Unknown) != 0 {
			return NPPState{}, fmt.Errorf("unknown NPP FileBrowser element")
		}
		result.FileBrowserIncluded, result.FolderSelected = true, browser.Selected
		for _, root := range browser.Roots {
			if len(root.Unknown) != 0 {
				return NPPState{}, fmt.Errorf("unknown NPP folder root element")
			}
			folder := NPPFolderRoot{Path: root.Path}
			for _, expanded := range root.Expanded {
				if len(expanded.Unknown) != 0 {
					return NPPState{}, fmt.Errorf("unknown NPP expanded folder element")
				}
				folder.Expanded = append(folder.Expanded, expanded.Path)
			}
			result.FolderRoots = append(result.FolderRoots, folder)
		}
	}
	for pane, view := range []nppXMLView{stored.Session.Main, stored.Session.Sub} {
		for _, entry := range view.Files {
			if entry.BackupFilePath != "" {
				return NPPState{}, fmt.Errorf("unsupported NPP session state for %q", entry.Filename)
			}
			readOnly, err := nppReadOnly(entry.UserReadOnly)
			if err != nil {
				return NPPState{}, fmt.Errorf("invalid NPP read-only state for %q: %w", entry.Filename, err)
			}
			pinned, err := nppReadOnly(entry.TabPinned)
			if err != nil {
				return NPPState{}, fmt.Errorf("invalid NPP pinned state for %q: %w", entry.Filename, err)
			}
			color := uint8(0)
			if entry.TabColourID != nil {
				if *entry.TabColourID < -1 || *entry.TabColourID > 4 {
					return NPPState{}, fmt.Errorf("invalid NPP tabColourId %d for %q", *entry.TabColourID, entry.Filename)
				}
				color = uint8(*entry.TabColourID + 1)
			}
			encoding := -1
			if entry.Encoding != nil {
				encoding = *entry.Encoding
			}
			file := NPPFile{Path: entry.Filename, Encoding: encoding, Language: entry.Language, StartPos: entry.StartPos, EndPos: entry.EndPos, ReadOnly: readOnly, Pinned: pinned, Color: color}
			for _, mark := range entry.Marks {
				if mark.Line == nil {
					return NPPState{}, fmt.Errorf("NPP bookmark has no line in %q", entry.Filename)
				}
				file.Marks = append(file.Marks, *mark.Line)
			}
			for _, fold := range entry.Folds {
				if fold.Line == nil {
					return NPPState{}, fmt.Errorf("NPP fold has no line in %q", entry.Filename)
				}
				file.Folds = append(file.Folds, *fold.Line)
			}
			sort.Ints(file.Marks)
			sort.Ints(file.Folds)
			if pane == 0 {
				result.Main = append(result.Main, file)
			} else {
				result.Sub = append(result.Sub, file)
			}
		}
	}
	if err := result.validate(); err != nil {
		return NPPState{}, err
	}
	return result, nil
}

func nppReadOnly(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "", "no", "false", "0":
		return false, nil
	case "yes", "true", "1":
		return true, nil
	}
	return false, fmt.Errorf("unknown value %q", value)
}

func SaveNPP(path string, state NPPState) error {
	if err := state.validate(); err != nil {
		return err
	}
	toView := func(files []NPPFile, active int) nppXMLView {
		view := nppXMLView{ActiveIndex: active}
		for _, file := range files {
			encoding := file.Encoding
			colorID := int(file.Color) - 1
			entry := nppXMLFile{
				Filename: file.Path, Encoding: &encoding, Language: file.Language,
				StartPos: file.StartPos, EndPos: file.EndPos,
				TabColourID: &colorID,
			}
			if file.ReadOnly {
				entry.UserReadOnly = "yes"
			}
			if file.Pinned {
				entry.TabPinned = "yes"
			}
			for _, line := range file.Marks {
				markLine := line
				entry.Marks = append(entry.Marks, nppXMLMark{Line: &markLine})
			}
			for _, line := range file.Folds {
				foldLine := line
				entry.Folds = append(entry.Folds, nppXMLFold{Line: &foldLine})
			}
			sort.Slice(entry.Marks, func(first, second int) bool {
				return *entry.Marks[first].Line < *entry.Marks[second].Line
			})
			sort.Slice(entry.Folds, func(first, second int) bool {
				return *entry.Folds[first].Line < *entry.Folds[second].Line
			})
			view.Files = append(view.Files, entry)
		}
		return view
	}
	stored := nppXMLRoot{Session: &nppXMLSession{
		ActiveView: state.ActiveView,
		Main:       toView(state.Main, state.ActiveMain), Sub: toView(state.Sub, state.ActiveSub),
	}}
	if state.FileBrowserIncluded {
		browser := &nppXMLBrowser{Selected: state.FolderSelected}
		for _, root := range state.FolderRoots {
			entry := nppXMLFolderRoot{Path: root.Path}
			expanded := append([]string(nil), root.Expanded...)
			sort.Strings(expanded)
			for _, path := range expanded {
				entry.Expanded = append(entry.Expanded, nppXMLPath{Path: path})
			}
			browser.Roots = append(browser.Roots, entry)
		}
		stored.Session.FileBrowser = browser
	}
	data, err := xml.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	data = append([]byte(xml.Header), data...)
	data = append(data, '\n')
	if len(data) > maxNPPBytes {
		return fmt.Errorf("NPP session exceeds 1 MiB")
	}
	absolute, err := document.CanonicalSavePath(path)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(absolute), ".vivapad-npp-session-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), absolute)
}
