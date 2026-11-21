package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/completion"
	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

type completionKind uint8

const (
	completionWord completionKind = iota + 1
	completionPath
	completionFunction
)

const functionCompletionCommand = "Function Completion"

func isCompletionCommand(command string) bool {
	return command == "Word Completion" || command == "Path Completion" || command == functionCompletionCommand
}

type wordCompletionState struct {
	kind     completionKind
	doc      *document.Document
	pane     int
	base     string
	language string
	catalog  *completion.FunctionCatalog
	source   string
	start    int
	end      int
	caret    int
	items    []string
	selected int
	offset   int
}

func (state *model) startFunctionCompletion() {
	doc := state.paneDocument(state.activePane)
	state.wordCompletion = nil
	state.wordFocusPending = true
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	if !state.hasSingleIdleCaret(doc) {
		state.notice = "Place the caret within a function name to complete it"
		return
	}
	language := state.languageFor(doc)
	catalog, err := state.functionCatalog(language)
	if err != nil {
		state.notice = "Function completion: " + err.Error()
		return
	}
	if catalog == nil {
		state.notice = "No API function catalog for this language"
		return
	}
	value := doc.Editor.String()
	result := functionCompletionsAtCaret(doc.Editor, value, catalog)
	if len(result.Candidates) == 0 {
		state.notice = "No function completion"
		return
	}
	if len(result.Candidates) == 1 {
		state.completeCandidate(doc, result.Start, result.End, result.Candidates[0], completionFunction)
		return
	}
	state.wordCompletion = &wordCompletionState{
		kind: completionFunction, doc: doc, pane: state.activePane, source: value,
		language: language, catalog: catalog, start: result.Start, end: result.End,
		caret: doc.Editor.Caret(), items: result.Candidates,
	}
	if result.Truncated {
		state.notice = "Function completion limited to 2000 suggestions"
	}
}

func (state *model) functionCatalog(language string) (*completion.FunctionCatalog, error) {
	spec, _ := syntax.LanguageByMode(language)
	directory := ""
	if state.preferencesPath != "" {
		directory = filepath.Dir(state.preferencesPath)
	}
	return completion.LoadFunctionCatalog(directory, spec.NPPName)
}

func functionCompletionsAtCaret(editor *text.Editor, value string, catalog *completion.FunctionCatalog) completion.Result {
	caret := editor.Caret()
	offset := editor.ByteOffset(caret)
	start := strings.LastIndexAny(value[:offset], "\r\n") + 1
	end := len(value)
	if newline := strings.IndexAny(value[offset:], "\r\n"); newline >= 0 {
		end = offset + newline
	}
	result := catalog.Complete(value[start:end], offset-start)
	if len(result.Candidates) > 0 {
		lineStart := caret - utf8.RuneCountInString(value[start:offset])
		result.Start += lineStart
		result.End += lineStart
	}
	return result
}

func (state *model) startWordCompletion() {
	doc := state.paneDocument(state.activePane)
	state.wordCompletion = nil
	state.wordFocusPending = true
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	if !state.hasSingleIdleCaret(doc) {
		state.notice = "Place the caret within a word to complete it"
		return
	}
	value := doc.Editor.String()
	language := state.languageFor(doc)
	ignoreCase := language != "" && language != ".txt"
	result := completion.Words(value, doc.Editor.Caret(), ignoreCase)
	if len(result.Candidates) == 0 {
		state.notice = "No word completion"
		return
	}
	if len(result.Candidates) == 1 {
		state.completeCandidate(doc, result.Start, result.End, result.Candidates[0], completionWord)
		return
	}
	state.wordCompletion = &wordCompletionState{
		kind: completionWord, doc: doc, pane: state.activePane, source: value,
		start: result.Start, end: result.End, caret: doc.Editor.Caret(), items: result.Candidates,
	}
}

func (state *model) startPathCompletion() {
	doc := state.paneDocument(state.activePane)
	state.wordCompletion = nil
	state.wordFocusPending = true
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	if !state.hasSingleIdleCaret(doc) {
		state.notice = "Place the caret within a path to complete it"
		return
	}
	base, err := os.Getwd()
	if doc.Path != "" {
		base = filepath.Dir(doc.Path)
	}
	if err != nil && doc.Path == "" {
		state.notice = "Path completion: " + err.Error()
		return
	}
	value := doc.Editor.String()
	result, err := pathCompletionsAtCaret(doc.Editor, value, base)
	if err != nil {
		state.notice = "Path completion: " + err.Error()
		return
	}
	if len(result.Candidates) == 0 {
		if result.Truncated {
			state.notice = "No path completion in first 2000 entries"
		} else {
			state.notice = "No path completion"
		}
		return
	}
	state.wordCompletion = &wordCompletionState{
		kind: completionPath, doc: doc, pane: state.activePane, base: base, source: value,
		start: result.Start, end: result.End, caret: doc.Editor.Caret(), items: result.Candidates,
	}
	if result.Truncated {
		state.notice = "Path completion limited to first 2000 entries"
	} else {
		state.notice = ""
	}
}

func pathCompletionsAtCaret(editor *text.Editor, value, base string) (completion.Result, error) {
	caret := editor.Caret()
	offset := editor.ByteOffset(caret)
	start := strings.LastIndexAny(value[:offset], "\r\n") + 1
	end := len(value)
	if newline := strings.IndexAny(value[offset:], "\r\n"); newline >= 0 {
		end = offset + newline
	}
	lineCaret := utf8.RuneCountInString(value[start:offset])
	result, err := completion.Paths(value[start:end], lineCaret, base)
	if err != nil || len(result.Candidates) == 0 {
		return result, err
	}
	lineStart := caret - lineCaret
	result.Start += lineStart
	result.End += lineStart
	return result, nil
}

func (state *model) hasSingleIdleCaret(doc *document.Document) bool {
	editor := doc.Editor
	selection := editor.Selection()
	if editor.CompositionActive() || editor.SelectionCount() != 1 || selection.Start != selection.End || state.hasMultipleStreamSelections(doc) {
		return false
	}
	rect := state.rectangles[state.activePane]
	return !rect.active || rect.doc != doc || rect.value != editor.String()
}

func (state *model) validWordCompletion() bool {
	current := state.wordCompletion
	if current == nil || state.activePane != current.pane || state.paneDocument(current.pane) != current.doc ||
		state.space.Current() != current.doc || state.isReadOnly(current.doc) || current.doc.Editor.String() != current.source ||
		current.doc.Editor.Caret() != current.caret || current.kind == completionFunction && state.languageFor(current.doc) != current.language {
		return false
	}
	return state.hasSingleIdleCaret(current.doc)
}

func (state *model) acceptWordCompletion(index int) {
	current := state.wordCompletion
	if !state.validWordCompletion() || index < 0 || index >= len(current.items) {
		state.wordCompletion = nil
		return
	}
	before := current.doc.Editor.String()
	candidate := current.items[index]
	state.wordCompletion = nil
	if current.kind == completionPath {
		editor := current.doc.Editor
		raw := before[editor.ByteOffset(current.start):editor.ByteOffset(current.end)]
		if exists, err := completion.PathNamesFile(raw, current.base); err != nil {
			state.notice = "Path completion: " + err.Error()
			return
		} else if exists {
			state.notice = "Path already names a file"
			return
		}
		if available, err := completion.PathCandidateAvailable(candidate, current.base); err != nil {
			state.notice = "Path completion: " + err.Error()
			return
		} else if !available {
			state.notice = "Path suggestion is no longer available"
			return
		}
	}
	state.completeCandidate(current.doc, current.start, current.end, candidate, current.kind)
	state.relocateSelectAnchors(current.doc, before, current.doc.Editor.String(), current.start)
}

func (state *model) updateWordCompletionAfterInput(doc *document.Document) {
	current := state.wordCompletion
	if current == nil || current.doc != doc {
		return
	}
	if !state.hasSingleIdleCaret(doc) {
		state.wordCompletion = nil
		return
	}
	value := doc.Editor.String()
	var result completion.Result
	if current.kind == completionPath {
		var err error
		result, err = pathCompletionsAtCaret(doc.Editor, value, current.base)
		if err != nil {
			state.wordCompletion = nil
			state.notice = "Path completion: " + err.Error()
			return
		}
	} else if current.kind == completionFunction {
		if state.languageFor(doc) != current.language {
			state.wordCompletion = nil
			return
		}
		result = functionCompletionsAtCaret(doc.Editor, value, current.catalog)
	} else {
		language := state.languageFor(doc)
		result = completion.Words(value, doc.Editor.Caret(), language != "" && language != ".txt")
	}
	if len(result.Candidates) == 0 {
		state.wordCompletion = nil
		return
	}
	current.source, current.start, current.end, current.caret = value, result.Start, result.End, doc.Editor.Caret()
	current.items, current.selected, current.offset = result.Candidates, 0, 0
}

func (state *model) completeCandidate(doc *document.Document, start, end int, word string, kind completionKind) {
	doc.Editor.SetSelection(text.Range{Start: start, End: end})
	state.editWithBookmarkAnchor(doc, func() { doc.Editor.Paste(word) })
	if kind == completionPath {
		state.notice = "Completed path"
	} else {
		state.notice = "Completed " + word
	}
}
