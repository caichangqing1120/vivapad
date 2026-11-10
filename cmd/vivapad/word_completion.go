package main

import (
	"os"
	"path/filepath"

	"github.com/vivaui/viva/text"
	"vivapad/internal/completion"
	"vivapad/internal/document"
)

type completionKind uint8

const (
	completionWord completionKind = iota + 1
	completionPath
)

type wordCompletionState struct {
	kind     completionKind
	doc      *document.Document
	pane     int
	base     string
	source   string
	start    int
	end      int
	items    []string
	selected int
	offset   int
}

func (state *model) startWordCompletion() {
	doc := state.paneDocument(state.activePane)
	state.wordCompletion = nil
	state.wordFocusPending = true
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	if _, composing := doc.Editor.Composition(); composing || doc.Editor.Selection().Start != doc.Editor.Selection().End {
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
		start: result.Start, end: result.End, items: result.Candidates,
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
	if _, composing := doc.Editor.Composition(); composing || doc.Editor.Selection().Start != doc.Editor.Selection().End {
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
	result, err := completion.Paths(value, doc.Editor.Caret(), base)
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
		start: result.Start, end: result.End, items: result.Candidates,
	}
	if result.Truncated {
		state.notice = "Path completion limited to first 2000 entries"
	}
}

func (state *model) validWordCompletion() bool {
	current := state.wordCompletion
	if current == nil || state.activePane != current.pane || state.paneDocument(current.pane) != current.doc ||
		state.space.Current() != current.doc || state.isReadOnly(current.doc) || current.doc.Editor.String() != current.source ||
		current.doc.Editor.Caret() != current.end {
		return false
	}
	selection := current.doc.Editor.Selection()
	_, composing := current.doc.Editor.Composition()
	return !composing && selection.Start == selection.End
}

func (state *model) acceptWordCompletion(index int) {
	current := state.wordCompletion
	if !state.validWordCompletion() || index < 0 || index >= len(current.items) {
		state.wordCompletion = nil
		return
	}
	before := current.doc.Editor.String()
	state.wordCompletion = nil
	state.completeCandidate(current.doc, current.start, current.end, current.items[index], current.kind)
	state.relocateSelectAnchors(current.doc, before, current.doc.Editor.String(), current.start)
}

func (state *model) updateWordCompletionAfterInput(doc *document.Document) {
	current := state.wordCompletion
	if current == nil || current.doc != doc {
		return
	}
	if _, composing := doc.Editor.Composition(); composing || doc.Editor.Selection().Start != doc.Editor.Selection().End {
		state.wordCompletion = nil
		return
	}
	value := doc.Editor.String()
	var result completion.Result
	if current.kind == completionPath {
		var err error
		result, err = completion.Paths(value, doc.Editor.Caret(), current.base)
		if err != nil {
			state.wordCompletion = nil
			state.notice = "Path completion: " + err.Error()
			return
		}
	} else {
		language := state.languageFor(doc)
		result = completion.Words(value, doc.Editor.Caret(), language != "" && language != ".txt")
	}
	if len(result.Candidates) == 0 {
		state.wordCompletion = nil
		return
	}
	current.source, current.start, current.end = value, result.Start, result.End
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
