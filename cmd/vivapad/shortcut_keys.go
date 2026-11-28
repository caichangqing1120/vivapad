package main

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/macro"
	"vivapad/internal/shortcuts"
)

func eventShortcut(event gfx.KeyEvent) (shortcuts.Chord, bool) {
	var binding shortcuts.Chord
	if event.Modifiers&primary() != 0 {
		binding.Modifiers |= shortcuts.Primary
	}
	if runtime.GOOS == "darwin" && event.Modifiers&gfx.ModifierControl != 0 {
		binding.Modifiers |= shortcuts.Control
	} else if runtime.GOOS != "darwin" && event.Modifiers&gfx.ModifierMeta != 0 {
		return binding, false
	}
	if event.Modifiers&gfx.ModifierAlt != 0 {
		binding.Modifiers |= shortcuts.Alt
	}
	if event.Modifiers&gfx.ModifierShift != 0 {
		binding.Modifiers |= shortcuts.Shift
	}
	switch event.Key {
	case gfx.KeyUnknown:
		binding.Key = strings.ToUpper(event.Text)
		if event.Text == " " {
			binding.Key = "Space"
		}
	case gfx.KeySpace:
		binding.Key = "Space"
	case gfx.KeyTab:
		binding.Key = "Tab"
	case gfx.KeyEnter:
		binding.Key = "Enter"
	case gfx.KeyEscape:
		binding.Key = "Escape"
	case gfx.KeyUp:
		binding.Key = "Up"
	case gfx.KeyDown:
		binding.Key = "Down"
	default:
		if event.Key >= gfx.KeyF1 && event.Key <= gfx.KeyF12 {
			binding.Key = "F" + strconv.Itoa(int(event.Key-gfx.KeyF1)+1)
		}
	}
	return binding, !event.Release && !binding.Empty() && binding.Valid()
}

var defaultShortcuts, _ = shortcuts.Load("", shortcuts.Defaults())

func findShortcut(registry *shortcuts.Registry, binding shortcuts.Chord) string {
	command := registry.Lookup(binding)
	if command == "" && runtime.GOOS != "darwin" && binding.Modifiers&shortcuts.Primary != 0 {
		binding.Modifiers ^= shortcuts.Primary | shortcuts.Control
		command = registry.Lookup(binding)
	}
	return command
}

func nativeEditShortcut(command string) bool {
	switch command {
	case "Cut", "Copy", "Paste", "Undo", "Redo", "Select All":
		return true
	}
	return false
}

func (state *model) dispatchPaletteShortcut(event gfx.KeyEvent) gfx.EventResult {
	if state.editorDialogOpen() || state.byteRangeOpen || state.markDialogOpen {
		return gfx.EventResult{}
	}
	binding, ok := eventShortcut(event)
	if !ok {
		return gfx.EventResult{}
	}
	command := findShortcut(state.shortcuts, binding)
	if command != shortcuts.CommandPalette && command != shortcuts.QuickSwitcher {
		return gfx.EventResult{}
	}
	if event.Repeat {
		return gfx.EventResult{Handled: true}
	}
	returnFocus := event.Target
	if palette := state.commandPalette; palette != nil {
		returnFocus = palette.returnFocus
		if palette.mode == command {
			state.closeCommandPalette()
			return gfx.EventResult{Handled: true, Repaint: true, Focus: returnFocus}
		}
	} else if returnFocus.IsZero() || state.menuOpen >= 0 || state.menuOverflowOpen || !state.statusPopup.kind.IsZero() {
		returnFocus = state.commandEditorFocusID()
	}
	state.openCommandPalette(command, returnFocus)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (state *model) dispatchShortcut(event gfx.KeyEvent) gfx.EventResult {
	binding, ok := eventShortcut(event)
	if !ok {
		return gfx.EventResult{}
	}
	if binding.Modifiers == shortcuts.Primary && len(binding.Key) == 1 && binding.Key[0] >= '1' && binding.Key[0] <= '9' && !isComparePreview(event.Target) {
		return state.selectNumberedTab(int(binding.Key[0]-'1'), event.Target)
	}
	command := findShortcut(state.shortcuts, binding)
	original := findShortcut(defaultShortcuts, binding)
	editorTarget := event.Target == editorID || event.Target == rightEditorID
	if !editorTarget && (nativeEditShortcut(command) || nativeEditShortcut(original)) {
		return gfx.EventResult{}
	}
	if command == "" {
		return gfx.EventResult{Handled: editorTarget && original != "" && original != shortcuts.MultiSelectExit}
	}
	if (command == nextDiffCommand || command == previousDiffCommand) && (state.comparison == nil || state.menuOpen >= 0) {
		return gfx.EventResult{}
	}
	if isComparePreview(event.Target) && command != nextDiffCommand && command != previousDiffCommand {
		return gfx.EventResult{}
	}
	if command == postItCommand && (state.requestDecoration == nil || state.distractionFree || event.Repeat) {
		return gfx.EventResult{Handled: true}
	}
	switch command {
	case "Word Completion", "Path Completion", "Duplicate Lines Up", textDirectionRTL, textDirectionLTR,
		shortcuts.MultiSelectNext, shortcuts.MultiSelectAll, shortcuts.MultiSelectSkip, shortcuts.MultiSelectUndo, shortcuts.MultiSelectExit:
		if !editorTarget {
			return gfx.EventResult{}
		}
	}
	fromPane := state.activePane
	if editorTarget {
		state.activatePane(paneForTarget(event.Target))
	}
	doc := state.paneDocument(state.activePane)
	result := gfx.EventResult{Handled: true, Repaint: true}
	if nativeEditShortcut(command) && doc.LargeFile() == nil && !state.isReadOnly(doc) {
		return gfx.EventResult{}
	}
	if command == original {
		var code components.CodeEditorCommand
		switch command {
		case "Indent Lines":
			code = components.CodeEditorIndent
		case "Unindent Lines":
			code = components.CodeEditorOutdent
		case "Duplicate Lines Up":
			code = components.CodeEditorDuplicateLinesUp
		case "Move Up Current Line":
			code = components.CodeEditorMoveLinesUp
		case "Move Down Current Line":
			code = components.CodeEditorMoveLinesDown
		case "Duplicate Line":
			if binding.Key == "Down" {
				code = components.CodeEditorDuplicateLinesDown
			}
		case "Delete Line":
			if binding.Key == "K" {
				code = components.CodeEditorDeleteLines
			}
		}
		if code != components.CodeEditorNoCommand {
			if doc.LargeFile() != nil {
				state.notice = document.ErrLargeFileReadOnly.Error()
			} else if state.isReadOnly(doc) {
				state.notice = "Read-only document"
			} else {
				state.applyCodeCommand(doc, code)
				state.recordMacro(doc, macro.Step{Kind: macro.CodeStep, Code: code})
			}
			return result
		}
	}
	switch command {
	case shortcuts.MultiSelectNext, shortcuts.MultiSelectAll:
		state.command(command + " / " + multiSelectWholeWordLabel)
	case shortcuts.MultiSelectExit:
		if !state.exitMultiSelection(state.activePane) {
			return gfx.EventResult{}
		}
	case "Search Results Window":
		if state.distractionFree {
			return result
		}
		if !state.resultsOpen || len(state.fileResults.Matches) == 0 {
			state.notice = "No search results"
			return result
		}
		state.searchOpen, state.goToOpen = false, false
		state.menuOpen = -1
		state.resultFocusMode = event.Target != resultsPaneID && event.Target.Scope() != resultScope
		result.Focus = state.commandEditorFocusID()
		if state.resultFocusMode {
			result.Focus = resultsPaneID
		}
	case "Incremental Search":
		state.cancelMacroForCommand()
		state.openIncrementalSearch(state.incrementOpen && event.Target == incrementQueryID)
	case "Find Next", "Find Previous":
		if state.incrementOpen && event.Target == incrementQueryID {
			state.findIncremental(command == "Find Previous", true)
		} else {
			state.command(command)
		}
	case "Toggle Macro Recording":
		if state.macroRecording {
			state.command("Stop Recording")
		} else {
			state.command("Record Macro")
		}
	default:
		if !state.runSavedShortcut(command) {
			state.command(command)
		}
		switch command {
		case "Find...", "Replace...", "Find in Files...", "Replace in Files...":
			result.Focus = queryID
		case "Go to Line...":
			result.Focus = goToFieldID
		case "Next Tab", "Previous Tab", nextDiffCommand, previousDiffCommand:
			result.Focus = state.commandEditorFocusID()
		case "Next Search Result", "Previous Search Result":
			if state.splitEnabled && paneForTarget(event.Target) == fromPane && state.activePane != fromPane {
				result.Focus = state.commandEditorFocusID()
			}
		}
	}
	return result
}

func (state *model) runSavedShortcut(command string) bool {
	if name, ok := strings.CutPrefix(command, "macro:"); ok {
		for _, entry := range state.namedMacros {
			if entry.Name == name {
				state.menuOpen = -1
				state.playMacroSteps(entry.Steps, 1, false)
				break
			}
		}
		return true
	}
	if name, ok := strings.CutPrefix(command, "run:"); ok {
		for _, entry := range state.namedRuns {
			if entry.Name == name {
				state.menuOpen = -1
				if err := state.launchRunTemplate(entry.Template); err != nil {
					state.notice = "Run failed: " + err.Error()
				}
				state.requestEditorCommandFocus()
				break
			}
		}
		return true
	}
	return false
}
