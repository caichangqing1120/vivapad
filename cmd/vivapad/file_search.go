package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/searchfiles"
)

func (state *model) filesDialogField(id gfx.ID) bool {
	return id == queryID || id == filesRootID || id == filesPatternID ||
		state.filesReplaceMode && id == replacementID
}

func (state *model) openFilesDialog() {
	state.closeIncrementalSearch()
	state.markDialogOpen = false
	state.filesDialogOpen, state.searchOpen, state.goToOpen = true, false, false
	state.filesMessage = ""
	if path := state.space.Current().Path; path != "" {
		state.filesRoot = text.NewEditor(filepath.Dir(path))
	}
}

func (state *model) clearResultFocus() {
	state.resultFocusMode, state.resultFocusRequested = false, false
	state.resultFocusArmed, state.resultReturnToEditor = false, false
}

func (state *model) discardFileResults() {
	state.fileResults = searchfiles.Result{}
	state.resultQuery, state.resultOffset, state.resultSelected = "", 0, -1
	state.resultsOpen, state.resultDocument, state.resultDocuments, state.resultOpenDocuments = false, nil, nil, false
	state.clearResultFocus()
}

func (state *model) browseFilesDirectory() {
	path, err := app.OpenDirectory(app.OpenDirectoryOptions{Title: "Find in Files"})
	if err != nil {
		if !errors.Is(err, app.ErrCanceled) {
			state.notice = err.Error()
			state.filesMessage = err.Error()
		}
		return
	}
	state.filesRoot = text.NewEditor(path)
}

func (state *model) startFileSearch() {
	state.cancelFileSearch()
	state.clearResultFocus()
	state.resultSelected = -1
	state.filesMessage = ""
	state.searchGeneration++
	generation := state.searchGeneration
	root, pattern, query, options := state.filesRoot.String(), state.filesPattern.String(), state.query.String(), state.searchOptions
	recursive, encoding := state.filesRecursive, state.filesEncoding
	ctx, cancel := context.WithCancel(context.Background())
	state.searchCancel = cancel
	state.filesSearching = true
	state.notice = "Searching files..."
	go func() {
		result, err := searchfiles.Find(ctx, root, query, options, searchfiles.Options{Pattern: pattern, Recursive: recursive, Encoding: encoding})
		reply := fileSearchResponse{generation: generation, query: query, result: result, err: err}
		state.sendFileReply(reply)
	}()
}

func (state *model) startReplacePreview() {
	state.cancelFileSearch()
	state.filesMessage = ""
	state.searchGeneration++
	generation := state.searchGeneration
	root, pattern, query, replacement, options := state.filesRoot.String(), state.filesPattern.String(), state.query.String(), state.replacement.String(), state.searchOptions
	recursive, encoding := state.filesRecursive, state.filesEncoding
	ctx, cancel := context.WithCancel(context.Background())
	state.searchCancel = cancel
	state.filesSearching = true
	state.notice = "Preparing replacement preview..."
	go func() {
		plan, err := searchfiles.PreviewReplace(ctx, root, query, replacement, options, searchfiles.Options{Pattern: pattern, Recursive: recursive, Encoding: encoding})
		state.sendFileReply(fileSearchResponse{kind: fileJobReplacePreview, generation: generation, plan: plan, err: err})
	}()
}

func (state *model) startReplaceApply() {
	if state.replacePlan == nil || state.filesApplying {
		return
	}
	for _, change := range state.replacePlan.Changes {
		for _, doc := range state.space.Documents {
			if doc.Path == change.Path {
				if state.isReadOnly(doc) {
					state.notice = "Read-only open tab cannot be replaced on disk"
					return
				}
				if doc.Dirty() {
					state.notice = "Save or close modified tabs before replacing files"
					return
				}
				if doc.Encoding() != change.Encoding {
					state.notice = "Close tabs opened with another encoding before replacing files"
					return
				}
			}
		}
	}
	state.searchGeneration++
	generation, plan := state.searchGeneration, state.replacePlan
	state.filesApplying = true
	state.notice = "Replacing files..."
	go func() {
		result, err := plan.Apply(context.Background())
		state.sendFileReply(fileSearchResponse{kind: fileJobReplaceApply, generation: generation, applied: result, err: err})
	}()
}

func (state *model) sendFileReply(reply fileSearchResponse) {
	for {
		select {
		case state.searchReplies <- reply:
			state.invalidator.Invalidate()
			return
		default:
			select {
			case <-state.searchReplies:
			default:
			}
		}
	}
}

func (state *model) consumeFileSearch() {
	for {
		select {
		case reply := <-state.searchReplies:
			if reply.generation != state.searchGeneration {
				continue
			}
			if reply.kind == fileJobReplaceApply {
				if !state.filesApplying {
					continue
				}
				state.filesApplying, state.replaceConfirmOpen = false, false
				state.replacePlan = nil
				state.discardFileResults()
				reloadErr := state.refreshReplacedTabs(reply.applied)
				if reply.err != nil {
					state.notice = reply.err.Error()
				} else {
					state.notice = fmt.Sprintf("Replaced %d matches in %d files", reply.applied.Replacements, reply.applied.FilesChanged)
				}
				if reloadErr != nil {
					state.notice += "; tab reload failed: " + reloadErr.Error()
				}
				continue
			}
			if !state.filesSearching {
				continue
			}
			state.filesSearching = false
			state.searchCancel()
			state.searchCancel = nil
			if reply.err != nil {
				state.notice = reply.err.Error()
				state.filesMessage = reply.err.Error()
				return
			}
			if reply.kind == fileJobReplacePreview {
				if len(reply.plan.Changes) == 0 {
					state.notice = "No changes to preview"
					state.filesMessage = state.notice
					return
				}
				state.replacePlan = reply.plan
				state.replaceConfirmOpen, state.filesDialogOpen = true, false
				state.discardFileResults()
				state.notice = fmt.Sprintf("Preview: %d matches in %d files", reply.plan.Replacements, len(reply.plan.Changes))
				continue
			}
			state.fileResults, state.resultOffset, state.resultSelected = reply.result, 0, -1
			state.clearResultFocus()
			state.resultDocument = nil
			state.resultDocuments, state.resultOpenDocuments = nil, false
			state.resultQuery = reply.query
			state.resultsOpen, state.filesDialogOpen = true, false
			state.notice = fmt.Sprintf("%d matches in %d files (%d skipped)", len(reply.result.Matches), reply.result.FilesSearched, reply.result.FilesSkipped)
		default:
			return
		}
	}
}

func (state *model) refreshReplacedTabs(result searchfiles.ApplyResult) error {
	var reloadErrors []error
	for _, path := range result.Paths {
		for index, doc := range state.space.Documents {
			if doc.Path != path {
				continue
			}
			language, selected := state.languages[doc]
			old, err := state.space.Reload(index)
			if err != nil {
				reloadErrors = append(reloadErrors, fmt.Errorf("%s: %w", path, err))
				continue
			}
			updated := state.space.Documents[index]
			state.rebindBookmarksOnReload(old, updated)
			state.replacePaneDocument(old, updated)
			// Directory replace retires the tab's document. Consume that source once
			// before it is released: it is the only source in this operation that
			// can have pending frames, and its explicit unavailable error must not
			// be discarded silently with the retired document. The replacement is a
			// fresh document.Open source bound before it is reachable, so it starts
			// with an available, empty history by construction.
			if _, historyErr := old.ChangeHistory(); historyErr != nil {
				reloadErrors = append(reloadErrors, fmt.Errorf("%s retired history: %w", path, historyErr))
			}
			state.releaseDocument(old)
			if selected {
				state.languages[updated] = language
			}
		}
	}
	return errors.Join(reloadErrors...)
}

func (state *model) cancelFileSearch() {
	if state.searchCancel != nil {
		state.searchCancel()
		state.searchCancel = nil
	}
	state.searchGeneration++
	state.filesSearching = false
}

func (state *model) navigateFileResult(previous bool) bool {
	if !state.resultsOpen || len(state.fileResults.Matches) == 0 {
		state.notice = "No search results"
		return false
	}
	index := state.resultSelected + 1
	if previous {
		index = state.resultSelected - 1
		if state.resultSelected < 0 {
			index = len(state.fileResults.Matches) - 1
		}
	}
	if index < 0 {
		index = len(state.fileResults.Matches) - 1
	} else if index >= len(state.fileResults.Matches) {
		index = 0
	}
	return state.openFileResult(index)
}

func (state *model) selectFileResult(index int) {
	state.resultSelected = index
	visible := max(1, state.resultVisible)
	if index < state.resultOffset {
		state.resultOffset = index
	} else if index >= state.resultOffset+visible {
		state.resultOffset = index - visible + 1
	}
}

func (state *model) openFileResult(index int) bool {
	if index < 0 || index >= len(state.fileResults.Matches) {
		return false
	}
	match := state.fileResults.Matches[index]
	doc := state.resultDocument
	if state.resultOpenDocuments && index < len(state.resultDocuments) {
		doc = state.resultDocuments[index]
	}
	inMemory, alreadyOpen := doc != nil, doc != nil
	if doc != nil {
		position := state.documentIndex(doc)
		if position < 0 {
			state.discardFileResults()
			state.notice = "Search result document is closed"
			return false
		}
	} else {
		absolute, err := filepath.Abs(match.Path)
		if err != nil {
			state.notice = err.Error()
			return false
		}
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
			absolute = resolved
		}
		for _, opened := range state.space.Documents {
			if opened.Path == absolute {
				doc, alreadyOpen = opened, true
				break
			}
		}
		if alreadyOpen && match.Encoding.IsLegacy() && doc.Encoding() != match.Encoding {
			state.notice = fmt.Sprintf("%s is already open as %s", doc.Name, doc.EncodingLabel())
			return false
		}
		if !alreadyOpen {
			if match.Encoding.IsLegacy() {
				doc, err = document.OpenWithEncoding(absolute, match.Encoding)
			} else {
				doc, err = document.Open(absolute)
			}
			if err != nil {
				state.notice = err.Error()
				return false
			}
		}
	}
	value := doc.Editor.String()
	if match.Source != value || match.Start < 0 || match.End > len(value) || match.Start >= match.End || value[match.Start:match.End] != match.Text {
		state.notice = "Search result changed; run the search again"
		return false
	}
	if !alreadyOpen {
		if err := state.space.AddOpened(doc); err != nil {
			state.notice = err.Error()
			return false
		}
	}
	state.space.Select(state.documentIndex(doc))
	if alreadyOpen && state.splitEnabled {
		pane := state.activePane
		if state.paneDocs[pane] != doc {
			if state.paneDocs[1-pane] == doc {
				pane = 1 - pane
			} else {
				pane = state.paneOwner[doc]
			}
		}
		state.activePane, state.paneDocs[pane] = pane, doc
	} else {
		state.showInActivePane(doc)
	}
	if !inMemory {
		state.rememberRecent(doc)
	}
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneIndex(match.Start), End: doc.Editor.RuneIndex(match.End)})
	state.codeEditor().Reveal(match.End)
	state.selectFileResult(index)
	if state.notice == "Search result changed; run the search again" {
		state.notice = ""
	}
	return true
}

func (state *model) findAllCurrent() {
	state.cancelFileSearch()
	state.clearResultFocus()
	state.resultSelected = -1
	doc := state.space.Current()
	query := state.query.String()
	result, err := searchfiles.FindDocument(doc, query, state.searchOptions, 2000)
	if err != nil {
		state.discardFileResults()
		state.notice = err.Error()
		return
	}
	state.fileResults, state.resultOffset, state.resultSelected, state.resultDocument, state.resultQuery = result, 0, -1, doc, query
	state.resultDocuments, state.resultOpenDocuments = nil, false
	state.resultsOpen, state.searchOpen, state.goToOpen = true, false, false
	state.notice = fmt.Sprintf("%d matches in current document", len(result.Matches))
	if result.Truncated {
		state.notice = "Showing first 2000 matches in current document"
	}
}

func (state *model) findAllOpenDocuments() {
	state.cancelFileSearch()
	state.clearResultFocus()
	state.resultSelected = -1
	query := state.query.String()
	result := searchfiles.Result{}
	var sources []*document.Document
	for _, doc := range state.space.Documents {
		remaining := 2000 - len(result.Matches)
		if remaining == 0 {
			remaining = 1
		}
		part, err := searchfiles.FindDocument(doc, query, state.searchOptions, remaining)
		if err != nil {
			state.discardFileResults()
			state.notice = err.Error()
			return
		}
		result.FilesSearched++
		if len(result.Matches) == 2000 {
			if len(part.Matches) > 0 {
				result.Truncated = true
				break
			}
			continue
		}
		result.Matches = append(result.Matches, part.Matches...)
		for range part.Matches {
			sources = append(sources, doc)
		}
		if part.Truncated {
			result.Truncated = true
			break
		}
	}
	state.fileResults, state.resultOffset, state.resultSelected, state.resultQuery = result, 0, -1, query
	state.resultDocument, state.resultDocuments, state.resultOpenDocuments = nil, sources, true
	state.resultsOpen, state.searchOpen, state.goToOpen = true, false, false
	state.notice = fmt.Sprintf("%d matches in open documents", len(result.Matches))
	if result.Truncated {
		state.notice = "Showing first 2000 matches in open documents"
	}
}

func (state *model) replaceAllOpenDocuments() {
	state.cancelMacroForCommand()
	state.cancelFileSearch()
	plan, err := searchfiles.PreviewOpenReplace(state.space.Documents, state.query.CommittedString(), state.replacement.CommittedString(), state.searchOptions)
	if err != nil {
		state.notice = err.Error()
		return
	}
	if len(plan.Changes) == 0 {
		state.notice = "No open-document replacements needed"
		return
	}
	for _, change := range plan.Changes {
		if state.isReadOnly(change.Doc) {
			state.notice = "Read-only document in replacement plan"
			return
		}
	}
	for _, change := range plan.Changes {
		state.bookmarkLines(change.Doc)
	}
	applyError := plan.Apply(state.space.Documents)
	for _, change := range plan.Changes {
		state.bookmarkLines(change.Doc)
	}
	// Every document in the plan may have changed (including a partial apply);
	// consume each bound source exactly once before reporting.
	var historyErr error
	for _, change := range plan.Changes {
		if _, err := change.Doc.ChangeHistory(); err != nil && historyErr == nil {
			historyErr = err
		}
	}
	if applyError != nil {
		state.notice = applyError.Error()
		if historyErr != nil {
			state.notice += "; " + historyErr.Error()
		}
		return
	}
	state.discardFileResults()
	state.marks = searchMarkCache{}
	state.notice = fmt.Sprintf("Unsaved: %d matches replaced in %d open tabs", plan.Replacements, len(plan.Changes))
	if historyErr != nil {
		state.notice += "; " + historyErr.Error()
	}
}
