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
				if candidate.DiskDigest() == tab.DiskDigest {
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
		doc.SetEncoding(tab.Encoding)
		doc.SetLineEnding(tab.LineEnding)
		if tab.LanguageSelected {
			state.languages[doc] = tab.Language
			state.configureCode(doc)
		}
		if tab.LineEndings != "" {
			if err := doc.RestoreEOLSequence(tab.LineEndings); err != nil {
				state.notice = "Recovery EOL unavailable: " + err.Error()
			}
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
}

func (state *model) currentRecovery() recovery.State {
	result := recovery.State{Active: -1}
	if state.splitEnabled {
		result.Split, result.ActivePane, result.Divider = true, state.activePane, state.splitRatio
	}
	for _, doc := range state.space.Documents {
		if !doc.Dirty() {
			continue
		}
		selection := doc.Editor.Selection()
		mode, selected := state.languages[doc]
		result.Tabs = append(result.Tabs, recovery.Tab{
			Path: doc.Path, Name: doc.Name, Content: doc.Editor.String(),
			Language: mode, LanguageSelected: selected, FoldedHeaders: state.foldHeaders(doc),
			Encoding: doc.Encoding(), DiskEncoding: doc.SavedEncoding(), LineEnding: doc.LineEnding(), LineEndings: doc.EOLSequence(), DiskDigest: doc.DiskDigest(),
			SelectionStart: selection.Start, SelectionEnd: selection.End,
			Pane: state.paneOwner[doc], ReadOnly: state.isReadOnly(doc), Pinned: state.isPinned(doc), TabColor: state.tabColors[doc],
		})
		if result.Active == -1 || doc == state.space.Current() {
			result.Active = len(result.Tabs) - 1
		}
	}
	return result
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
	if state.recoveryWriter == nil || state.closingAccepted {
		return
	}
	current := state.currentRecovery()
	if recoveryEqual(current, state.lastRecovery) {
		return
	}
	state.lastRecovery = current
	if err := state.recoveryWriter.Submit(current); err != nil {
		state.notice = "Recovery failed: " + err.Error()
		state.invalidator.Invalidate()
	}
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
