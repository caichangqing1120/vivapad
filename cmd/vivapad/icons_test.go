package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestFigmaNavigationArrowsPointInTheirActionDirection(t *testing.T) {
	for _, test := range []struct {
		name string
		icon ui.Icon
		want []ui.PathCommand
	}{
		{"left", leftIcon, []ui.PathCommand{ui.MoveTo(10, 4), ui.LineTo(6, 8), ui.LineTo(10, 12)}},
		{"right", rightIcon, []ui.PathCommand{ui.MoveTo(6, 12), ui.LineTo(10, 8), ui.LineTo(6, 4)}},
	} {
		if !reflect.DeepEqual(test.icon.Commands, test.want) {
			t.Errorf("%s navigation lost its Figma rotation: %+v", test.name, test.icon.Commands)
		}
	}
}

func TestFigmaToolbarRetainsEditingCommandsAndFitsCompactWindows(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("alpha\nbeta\nalpha")
	doc.Editor.ClearHistory()
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, hit := range session.Snapshot().Hits {
		if (hit.ID == undoID || hit.ID == redoID) && !hit.Disabled {
			t.Fatalf("unavailable history command remains enabled: %s", hit.Label)
		}
	}
	if err := session.ClickLabel("Find"); err != nil || !state.searchOpen {
		t.Fatalf("Find icon did not open the existing search panel: %v", err)
	}
	if err := session.Driver().Move(620, 350); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct {
		name   string
		width  int
		height int
	}{
		{"desktop", 1280, 800},
		{"small", 800, 600},
		{"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s icon controls overlap: %+v", size.name, problems)
		}
		if output := os.Getenv("VIVAPAD_ICON_SHOTS"); output != "" {
			if err := session.Screenshot(filepath.Join(output, "figma-icons-"+size.name+".png")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := session.ClickLabel("New"); err != nil || state.space.Current() == doc || doc.Editor.String() != "alpha\nbeta\nalpha" {
		t.Fatalf("New icon must create a document without altering its source: %v", err)
	}
}

func TestToolbarTintKeepsDisabledCommandsDistinct(t *testing.T) {
	for _, test := range []struct {
		id      ui.ID
		canUndo bool
		canRedo bool
		want    ui.Color
	}{
		{id: newID, want: ui.RGB(52, 61, 67)},
		{id: openID, want: ui.RGB(52, 61, 67)},
		{id: copyID, want: ui.RGB(52, 61, 67)},
		{id: pasteID, want: ui.RGB(52, 61, 67)},
		{id: saveID, want: ui.RGB(52, 61, 67)},
		{id: undoID, want: ui.RGB(166, 175, 182)},
		{id: undoID, canUndo: true, want: ui.RGB(52, 61, 67)},
		{id: redoID, want: ui.RGB(166, 175, 182)},
		{id: redoID, canRedo: true, want: ui.RGB(52, 61, 67)},
	} {
		if got := toolbarTint(test.id, test.canUndo, test.canRedo); got != test.want {
			t.Errorf("icon %v tint = %+v, want %+v", test.id, got, test.want)
		}
	}
}
