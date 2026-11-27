package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestChangeHistoryNavigationReplacesActiveSelectionMode(t *testing.T) {
	for _, mode := range []string{"multiple", "rectangle"} {
		for _, command := range []string{"Next Change", "Previous Change"} {
			t.Run(mode+"/"+command, func(t *testing.T) {
				state, driver := startDriver(t)
				path := filepath.Join(t.TempDir(), "selection.txt")
				if err := os.WriteFile(path, []byte("cat\ncat\nlast\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := state.space.Open(path); err != nil {
					t.Fatal(err)
				}
				doc := state.space.Current()
				state.showInActivePane(doc)
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, editorID)
				doc.Editor.MoveTo(8, false)
				doc.Editor.Insert("X")
				if mode == "multiple" {
					doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
					state.multiSelectAll(document.SearchOptions{})
					if len(state.multiSelections[0].ranges) != 2 {
						t.Fatal("fixture did not select both cat occurrences")
					}
				} else {
					doc.Editor.MoveTo(0, false)
					state.command("Begin/End Select in Column Mode")
					doc.Editor.MoveTo(5, false)
					state.command("Begin/End Select in Column Mode")
					if !state.rectangles[0].active {
						t.Fatal("fixture did not activate the rectangle")
					}
				}
				state.command(command)
				if doc.Editor.Caret() != 8 || doc.Editor.Selection() != (text.Range{Start: 8, End: 8}) {
					t.Fatalf("navigation selection = %+v, caret %d", doc.Editor.Selection(), doc.Editor.Caret())
				}
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				if err := driver.Type("Q"); err != nil {
					t.Fatal(err)
				}
				if got := doc.Editor.String(); got != "cat\ncat\nQXlast\n" {
					t.Fatalf("typing after navigation used the old %s selection: %q", mode, got)
				}
				state.command("Undo")
				if got := doc.Editor.String(); got != "cat\ncat\nXlast\n" {
					t.Fatalf("Undo crossed the navigation boundary: %q", got)
				}
			})
		}
	}
}

func TestChangeHistoryColorsMatchPinnedClassicTheme(t *testing.T) {
	// Notepad++ dd40fe4, PowerEditor/src/stylers.model.xml:1973-1976.
	want := [4]gfx.Color{
		gfx.RGB(0x40, 0xa0, 0xbf),
		gfx.RGB(0x00, 0xa0, 0x00),
		gfx.RGB(0xff, 0x80, 0x00),
		gfx.RGB(0xa0, 0xc0, 0x00),
	}
	got := classicPalette.changeMarginColors()
	if got != want {
		t.Fatalf("classic change colors = %v, want %v", got, want)
	}
}

func TestChangeHistoryNavigationWithoutTargetPreservesSelectionModes(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		for _, mode := range []string{"multiple", "rectangle"} {
			state := openedDocument(t, "cat\ncat\nlast\n")
			doc := state.space.Current()
			if unavailable {
				doc.Editor = text.NewEditor("cat\ncat\nlast\n")
			}
			if mode == "multiple" {
				doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
				state.multiSelectAll(document.SearchOptions{})
			} else {
				doc.Editor.MoveTo(0, false)
				state.command("Begin/End Select in Column Mode")
				doc.Editor.MoveTo(5, false)
				state.command("Begin/End Select in Column Mode")
			}
			rectangle, multiple := state.rectangles, state.multiSelections
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			for _, command := range []string{"Next Change", "Previous Change"} {
				state.command(command)
				if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || state.rectangles != rectangle || !reflect.DeepEqual(state.multiSelections, multiple) {
					t.Fatalf("%s discarded the %s selection without a target (unavailable=%v)", command, mode, unavailable)
				}
			}
		}
	}
}

func TestChangeHistoryUnavailableRightPanePointerIsInert(t *testing.T) {
	for _, cause := range []string{"replaced", "gap"} {
		t.Run(cause, func(t *testing.T) {
			state, driver := startDriver(t)
			left := state.space.Current()
			left.Editor.Insert("left untouched")
			right := state.space.NewDocument()
			right.Editor.Insert("right untouched")
			state.showInActivePane(right)
			state.command("Split View")
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, rightEditorID)
			if cause == "replaced" {
				right.Editor = text.NewEditor("right replacement")
			} else {
				for range 300 {
					right.Editor.Insert("x")
					right.Editor.SetSavePoint()
				}
			}
			before := right.Editor.String()
			right.Editor.SetSelection(text.Range{Start: 1, End: 3})
			selection := right.Editor.Selection()
			clickID(t, driver, menuScope.At(2))
			clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[2], "Change History")))
			hits := submenuHits(t, driver)
			for index, label := range []string{"Go to Next Change", "Go to Previous Change"} {
				if !hits[label].Disabled {
					t.Fatalf("%s is not disabled for %s", label, cause)
				}
				clickID(t, driver, changeHistoryScope.At(index))
				if !state.changeHistoryMenuOpen || state.menuOpen != 2 {
					t.Fatal("disabled pointer click dispatched and dismissed the submenu")
				}
				if state.activePane != 1 || state.space.Current() != right || right.Editor.String() != before || right.Editor.Selection() != selection || left.Editor.String() != "left untouched" {
					t.Fatal("disabled pointer click changed a document, selection or active pane")
				}
				if state.notice == "" || strings.Contains(state.notice, "No changes") {
					t.Fatalf("unavailable reason was lost: %q", state.notice)
				}
			}
		})
	}
}

func TestChangeHistoryNavigationClearsOnlyActivePaneSelections(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(0, false)
	state.command("Clone to Other View")
	state.activatePane(1)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: 0, caret: 2, value: doc.Editor.String(), active: true}
	state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), ranges: []text.Range{{Start: 0, End: 1}}, active: true}
	state.selectAnchors[0] = selectAnchor{doc: doc, index: 2, value: doc.Editor.String()}
	state.selectAnchors[1] = selectAnchor{doc: doc, index: 0, value: doc.Editor.String()}
	inactiveRectangle, inactiveMultiple, inactiveAnchor := state.rectangles[0], state.multiSelections[0], state.selectAnchors[0]
	state.command("Next Change")
	if state.activePane != 1 || state.selectAnchors[1] != (selectAnchor{}) {
		t.Fatal("successful navigation kept the active selection anchor or changed panes")
	}
	if state.rectangles[0] != inactiveRectangle || !reflect.DeepEqual(state.multiSelections[0], inactiveMultiple) || state.selectAnchors[0] != inactiveAnchor {
		t.Fatal("navigation changed an inactive pane's selection state")
	}
}
