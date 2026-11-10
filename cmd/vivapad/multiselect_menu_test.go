package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestMultiSelectEditMenusReachAllOptionsAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("cat cat CAT")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	clickID(t, driver, menuScope.At(1))
	for _, index := range []int{25, 26, 27, 28} {
		if bounds := revealEditMenuItem(t, driver, index); bounds.Y < menuHeight || bounds.Y+bounds.Height > 400-statusHeight {
			t.Fatalf("multi-select row %d clipped: %+v", index, bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(25))
	for index := range 4 {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == multiSelectAllScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("All option %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("All option %d missing", index)
		}
	}
	clickID(t, driver, multiSelectAllScope.At(2))
	if got := state.multiSelections[0].ranges; len(got) != 3 || state.multiSelectOptions != (document.SearchOptions{WholeWord: true}) {
		t.Fatalf("All whole-word selection = %+v, flags=%+v", got, state.multiSelectOptions)
	}
	clickID(t, driver, menuScope.At(1))
	revealEditMenuItem(t, driver, 27)
	clickID(t, driver, menuItemScope.At(27))
	if len(state.multiSelections[0].ranges) != 2 {
		t.Fatalf("Undo latest from Edit menu = %+v", state.multiSelections[0].ranges)
	}
	clickID(t, driver, menuScope.At(1))
	revealEditMenuItem(t, driver, 28)
	clickID(t, driver, menuItemScope.At(28))
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 0, End: 3}, {Start: 8, End: 11}}) {
		t.Fatalf("Skip using previous flags = %+v", got)
	}
	clickID(t, driver, menuScope.At(1))
	revealEditMenuItem(t, driver, 26)
	clickID(t, driver, menuItemScope.At(26))
	clickID(t, driver, multiSelectNextScope.At(0))
	if len(state.multiSelections[0].ranges) != 3 || doc.Editor.String() != "cat cat CAT" {
		t.Fatalf("Next menu selection = %+v, text=%q", state.multiSelections[0].ranges, doc.Editor.String())
	}
}

func TestCaptureMultiSelectWhenRequested(t *testing.T) {
	directory := os.Getenv("VIVAPAD_MULTISELECT_SHOTS")
	if directory == "" {
		t.Skip("set VIVAPAD_MULTISELECT_SHOTS to capture menu and selections")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat CAT cat\ncat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(1))
	revealEditMenuItem(t, session.Driver(), 25)
	clickID(t, session.Driver(), menuItemScope.At(25))
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("multi-select menu layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(directory, "multiselect-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), multiSelectAllScope.At(0))
	if len(state.multiSelections[0].ranges) != 4 {
		t.Fatalf("multi-select result = %+v", state.multiSelections[0].ranges)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("multi-select editor layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(directory, "multiselect-ranges-compact.png")); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Type("X"); err != nil || doc.Editor.String() != "X X X\nX" {
		t.Fatalf("multi-caret screenshot edit = %q, %v", doc.Editor.String(), err)
	}
	state.invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("multi-caret layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(directory, "multiselect-carets-compact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSelectMenuReturnsFocusForImmediateTyping(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	clickID(t, driver, menuScope.At(1))
	revealEditMenuItem(t, driver, 25)
	clickID(t, driver, menuItemScope.At(25))
	clickID(t, driver, multiSelectAllScope.At(0))
	if err := driver.Type("X"); err != nil || doc.Editor.String() != "X X" {
		t.Fatalf("typing after Multi-select All = %q, %v", doc.Editor.String(), err)
	}
}
