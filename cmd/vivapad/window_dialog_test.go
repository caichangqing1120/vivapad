package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestWindowsDialogActivatesAndSortsCurrentView(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Name = "z.txt"
	second := state.space.NewDocument()
	second.Name = "a.txt"
	third := state.space.NewDocument()
	third.Name = "m.txt"
	state.space.Select(state.documentIndex(first))
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	if !state.windowsDialogOpen {
		t.Fatal("Windows dialog did not open")
	}
	clickID(t, driver, windowsRowScope.At(1))
	clickID(t, driver, windowsActivateID)
	if state.windowsDialogOpen || state.space.Current() != second || len(state.space.Documents) != 3 {
		t.Fatal("Windows dialog activation changed the workspace")
	}
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	clickID(t, driver, windowsSortNameID)
	clickID(t, driver, windowsSortTabsID)
	if !reflect.DeepEqual(state.space.Documents, []*document.Document{second, third, first}) || state.space.Current() != second {
		t.Fatalf("dialog tab sort = %+v active=%p", state.space.Documents, state.space.Current())
	}
	clickID(t, driver, windowsOKID)
}

func TestWindowsDialogDisplaysContainingDirectory(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"compact", 640, 400}, {"desktop", 1280, 800}} {
		t.Run(size.name, func(t *testing.T) {
			state, driver := startDriver(t)
			saved := state.space.Current()
			saved.Name = "draft.txt"
			saved.Path = filepath.Join("notes", saved.Name)
			untitled := state.space.NewDocument()
			state.openWindowsDialog()
			if err := driver.Resize(size.width, size.height); err != nil {
				t.Fatal(err)
			}
			snapshot := driver.Snapshot()
			var pathHeader gfx.Rect
			var rows [2]gfx.Hit
			for _, hit := range snapshot.Hits {
				switch hit.ID {
				case windowsHeaderScope.At(1):
					pathHeader = hit.Bounds
				case windowsRowScope.At(0):
					rows[0] = hit
				case windowsRowScope.At(1):
					rows[1] = hit
				}
			}
			if pathHeader.Width == 0 || rows[0].Bounds.Height == 0 || rows[1].Bounds.Height == 0 {
				t.Fatal("path header or document rows not visible")
			}
			var paths [2]string
			for _, text := range snapshot.Texts {
				if text.Bounds.X < pathHeader.X || text.Bounds.X >= pathHeader.X+pathHeader.Width {
					continue
				}
				for i, row := range rows {
					if text.Bounds.Y >= row.Bounds.Y && text.Bounds.Y < row.Bounds.Y+row.Bounds.Height {
						paths[i] += text.Value
					}
				}
			}
			if want := [2]string{"notes" + string(filepath.Separator), ""}; paths != want {
				t.Errorf("Path cells = %q, want %q", paths, want)
			}
			if rows[0].Hint != saved.Path || rows[1].Hint != "" {
				t.Errorf("row hints = %q, %q; want %q and empty", rows[0].Hint, rows[1].Hint, saved.Path)
			}
			if state.space.Current() != untitled || !reflect.DeepEqual(state.space.Documents, []*document.Document{saved, untitled}) {
				t.Fatal("showing path cells changed the active document or tab order")
			}
		})
	}
}

func TestWindowLengthsUseLiveUTF8BytesAndFollowUndo(t *testing.T) {
	state, driver := startDriver(t)
	unicode := state.space.Current()
	unicode.Editor.Insert("\u6c34\U0001f680")
	ascii := state.space.NewDocument()
	ascii.Editor.Insert("letters123")
	showLength := func(want int) {
		t.Helper()
		clickID(t, driver, menuScope.At(11))
		clickID(t, driver, menuItemScope.At(1))
		found := false
		for _, label := range driver.Snapshot().Texts {
			found = found || label.Value == strconv.Itoa(want)
		}
		if !found {
			t.Fatalf("Windows dialog did not show length %d", want)
		}
		clickID(t, driver, windowsOKID)
	}
	showLength(7)
	unicode.Editor.BreakUndoGroup()
	unicode.Editor.Insert("\U0001f680")
	showLength(11)
	if err := state.sortWindowTabs(windowSortCommand{column: 3}); err != nil || state.space.Documents[0] != ascii {
		t.Fatal("length sort did not use the 11-byte edited value")
	}
	if !unicode.Editor.Undo() {
		t.Fatal("could not undo the edit")
	}
	showLength(7)
	if err := state.sortWindowTabs(windowSortCommand{column: 3}); err != nil || state.space.Documents[0] != unicode {
		t.Fatal("length sort did not use the 7-byte value after undo")
	}
}

func TestWindowsDialogActivationKeepsFocusAndTabOwnership(t *testing.T) {
	for _, tc := range []struct {
		name          string
		clone, button bool
	}{{name: "single view Enter"}, {name: "shared clone Enter", clone: true}, {name: "shared clone Activate", clone: true, button: true}} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			peer := state.space.Current()
			peer.Name = "peer.txt"
			peer.Editor.Insert("peer")
			target := state.space.NewDocument()
			target.Name = "target.txt"
			target.Editor.Insert("target")
			target.Editor.BreakUndoGroup()
			focus := editorID
			if tc.clone {
				state.splitEnabled, state.activePane = true, 1
				state.paneDocs = [2]*document.Document{peer, target}
				state.paneOwner[peer], state.paneOwner[target] = 0, 0
				focus = rightEditorID
			} else {
				state.space.Select(state.documentIndex(peer))
			}
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuOverflowID)
			clickID(t, driver, hiddenMenuScope.At(11))
			clickID(t, driver, menuItemScope.At(1))
			clickLabeledControl(t, driver, "Select target.txt")
			if tc.button {
				clickID(t, driver, windowsActivateID)
			} else if err := driver.Key(gfx.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			if state.windowsDialogOpen || state.space.Current() != target {
				t.Fatal("activation did not select the target and close the dialog")
			}
			if tc.clone && (state.paneOwner[target] != 0 || state.paneDocs[0] != peer || state.paneDocs[1] != target || state.activePane != 1) {
				t.Errorf("activation changed clone ownership or pane: owner=%d active=%d", state.paneOwner[target], state.activePane)
			}
			if got := driver.Snapshot().Focused; got != focus {
				t.Fatalf("activation focused %v; want %v", got, focus)
			}
			if err := driver.Type("x"); err != nil {
				t.Fatal(err)
			}
			if target.Editor.String() != "targetx" || peer.Editor.String() != "peer" {
				t.Fatalf("first key: target=%q peer=%q", target.Editor.String(), peer.Editor.String())
			}
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || target.Editor.String() != "target" || peer.Editor.String() != "peer" {
				t.Fatalf("undo after activation: target=%q peer=%q handled=%v err=%v", target.Editor.String(), peer.Editor.String(), handled, err)
			}
		})
	}
}

func TestWindowsDialogSaveCancelAndDirtyCloseGate(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("keep")
	other := state.space.NewDocument()
	state.space.Select(state.documentIndex(doc))
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	clickID(t, driver, windowsRowScope.At(0))
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, windowsSaveID)
	if doc.Path != "" || !doc.Dirty() || doc.Editor.String() != "keep" || !state.windowsDialogOpen {
		t.Fatal("canceled Windows dialog save discarded edits")
	}
	path := filepath.Join(t.TempDir(), "kept.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, windowsSaveID)
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" || doc.Dirty() {
		t.Fatalf("dialog save = %q, %v dirty=%v", data, err, doc.Dirty())
	}
	doc.Editor.Insert(" again")
	clickID(t, driver, windowsCloseID)
	if state.windowsDialogOpen || state.pendingClose < 0 {
		t.Fatal("dirty Windows close bypassed confirmation")
	}
	clickID(t, driver, cancelCloseID)
	if state.documentIndex(doc) < 0 || doc.Editor.String() != "keep again" || state.space.Documents[state.documentIndex(other)] != other {
		t.Fatal("canceling dirty close lost a tab")
	}
}

func TestWindowsDialogCompactMultiSelectAndWheel(t *testing.T) {
	state, driver := startDriver(t)
	for index := range 25 {
		doc := state.space.NewDocument()
		doc.Name = "tab" + strconv.Itoa(index)
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuOverflowID)
	clickID(t, driver, hiddenMenuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	panel := gfx.Rect{}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == windowsPanelID {
			panel = hit.Bounds
		}
		if hit.ID.Scope() == windowsRowScope && (hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight) {
			t.Fatalf("Windows list row clipped: %+v", hit.Bounds)
		}
	}
	if panel.Width == 0 {
		t.Fatal("compact Windows dialog list missing")
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: panel.X + 20, Y: panel.Y + 45, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if state.windowsOffset == 0 {
		t.Fatal("Windows list ignored wheel")
	}
	clickID(t, driver, windowsRowScope.At(state.windowsOffset))
	if len(state.windowsSelected) != 1 {
		t.Fatal("Windows multi-selection did not track clicked row")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.windowsDialogOpen || len(state.space.Documents) != 26 || strings.TrimSpace(state.space.Current().Name) == "" {
		t.Fatal("closing Windows dialog changed tabs")
	}
}

func TestWindowsDialogMultiCloseStopsAtDirtyCancel(t *testing.T) {
	state, driver := startDriver(t)
	keep := state.space.Current()
	keep.Name = "keep.txt"
	dirty := state.space.NewDocument()
	dirty.Editor.Insert("draft")
	clean := state.space.NewDocument()
	state.space.Select(state.documentIndex(keep))
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	clickID(t, driver, windowsRowScope.At(1))
	clickID(t, driver, windowsRowScope.At(2))
	if len(state.windowsSelected) != 2 {
		t.Fatal("Windows multi-selection did not retain both rows")
	}
	clickID(t, driver, windowsCloseID)
	if state.windowsDialogOpen || state.pendingClose < 0 || state.documentIndex(clean) >= 0 || len(state.space.Documents) != 2 {
		t.Fatalf("multi-close did not stop at dirty gate: pending=%d docs=%+v", state.pendingClose, state.space.Documents)
	}
	clickID(t, driver, cancelCloseID)
	if state.documentIndex(dirty) < 0 || state.documentIndex(keep) < 0 || dirty.Editor.String() != "draft" || !dirty.Dirty() {
		t.Fatal("canceling multi-close lost a dirty or unselected tab")
	}
}

func TestWindowsDialogShrinksForFewDocumentsAndKeepsPinnedOrder(t *testing.T) {
	state, driver := startDriver(t)
	pinned := state.space.Current()
	pinned.Name = "z.txt"
	other := state.space.NewDocument()
	other.Name = "a.txt"
	state.pinned[pinned] = true
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == windowsPanelID && hit.Bounds.Height >= 170 {
			t.Fatalf("two-document list wastes most of dialog: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, windowsSortNameID)
	if len(state.windowsRows) != 2 || state.windowsRows[0] != pinned || state.windowsRows[1] != other {
		t.Fatalf("dialog sort displayed pinned tabs behind unpinned: %+v", state.windowsRows)
	}
	clickID(t, driver, windowsSortTabsID)
	if !reflect.DeepEqual(state.space.Documents, []*document.Document{pinned, other}) {
		t.Fatalf("sorting crossed pinned boundary: %+v", state.space.Documents)
	}
}

func TestWindowsDialogClosesOnlySelectedActiveSplitView(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Name = "left.txt"
	right := state.space.NewDocument()
	right.Name = "right.txt"
	secondLeft := state.space.NewDocument()
	secondLeft.Name = "left-two.txt"
	secondRight := state.space.NewDocument()
	secondRight.Name = "right-two.txt"
	state.paneOwner[left], state.paneOwner[secondLeft] = 0, 0
	state.paneOwner[right], state.paneOwner[secondRight] = 1, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, menuItemScope.At(1))
	if len(state.windowsRows) != 2 || state.windowsRows[0] != right || state.windowsRows[1] != secondRight {
		t.Fatalf("active-view Windows list = %+v", state.windowsRows)
	}
	clickID(t, driver, windowsRowScope.At(0))
	clickID(t, driver, windowsCloseID)
	if state.documentIndex(right) >= 0 || state.documentIndex(left) < 0 || state.documentIndex(secondLeft) < 0 || state.documentIndex(secondRight) < 0 || state.paneDocs[0] != left {
		t.Fatalf("split Windows close affected peer: docs=%+v panes=%+v", state.space.Documents, state.paneDocs)
	}
}
