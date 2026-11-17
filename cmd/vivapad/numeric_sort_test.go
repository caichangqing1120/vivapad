package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestNumericSortSubmenuReachesEveryCommandAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("item10\nitem2\nitem1\n")
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	seen := make([]bool, len(lineCommands))
	for step := 0; step < len(lineCommands); step++ {
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID.Scope() != lineScope {
				continue
			}
			if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
				t.Fatalf("line menu row outside usable height: %+v", hit.Bounds)
			}
			for index := range seen {
				if hit.ID == lineScope.At(index) {
					seen[index] = true
				}
			}
		}
		if seen[len(seen)-1] {
			break
		}
		clickID(t, driver, lineNextID)
	}
	for index, visible := range seen {
		if !visible {
			t.Fatalf("line command %d (%s) cannot be reached", index, lineCommands[index])
		}
	}
	clickID(t, driver, lineScope.At(13))
	if got := doc.Editor.String(); got != "\nitem1\nitem2\nitem10" {
		t.Fatalf("integer-aware ascending menu = %q", got)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestNumericSortWheelAndReadOnlyErrorPreserveDocument(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("3.0\n- invalid\n1.0\n")
	doc.Editor.ClearHistory()
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	if err := driver.Scroll(gfx.ScrollEvent{X: 390, Y: 260, DeltaY: 240}); err != nil {
		t.Fatal(err)
	}
	if state.lineMenuOffset == 0 {
		t.Fatal("wheel did not scroll Line Operations")
	}
	state.command("Sort As Decimals (Dot) Ascending")
	if doc.Editor.String() != "3.0\n- invalid\n1.0\n" || !strings.Contains(state.notice, "line 2") || doc.Editor.CanUndo() {
		t.Fatalf("invalid decimal sort changed text: %q notice=%q", doc.Editor.String(), state.notice)
	}
	state.command("Read-Only on Current Document")
	state.command("Sort By Length Ascending")
	if doc.Editor.String() != "3.0\n- invalid\n1.0\n" || state.notice != "Read-only document" {
		t.Fatalf("read-only sort = %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestNumericSortClearsWholeBufferMarkersAndKeepsActivePane(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("10\n2\n1\n")
	right.Editor.MoveTo(0, false)
	state.toggleBookmark()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Sort As Integers Ascending")
	if left.Editor.String() != "left" || right.Editor.String() != "1\n2\n10\n" || state.activePane != 1 {
		t.Fatalf("split numeric sort = left:%q right:%q pane:%d", left.Editor.String(), right.Editor.String(), state.activePane)
	}
	if marks := state.bookmarkLines(right); len(marks) != 0 {
		t.Fatalf("sorted bookmark lines = %v", marks)
	}
	state.command("Undo")
	if right.Editor.String() != "10\n2\n1\n" || len(state.bookmarkLines(right)) != 0 {
		t.Fatalf("numeric sort undo = %q marks=%v", right.Editor.String(), state.bookmarkLines(right))
	}
}

func TestNumericSortCommandsRouteAllEightDirections(t *testing.T) {
	for _, test := range []struct {
		command, before, want string
	}{
		{"Sort As Integers Ascending", "n10\nn2", "n2\nn10"},
		{"Sort As Integers Descending", "n2\nn10", "n10\nn2"},
		{"Sort As Decimals (Comma) Ascending", "2,5\n1,5", "1,5\n2,5"},
		{"Sort As Decimals (Comma) Descending", "1,5\n2,5", "2,5\n1,5"},
		{"Sort As Decimals (Dot) Ascending", "2.5\n1.5", "1.5\n2.5"},
		{"Sort As Decimals (Dot) Descending", "1.5\n2.5", "2.5\n1.5"},
		{"Sort By Length Ascending", "long\n短", "短\nlong"},
		{"Sort By Length Descending", "短\nlong", "long\n短"},
	} {
		t.Run(test.command, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(test.before)
			doc.Editor.ClearHistory()
			state.command(test.command)
			if got := doc.Editor.String(); got != test.want {
				t.Fatalf("%s = %q, want %q", test.command, got, test.want)
			}
			if !doc.Editor.Undo() || doc.Editor.String() != test.before {
				t.Fatal("sort menu command did not undo in one step")
			}
		})
	}
}
