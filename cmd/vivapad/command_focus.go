package main

import (
	"slices"

	"github.com/vivaui/viva/gfx"
)

func (state *model) editorCommandFocusBlocked() bool {
	return state.menuOpen >= 0 || state.saveSessionOptionsOpen || state.columnDialogOpen ||
		state.preferencesOpen || state.runDialogOpen || state.summaryOpen || state.helpMode != noHelp || state.webSearchOpen || state.windowsDialogOpen ||
		state.macroSaveOpen || state.macroRunOpen || state.hashDialogOpen || state.dateTimeDialogOpen ||
		state.renameDoc != nil || state.renamePlan != nil || state.pendingTrash != nil ||
		state.pendingClose >= 0 || state.pendingReload >= 0 || state.filesDialogOpen || state.replaceConfirmOpen
}

func (state *model) restoreEditorCommandFocus(command string) {
	if command == "" || state.editorCommandFocusBlocked() {
		return
	}
	if command == "Record Macro" && !state.macroRecording {
		return
	}
	// The change-history commands act on the editor even though they sit in the
	// Search menu, so they hand focus back like the edit commands do.
	historyCommand := command == "Next Change" || command == "Previous Change" || command == "Clear Change History"
	returnsToEditor := state.textMutation(command) || historyCommand ||
		slices.Contains(menuCommands[1][:], command) || slices.Contains(menuCommands[6][:], command) ||
		slices.Contains(symbolCommands[:], command) || slices.Contains(copyPathsCommands[:], command) ||
		slices.Contains(viewTabCommands[:], command) || foldLevelForCommand(command)
	if _, language := languageExtension(command); language {
		returnsToEditor = true
	}
	switch command {
	case "Record Macro", "Stop Recording", "Clear Macro", "Print...", "Print Now", "Generate from selection into clipboard",
		openCommandPromptCommand, openPowerShellCommand, webSearchCommand, parameterHintCommand, previousHintCommand, nextHintCommand,
		"Read-Only on Current Document", "Read-Only for All Documents", "Clear Read-Only for All Documents",
		"Monitoring (tail -f)",
		"Show Line Numbers", "Word Wrap", "Zoom In", "Zoom Out", "Restore Default Zoom", "Toggle Full Screen Mode", distractionFreeCommand,
		"Split View", "Move to Other View", "Clone to Other View", "Focus on Another View",
		"Fold All", "Unfold All", "Fold Current Level", "Unfold Current Level", "Hide Lines",
		"Synchronize Vertical Scrolling", "Synchronize Horizontal Scrolling", "Synchronize Zoom Across Views":
		returnsToEditor = true
	}
	if !returnsToEditor {
		return
	}
	state.requestEditorCommandFocus()
}

func (state *model) requestEditorCommandFocus() {
	if state.editorCommandFocusBlocked() {
		return
	}
	state.focusViewRequested, state.focusViewArmed = true, false
	state.invalidator.Invalidate()
}

func (state *model) requestNativeChooserFocus() {
	if state.editorCommandFocusBlocked() {
		return
	}
	state.focusViewRequested, state.focusViewArmed = true, true
	state.invalidator.Invalidate()
}

func (state *model) focusEditorOnTabClick(canvas *gfx.Canvas) {
	if state.editorCommandFocusBlocked() {
		return
	}
	// Carry focus out of this click so native input does not wait for a redraw.
	state.focusViewRequested, state.focusViewArmed = false, false
	canvas.RequestFocus(state.commandEditorFocusID())
}

func (state *model) commandEditorFocusID() gfx.ID {
	if state.splitEnabled && state.activePane == 1 {
		return rightEditorID
	}
	return editorID
}

func (state *model) paintEditorCommandFocus(canvas *gfx.Canvas) {
	if !state.focusViewRequested {
		return
	}
	focus := state.commandEditorFocusID()
	requested := canvas.FocusRequest()
	if state.editorCommandFocusBlocked() || requested != (gfx.ID{}) && requested != focus {
		state.focusViewRequested, state.focusViewArmed = false, false
		return
	}
	if state.focusViewArmed {
		canvas.RequestFocus(focus)
		state.focusViewRequested, state.focusViewArmed = false, false
	} else {
		state.focusViewArmed = true
		state.invalidator.Invalidate()
	}
}
