package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
)

func nppBytePosition(doc *document.Document, index int) int64 {
	value := doc.Editor.String()
	sourceOffset := doc.Editor.ByteOffset(index)
	physicalOffset, err := doc.SavedEncoding().EncodedLength(value[:sourceOffset])
	if err != nil {
		return -1
	}
	line := 0
	for position := 0; position < sourceOffset; position++ {
		if value[position] == '\n' {
			if doc.LineEndingAt(line) == "\r\n" {
				physicalOffset++
			}
			line++
		}
	}
	return int64(physicalOffset)
}

func runeFromNPPPosition(doc *document.Document, position int64) int {
	value := doc.Editor.String()
	var offset int64
	line := 0
	for byteIndex, character := range value {
		width := utf8.RuneLen(character)
		if doc.SavedEncoding().IsLegacy() {
			encoded, err := doc.SavedEncoding().EncodedLength(string(character))
			if err != nil {
				return doc.Editor.RuneIndex(byteIndex)
			}
			width = encoded
		}
		if character == '\n' {
			if doc.LineEndingAt(line) == "\r\n" {
				width = 2
			}
			line++
		}
		if offset+int64(width) > position {
			return doc.Editor.RuneIndex(byteIndex)
		}
		offset += int64(width)
	}
	return doc.Editor.RuneCount()
}

func nppCodepage(doc *document.Document) int {
	if codePage, ok := doc.SavedEncoding().CodePage(); ok {
		return codePage
	}
	return -1
}

func nppLanguage(extension string) string {
	if extension == ".txt" {
		return "normal"
	}
	if spec, ok := syntax.LanguageByMode(extension); ok {
		return spec.NPPName
	}
	return ""
}

func nppExtension(language string) (string, error) {
	if language == "" {
		return "", nil
	}
	if strings.EqualFold(language, "text") || strings.EqualFold(language, "txt") {
		return "", nil
	}
	if spec, ok := syntax.LanguageByNPP(language); ok {
		return spec.Mode, nil
	}
	return "", fmt.Errorf("unsupported NPP language %q", language)
}

func (state *model) nppSessionState() (session.NPPState, error) {
	return state.nppSessionStateWithFolders(state.folderVisible)
}

func (state *model) nppSessionStateWithFolders(includeWorkspace bool) (session.NPPState, error) {
	result := session.NPPState{}
	for pane := range 2 {
		if pane == 1 && !state.splitEnabled {
			break
		}
		docs := state.space.Documents
		if state.splitEnabled {
			docs = state.paneTabs(pane)
		}
		for _, doc := range docs {
			if doc.Dirty() {
				return session.NPPState{}, fmt.Errorf("Save modified tabs before exporting an NPP session")
			}
			if doc.Path == "" {
				continue
			}
			selection := doc.Editor.Selection()
			marks := state.bookmarkLines(doc)
			if !state.bookmarksReady(doc) {
				return session.NPPState{}, fmt.Errorf("%s", state.notice)
			}
			entry := session.NPPFile{
				Path: doc.Path, Encoding: nppCodepage(doc), Language: nppLanguage(state.languageFor(doc)),
				StartPos: nppBytePosition(doc, selection.Start), EndPos: nppBytePosition(doc, selection.End),
				Marks: append([]int(nil), marks...), Folds: state.foldHeaders(doc), ReadOnly: state.readOnly[doc] && state.monitors[doc] == nil, Pinned: state.isPinned(doc), Color: state.tabColors[doc],
			}
			if selection.Start != selection.End && doc.Editor.Caret() == selection.Start {
				entry.StartPos, entry.EndPos = entry.EndPos, entry.StartPos
			}
			if pane == 0 {
				result.Main = append(result.Main, entry)
			} else {
				result.Sub = append(result.Sub, entry)
			}
		}
	}
	activePath := state.space.Current().Path
	mainPath := activePath
	if state.splitEnabled && state.paneDocs[0] != nil {
		mainPath = state.paneDocs[0].Path
	}
	for index, file := range result.Main {
		if file.Path == mainPath {
			result.ActiveMain = index
		}
	}
	if state.splitEnabled && len(result.Sub) > 0 {
		result.ActiveView = state.activePane
		if state.paneDocs[1] != nil {
			for index, file := range result.Sub {
				if file.Path == state.paneDocs[1].Path {
					result.ActiveSub = index
				}
			}
		}
	}
	if includeWorkspace {
		if err := state.exportNPPFolders(&result); err != nil {
			return session.NPPState{}, err
		}
	}
	return result, nil
}

func (state *model) saveNPPSession(includeWorkspace bool) {
	stored, err := state.nppSessionStateWithFolders(includeWorkspace)
	if err != nil {
		state.saveSessionError = err.Error()
		return
	}
	state.saveSessionOptionsOpen = false
	path, err := app.SaveFile(app.SaveFileOptions{Title: "Save Session", SuggestedName: "session.xml", DefaultExtension: "xml"})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return
	}
	state.writeNPPSession(path, stored)
}

func (state *model) saveNPPSessionNative(selection app.SaveFileSelection) {
	stored, err := state.nppSessionStateWithFolders(selection.Checked)
	if err != nil {
		state.notice = err.Error()
		return
	}
	state.writeNPPSession(selection.Path, stored)
}

func (state *model) writeNPPSession(path string, stored session.NPPState) {
	absolute, err := document.CanonicalSavePath(path)
	if err != nil {
		state.notice = err.Error()
		return
	}
	if state.isPrivateDataPath(absolute) {
		state.notice = "Session destination is an internal VivaPad data file"
		return
	}
	for _, doc := range state.space.Documents {
		if doc.Path != "" && doc.Path == absolute {
			state.notice = "Session destination is already open in a tab"
			return
		}
	}
	if err := session.SaveNPP(absolute, stored); err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = "Saved session " + filepath.Base(absolute)
}

type preparedNPPFile struct {
	file     session.NPPFile
	doc      *document.Document
	pane     int
	language string
}

func (state *model) loadNPPSession() {
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: "Load Session"})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
		}
		return
	}
	if len(paths) == 0 {
		return
	}
	stored, err := session.LoadNPP(paths[0])
	if err != nil {
		state.notice = "Session not loaded: " + err.Error()
		return
	}
	var prepared []preparedNPPFile
	var paneFiles [2][]*document.Document
	seen := make(map[string]*document.Document)
	for pane, entries := range [][]session.NPPFile{stored.Main, stored.Sub} {
		for _, file := range entries {
			language, err := nppExtension(file.Language)
			if err != nil {
				state.notice = "Session not loaded: " + err.Error()
				return
			}
			canonical, err := filepath.EvalSymlinks(file.Path)
			if err != nil {
				state.notice = "Session file unavailable: " + file.Path + ": " + err.Error()
				return
			}
			doc := seen[canonical]
			if doc == nil {
				for _, existing := range state.space.Documents {
					if existing.Path == canonical {
						doc = existing
						break
					}
				}
			}
			format, explicit := document.EncodingFromCodePage(file.Encoding)
			if doc == nil {
				if explicit {
					doc, err = document.OpenWithEncoding(canonical, format)
				} else {
					doc, err = document.Open(canonical)
				}
				if err != nil {
					state.notice = "Session file unavailable: " + file.Path + ": " + err.Error()
					return
				}
			}
			if explicit && doc.Encoding() != format || !explicit && doc.Encoding().IsLegacy() {
				state.notice = "Session file is already open with another encoding: " + canonical
				return
			}
			for _, line := range file.Marks {
				if line >= doc.LineCount() {
					state.notice = fmt.Sprintf("Session bookmark line %d is out of range in %s", line, canonical)
					return
				}
			}
			foldMode := language
			if file.Language == "" {
				foldMode = state.languageFor(doc)
			}
			regions := syntax.FoldRegions(foldMode, doc.Editor.String())
			validFolds := make(map[int]bool, len(regions))
			for _, region := range regions {
				validFolds[region.Header] = true
			}
			for _, line := range file.Folds {
				if !validFolds[line] {
					state.notice = fmt.Sprintf("Session fold line %d is unavailable in %s", line, canonical)
					return
				}
			}
			if existing := seen[canonical]; existing != nil {
				for _, item := range prepared {
					if item.doc == existing && (!slices.Equal(item.file.Marks, file.Marks) || !slices.Equal(item.file.Folds, file.Folds) || item.file.ReadOnly != file.ReadOnly || item.file.Pinned != file.Pinned || item.file.Color != file.Color || item.language != language) {
						state.notice = "Session has conflicting document state for " + canonical
						return
					}
				}
			}
			seen[canonical] = doc
			prepared = append(prepared, preparedNPPFile{file: file, doc: doc, pane: pane, language: language})
			paneFiles[pane] = append(paneFiles[pane], doc)
		}
	}
	folders, selectedFolder, err := state.prepareNPPFolders(stored)
	if err != nil {
		state.notice = "Session not loaded: " + err.Error()
		return
	}
	if len(prepared) == 0 && len(folders) == 0 {
		state.notice = "Session contains no saved files"
		return
	}
	oldBlank := state.space.Current()
	if len(state.space.Documents) != 1 || oldBlank.Path != "" || oldBlank.Dirty() {
		oldBlank = nil
	}
	for _, item := range prepared {
		if state.documentIndex(item.doc) < 0 {
			if err := state.space.AddOpened(item.doc); err != nil {
				state.notice = "Session import failed: " + err.Error()
				return
			}
		}
		state.paneOwner[item.doc] = item.pane
		state.setBookmarkLines(item.doc, item.file.Marks)
		state.readOnly[item.doc] = item.file.ReadOnly
		state.pinned[item.doc] = item.file.Pinned
		if item.file.Color == 0 {
			delete(state.tabColors, item.doc)
		} else {
			state.tabColors[item.doc] = item.file.Color
		}
		if item.file.Language != "" {
			state.languages[item.doc] = item.language
			state.configureCode(item.doc)
		}
		if err := state.applyFoldHeaders(item.doc, item.file.Folds); err != nil {
			state.notice = "Session fold unavailable: " + err.Error()
			return
		}
		item.doc.Editor.SetSelection(text.Range{
			Start: runeFromNPPPosition(item.doc, item.file.StartPos),
			End:   runeFromNPPPosition(item.doc, item.file.EndPos),
		})
		state.rememberRecent(item.doc)
	}
	if oldBlank != nil && state.documentIndex(oldBlank) < 0 {
		var replacement *document.Document
		if len(paneFiles[0]) > 0 {
			replacement = paneFiles[0][0]
		} else {
			replacement = paneFiles[1][0]
		}
		for pane, current := range state.paneDocs {
			if current == oldBlank {
				state.paneDocs[pane] = replacement
			}
		}
		state.releaseDocument(oldBlank)
	}
	if len(paneFiles[0]) > 0 {
		state.paneDocs[0] = paneFiles[0][stored.ActiveMain]
	}
	if len(paneFiles[1]) > 0 {
		state.splitEnabled = true
		state.paneDocs[1] = paneFiles[1][stored.ActiveSub]
		state.activePane = stored.ActiveView
	} else if len(paneFiles[0]) > 0 {
		state.activePane = 0
	}
	if index := state.documentIndex(state.paneDocument(state.activePane)); index >= 0 {
		state.space.Select(index)
	}
	if err := state.applyNPPFolders(folders, selectedFolder); err != nil {
		state.notice = "Session folders partially loaded: " + err.Error()
		return
	}
	state.notice = fmt.Sprintf("Loaded %d session file(s), %d folder(s)", len(seen), len(folders))
}
