package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestFigmaNavigationArrowsPointInTheirActionDirection(t *testing.T) {
	for _, test := range []struct {
		name string
		icon gfx.Icon
		want []gfx.PathCommand
	}{
		{"left", leftIcon, []gfx.PathCommand{gfx.MoveTo(10, 4), gfx.LineTo(6, 8), gfx.LineTo(10, 12)}},
		{"right", rightIcon, []gfx.PathCommand{gfx.MoveTo(6, 12), gfx.LineTo(10, 8), gfx.LineTo(6, 4)}},
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
	if enabled, disabled := classicPalette.toolbarTint(true), classicPalette.toolbarTint(false); enabled == disabled {
		t.Fatalf("enabled and disabled toolbar icons have the same tint: %+v", enabled)
	}
}
