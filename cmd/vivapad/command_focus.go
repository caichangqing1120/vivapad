package main

import (
	"slices"

	"github.com/vivaui/viva/ui"
)

func (state *model) editorCommandFocusBlocked() bool {
	return state.menuOpen >= 0 || state.saveSessionOptionsOpen || state.columnDialogOpen ||
		state.preferencesOpen || state.runDialogOpen || state.summaryOpen || state.windowsDialogOpen ||
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
	returnsToEditor := state.textMutation(command) ||
		slices.Contains(menuCommands[1][:], command) || slices.Contains(menuCommands[6][:], command) ||
		slices.Contains(symbolCommands[:], command) || slices.Contains(copyPathsCommands[:], command) ||
		slices.Contains(viewTabCommands[:], command) || foldLevelForCommand(command)
	if _, language := languageExtension(command); language {
		returnsToEditor = true
	}
	switch command {
	case "Record Macro", "Stop Recording", "Clear Macro",
		"Read-Only on Current Document", "Read-Only for All Documents", "Clear Read-Only for All Documents",
		"Show Line Numbers", "Word Wrap", "Zoom In", "Zoom Out", "Restore Default Zoom",
		"Split View", "Move to Other View", "Clone to Other View", "Focus on Another View",
		"Fold All", "Unfold All", "Fold Current Level", "Unfold Current Level", "Hide Lines",
		"Synchronize Vertical Scrolling", "Synchronize Horizontal Scrolling", "Synchronize Zoom Across Views":
		returnsToEditor = true
	}
	if !returnsToEditor {
		return
	}
	state.focusViewRequested, state.focusViewArmed = true, false
	state.invalidator.Invalidate()
}

func (state *model) paintEditorCommandFocus(canvas *ui.Canvas) {
	if !state.focusViewRequested {
		return
	}
	focus := editorID
	if state.splitEnabled && state.activePane == 1 {
		focus = rightEditorID
	}
	requested := canvas.FocusRequest()
	if state.editorCommandFocusBlocked() || requested != (ui.ID{}) && requested != focus {
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
