package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
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
	panel := ui.Rect{}
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
	if err := driver.Scroll(ui.ScrollEvent{X: panel.X + 20, Y: panel.Y + 45, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if state.windowsOffset == 0 {
		t.Fatal("Windows list ignored wheel")
	}
	clickID(t, driver, windowsRowScope.At(state.windowsOffset))
	if len(state.windowsSelected) != 1 {
		t.Fatal("Windows multi-selection did not track clicked row")
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
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
