package main

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestToolbarIconsShareOneVisualGrid(t *testing.T) {
	icons := []ui.Icon{newIcon, newAccentIcon, openIcon, saveIcon, undoIcon, redoIcon, cutIcon, copyIcon, pasteIcon, searchIcon, closeIcon, fullscreenRestoreIcon, leftIcon, rightIcon, checkIcon, pinIcon}
	for index, icon := range icons {
		if icon.ViewBox != 24 || icon.Stroke != 2 || icon.Cap != ui.CapRound || icon.Join != ui.JoinRound || len(icon.Commands) == 0 {
			t.Errorf("icon %d does not use the shared 24px/2px rounded grid: %+v", index, icon)
		}
	}
}

func TestNewIconSeparatesDocumentFromAccent(t *testing.T) {
	if len(newIcon.Commands) != 10 || len(newAccentIcon.Commands) != 4 {
		t.Fatalf("New icon should separate document outline from branded plus, got %d and %d commands", len(newIcon.Commands), len(newAccentIcon.Commands))
	}
}

func TestToolbarTintUsesVivaBrandSparingly(t *testing.T) {
	for _, test := range []struct {
		id      ui.ID
		canUndo bool
		canRedo bool
		dirty   bool
		want    ui.Color
	}{
		{id: newID, want: ui.RGB(24, 24, 27)},
		{id: openID, want: ui.RGB(24, 24, 27)},
		{id: copyID, want: ui.RGB(24, 24, 27)},
		{id: pasteID, want: ui.RGB(24, 24, 27)},
		{id: saveID, want: ui.RGB(24, 24, 27)},
		{id: saveID, dirty: true, want: palette.blue},
		{id: undoID, want: ui.RGB(151, 154, 160)},
		{id: undoID, canUndo: true, want: ui.RGB(24, 24, 27)},
		{id: redoID, want: ui.RGB(151, 154, 160)},
		{id: redoID, canRedo: true, want: ui.RGB(24, 24, 27)},
	} {
		if got := toolbarTint(test.id, test.canUndo, test.canRedo, test.dirty); got != test.want {
			t.Errorf("icon %v tint = %+v, want %+v", test.id, got, test.want)
		}
	}
}
