package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestBeginEndSelectForwardBackwardAndNoUndo(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a猫bcd")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select")
	if !state.menuSelected("Begin/End Select") || doc.Editor.Selection() != (text.Range{Start: 1, End: 1}) {
		t.Fatalf("first invocation changed selection: %+v", doc.Editor.Selection())
	}
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select")
	if state.menuSelected("Begin/End Select") || doc.Editor.Selection() != (text.Range{Start: 1, End: 4}) || doc.Editor.CanUndo() {
		t.Fatalf("forward selection = %+v undo=%v", doc.Editor.Selection(), doc.Editor.CanUndo())
	}
	doc.Editor.MoveTo(5, false)
	state.command("Begin/End Select")
	doc.Editor.MoveTo(1, false)
	state.command("Begin/End Select")
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 5}) || doc.Editor.Caret() != 1 || doc.Editor.CanUndo() {
		t.Fatalf("backward selection = %+v caret=%d", doc.Editor.Selection(), doc.Editor.Caret())
	}
}

func TestBeginEndSelectShortcutAndCompactMenu(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("abcdef")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(1, false)
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'b'); err != nil || !handled {
		t.Fatalf("start shortcut handled=%v err=%v", handled, err)
	}
	clickID(t, driver, menuScope.At(1))
	bounds := revealEditMenuItem(t, driver, 15)
	if bounds.Y+bounds.Height > 400-statusHeight {
		t.Fatalf("selection menu bounds = %+v", bounds)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(15) && !hit.Checked {
			t.Fatalf("selection menu is unchecked: %+v", hit)
		}
	}
	doc.Editor.MoveTo(4, false)
	clickID(t, driver, menuItemScope.At(15))
	if doc.Editor.Selection() != (text.Range{Start: 1, End: 4}) || state.menuSelected("Begin/End Select") {
		t.Fatalf("menu did not finish shortcut selection: %+v", doc.Editor.Selection())
	}
}

func TestBeginEndSelectAnchorTracksNativeAndCommandEdits(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abc")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	doc.Editor.MoveTo(0, false)
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(4, false)
	state.command("Begin/End Select")
	if doc.Editor.String() != "Xabc" || doc.Editor.Selection() != (text.Range{Start: 3, End: 4}) {
		t.Fatalf("native insertion moved anchor to wrong position: %+v text=%q", doc.Editor.Selection(), doc.Editor.String())
	}
	doc.Editor.MoveTo(3, false)
	state.command("Begin/End Select")
	doc.Editor.MoveTo(0, false)
	state.command("Delete")
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	if doc.Editor.String() != "abc" || doc.Editor.Selection() != (text.Range{Start: 2, End: 2}) {
		t.Fatalf("command deletion did not shift anchor: %+v text=%q", doc.Editor.Selection(), doc.Editor.String())
	}
}

func TestBeginEndSelectAnchorClampsWhenNativeEditRemovesIt(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("abcdef")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(3, false)
	state.command("Begin/End Select")
	doc.Editor.SetSelection(text.Range{Start: 1, End: 5})
	if err := driver.Key(gfx.KeyDelete, 0); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	if doc.Editor.String() != "af" || doc.Editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("deleted anchor selection = %+v, text=%q", doc.Editor.Selection(), doc.Editor.String())
	}
}

func TestBeginEndSelectMaintainsIndependentPaneAnchorsAndRejectsTabSwitch(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 0
	state.space.Select(0)
	left.Editor.MoveTo(1, false)
	state.command("Begin/End Select")
	state.activePane = 1
	state.space.Select(1)
	right.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	right.Editor.MoveTo(4, false)
	state.command("Begin/End Select")
	if right.Editor.Selection() != (text.Range{Start: 2, End: 4}) {
		t.Fatalf("right pane selection = %+v", right.Editor.Selection())
	}
	state.activePane = 0
	state.space.Select(0)
	left.Editor.MoveTo(3, false)
	state.command("Begin/End Select")
	if left.Editor.Selection() != (text.Range{Start: 1, End: 3}) {
		t.Fatalf("left pane anchor lost: %+v", left.Editor.Selection())
	}
	other := state.space.NewDocument()
	other.Editor.Insert("next")
	state.showInActivePane(other)
	other.Editor.MoveTo(1, false)
	state.command("Begin/End Select")
	if other.Editor.Selection() != (text.Range{Start: 1, End: 1}) || !state.menuSelected("Begin/End Select") {
		t.Fatalf("old document anchor applied across tab switch: %+v", other.Editor.Selection())
	}
}

func TestBeginEndSelectUnicodeInsertionUndoRestoresAnchor(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("猫ab")
	clickID(t, driver, editorID)
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	doc.Editor.MoveTo(0, false)
	if err := driver.Type("中"); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled {
		t.Fatalf("native Undo handled=%v err=%v", handled, err)
	}
	if doc.Editor.String() != "猫ab" {
		t.Fatalf("native Undo = %q", doc.Editor.String())
	}
	doc.Editor.MoveTo(3, false)
	state.command("Begin/End Select")
	if doc.Editor.Selection() != (text.Range{Start: 2, End: 3}) {
		t.Fatalf("Unicode Undo anchor = %+v", doc.Editor.Selection())
	}
}

func TestBeginEndSelectClearsClosedDocumentAnchor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(2, false)
	state.command("Begin/End Select")
	state.closeTab(0)
	if state.selectAnchors[0].doc != nil || state.menuSelected("Begin/End Select") {
		t.Fatalf("closed document retained selection anchor: %+v", state.selectAnchors[0])
	}
}
