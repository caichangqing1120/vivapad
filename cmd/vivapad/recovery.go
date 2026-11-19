package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
)

func (state *model) restoreRecovery(explicitActive *document.Document) {
	if state.recoveryPath == "" {
		return
	}
	stored, err := recovery.Load(state.recoveryPath)
	if err != nil {
		state.notice = "Recovery unavailable: " + err.Error()
		return
	}
	if len(stored.Tabs) == 0 && !stored.Split {
		return
	}
	var recoveredActive *document.Document
	var historyErr error
	for index, tab := range stored.Tabs {
		var doc *document.Document
		if tab.Path != "" {
			var err error
			if tab.DiskEncoding.IsLegacy() {
				err = state.space.OpenWithEncoding(tab.Path, tab.DiskEncoding)
			} else {
				err = state.space.Open(tab.Path)
			}
			if err == nil {
				candidate := state.space.Current()
				// Matching bytes can belong to a different file after a path is
				// redirected. A draft already restored here also owns its buffer.
				if candidate.Path == tab.Path && !candidate.Dirty() && candidate.DiskDigest() == tab.DiskDigest {
					doc = candidate
				}
			}
		}
		if doc == nil {
			name := tab.Name
			if tab.Path != "" {
				name = "Recovered " + filepath.Base(tab.Path)
			}
			doc = document.New(name)
			state.space.AddRecovered(doc)
		}
		doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
		doc.Editor.Insert(tab.Content)
		doc.Editor.SetSelection(text.Range{Start: tab.SelectionStart, End: tab.SelectionEnd})
		// A caret already at the end does not separate recovery from the next keystroke.
		doc.Editor.BreakUndoGroup()
		doc.SetEncoding(tab.Encoding)
		if err := doc.RestoreLineEndings(tab.LineEnding, tab.LineEndings); err != nil {
			state.notice = "Recovery EOL unavailable: " + err.Error()
		}
		if tab.LanguageSelected {
			state.languages[doc] = tab.Language
			state.configureCode(doc)
		}
		doc.MarkRecovered()
		if err := state.applyFoldHeaders(doc, tab.FoldedHeaders); err != nil {
			state.notice = "Recovery fold unavailable: " + err.Error()
		}
		state.readOnly[doc] = tab.ReadOnly
		state.pinned[doc] = tab.Pinned
		if tab.TabColor > 0 {
			state.tabColors[doc] = tab.TabColor
		}
		state.paneOwner[doc] = tab.Pane
		// The restore edits are real undoable edits on the document's bound
		// source; consume them once and keep the explicit unavailable error.
		if _, err := doc.ChangeHistory(); err != nil && historyErr == nil {
			historyErr = err
		}
		if index == stored.Active {
			recoveredActive = doc
		}
	}
	if stored.Split {
		state.splitEnabled, state.activePane, state.splitRatio = true, stored.ActivePane, stored.Divider
		if recoveredActive != nil {
			state.paneDocs[stored.Tabs[stored.Active].Pane] = recoveredActive
		}
		for pane := range state.paneDocs {
			if state.documentIndex(state.paneDocs[pane]) >= 0 {
				continue
			}
			for _, doc := range state.space.Documents {
				if state.paneOwner[doc] == pane {
					state.paneDocs[pane] = doc
					break
				}
			}
			if state.documentIndex(state.paneDocs[pane]) < 0 {
				state.paneDocs[pane] = state.space.Current()
			}
		}
	} else if recoveredActive != nil {
		state.splitEnabled, state.activePane = false, 0
		state.paneDocs = [2]*document.Document{recoveredActive}
		if explicitActive == nil {
			state.focusViewRequested, state.focusViewArmed = true, true
		}
	}
	selected := recoveredActive
	if stored.Split {
		selected = state.paneDocs[state.activePane]
	}
	if explicitActive != nil {
		state.activePane, state.paneDocs[0] = 0, explicitActive
		selected = explicitActive
	}
	if index := state.documentIndex(selected); index >= 0 {
		state.space.Select(index)
	}
	if len(stored.Tabs) != 0 {
		state.notice = fmt.Sprintf("Recovered %d unsaved document(s)", len(stored.Tabs))
	}
	if historyErr != nil {
		state.notice += "; " + historyErr.Error()
	}
}

func (state *model) recoveryTab(doc *document.Document) recovery.Tab {
	selection := doc.Editor.Selection()
	mode, selected := state.languages[doc]
	return recovery.Tab{
		Path: doc.Path, Name: doc.Name, Content: doc.Editor.String(),
		Language: mode, LanguageSelected: selected, FoldedHeaders: state.foldHeaders(doc),
		Encoding: doc.Encoding(), DiskEncoding: doc.SavedEncoding(), LineEnding: doc.LineEnding(), LineEndings: doc.EOLSequence(), DiskDigest: doc.DiskDigest(),
		SelectionStart: selection.Start, SelectionEnd: selection.End,
		Pane: state.paneOwner[doc], ReadOnly: state.readOnly[doc], Pinned: state.isPinned(doc), TabColor: state.tabColors[doc],
	}
}

func (state *model) currentRecovery() recovery.State {
	result := recovery.State{Active: -1}
	if state.splitEnabled {
		result.Split, result.ActivePane, result.Divider = true, state.activePane, state.splitRatio
	}
	for _, doc := range state.space.Documents {
		if tab, ok := state.recoverySnapshot(doc); ok {
			if !result.Split {
				// The live owner still determines where this tab goes if split view returns.
				tab.Pane = 0
			}
			result.Tabs = append(result.Tabs, tab)
			if result.Active == -1 || doc == state.space.Current() {
				result.Active = len(result.Tabs) - 1
			}
		}
	}
	return result
}

func (state *model) recoverySnapshot(doc *document.Document) (recovery.Tab, bool) {
	// Text, selection and EOL labels must all come from the same side of preedit.
	if state.provisional.doc == doc {
		if tab := state.provisional.recoveryTab; tab != nil {
			return *tab, true
		}
		return recovery.Tab{}, false
	}
	if tab, tracked := state.legacyRecovery[doc]; tracked {
		if _, composing := doc.Editor.Composition(); composing {
			if tab != nil {
				return *tab, true
			}
			return recovery.Tab{}, false
		}
		// Editor commands can settle a legacy mark outside the native event handler.
		delete(state.legacyRecovery, doc)
	}
	if !doc.Dirty() {
		return recovery.Tab{}, false
	}
	return state.recoveryTab(doc), true
}

func recoveryEqual(first, second recovery.State) bool {
	if first.Active != second.Active || first.Split != second.Split || first.ActivePane != second.ActivePane || first.Divider != second.Divider || len(first.Tabs) != len(second.Tabs) {
		return false
	}
	for index, tab := range first.Tabs {
		other := second.Tabs[index]
		if tab.Path != other.Path || tab.Name != other.Name || tab.Content != other.Content || tab.Encoding != other.Encoding ||
			tab.Language != other.Language || tab.LanguageSelected != other.LanguageSelected || !slices.Equal(tab.FoldedHeaders, other.FoldedHeaders) ||
			tab.DiskEncoding != other.DiskEncoding || tab.LineEnding != other.LineEnding || tab.LineEndings != other.LineEndings ||
			tab.DiskDigest != other.DiskDigest || tab.SelectionStart != other.SelectionStart || tab.SelectionEnd != other.SelectionEnd ||
			tab.Pane != other.Pane || tab.ReadOnly != other.ReadOnly || tab.Pinned != other.Pinned || tab.TabColor != other.TabColor {
			return false
		}
	}
	return true
}

func (state *model) snapshotRecovery() {
	state.queueRecovery(false)
}

func (state *model) queueRecovery(retry bool) {
	if state.recoveryWriter == nil || state.closingAccepted {
		return
	}
	current := state.currentRecovery()
	if recoveryEqual(current, state.lastRecovery) {
		if retry {
			state.recoveryWriter.RetryFailed()
		}
		return
	}
	state.lastRecovery = current
	state.recoveryWriter.Submit(current)
}

func (state *model) observeRecoveryError() {
	if state.recoveryWriter == nil {
		return
	}
	if err := state.recoveryWriter.LastError(); err != nil {
		state.notice = "Recovery failed: " + err.Error()
	} else if strings.HasPrefix(state.notice, "Recovery failed: ") {
		state.notice = ""
	}
}
