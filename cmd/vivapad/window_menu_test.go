package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestWindowMenuOverflowAndSortItemsFitCompactViewport(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	first := state.space.Current()
	first.Name = "z.txt"
	second := state.space.NewDocument()
	second.Name = "a.txt"
	clickID(t, driver, menuOverflowID)
	clickID(t, driver, hiddenMenuScope.At(11))
	if state.menuOpen != 11 {
		t.Fatal("Window menu not reachable through overflow")
	}
	clickID(t, driver, menuItemScope.At(0))
	if !state.windowSortMenuOpen {
		t.Fatal("Sort By submenu did not open")
	}
	seen := make([]bool, len(windowSortCommands))
	for _, hit := range driver.Snapshot().Hits {
		for index := range seen {
			if hit.ID == windowSortScope.At(index) {
				seen[index] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("Window sort row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("Window sort row %d unreachable", index)
		}
	}
	clickID(t, driver, windowSortScope.At(0))
	if !reflect.DeepEqual(state.space.Documents, []*document.Document{second, first}) || state.menuOpen != -1 || state.windowSortMenuOpen {
		t.Fatalf("menu sort = docs:%+v open:%d", state.space.Documents, state.menuOpen)
	}
}

func TestWindowMenuOverflowEscapePreservesDirtyTab(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("keep")
	clickID(t, driver, menuOverflowID)
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.menuOverflowOpen || doc.Editor.String() != "keep" || !doc.Dirty() {
		t.Fatal("Escape from overflow changed dirty document")
	}
}

func TestWindowRecentDynamicItemsFollowTabOrderAndDirtyState(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	first.Name = "first.txt"
	second := state.space.NewDocument()
	second.Name = "second.txt"
	second.Editor.Insert("draft")
	third := state.space.NewDocument()
	third.Name = "third.txt"
	state.space.Select(state.documentIndex(third))
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuOverflowID)
	clickID(t, driver, hiddenMenuScope.At(11))
	seen := make([]bool, 3)
	for _, hit := range driver.Snapshot().Hits {
		for index := range seen {
			if hit.ID == windowRecentScope.At(index) {
				seen[index] = true
				if index == 0 && !strings.Contains(hit.Label, first.Name) || index == 1 && (!strings.Contains(hit.Label, "second.txt*") || hit.Checked) || index == 2 && !hit.Checked {
					t.Fatalf("recent window state = %+v", hit)
				}
			}
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("dynamic window %d missing", index)
		}
	}
	clickID(t, driver, windowRecentScope.At(1))
	if state.space.Current() != second || state.menuOpen != -1 || second.Editor.String() != "draft" || !second.Dirty() {
		t.Fatal("dynamic recent window did not activate its tab safely")
	}
	if got := driver.Snapshot().Focused; got != editorID {
		t.Errorf("focus after Recent Window selection = %v, want editor", got)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if second.Editor.String() != "draftx" || first.Editor.String() != "" || third.Editor.String() != "" {
		t.Fatal("first key did not edit only the selected document")
	}
}

func TestWindowRecentMenuScrollsAndCapsOriginalFortyItems(t *testing.T) {
	state, driver := startDriver(t)
	docs := []*document.Document{state.space.Current()}
	for index := range 45 {
		doc := state.space.NewDocument()
		doc.Name = "tab" + strconv.Itoa(index)
		docs = append(docs, doc)
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuOverflowID)
	clickID(t, driver, hiddenMenuScope.At(11))
	seen := make([]bool, 40)
	for step := range 40 {
		for _, hit := range driver.Snapshot().Hits {
			for index := range seen {
				if hit.ID == windowRecentScope.At(index) {
					seen[index] = true
					if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight || hit.Bounds.X+hit.Bounds.Width > 640 {
						t.Fatalf("recent window %d clipped: %+v", index, hit.Bounds)
					}
				}
			}
			if hit.ID == windowRecentScope.At(40) {
				t.Fatal("recent menu exceeded original 40-command ID range")
			}
		}
		if seen[39] {
			break
		}
		if step == 0 {
			for _, hit := range driver.Snapshot().Hits {
				if hit.ID == windowRecentScope.At(0) {
					if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: 120}); err != nil {
						t.Fatal(err)
					}
					if state.windowRecentOffset == 0 {
						t.Fatal("dynamic Window menu ignored wheel")
					}
					break
				}
			}
		} else {
			clickID(t, driver, windowRecentNextID)
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("recent window %d unreachable", index)
		}
	}
	clickID(t, driver, windowRecentScope.At(39))
	if state.space.Current() != docs[39] {
		t.Fatal("last dynamic Window entry selected the wrong document")
	}
}

func TestWindowRecentDynamicListUsesOnlyActiveSplitView(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Name = "left.txt"
	right := state.space.NewDocument()
	right.Name = "right.txt"
	otherLeft := state.space.NewDocument()
	otherLeft.Name = "other-left.txt"
	otherRight := state.space.NewDocument()
	otherRight.Name = "other-right.txt"
	state.paneOwner[left], state.paneOwner[otherLeft] = 0, 0
	state.paneOwner[right], state.paneOwner[otherRight] = 1, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	clickID(t, driver, menuScope.At(11))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == windowRecentScope.At(2) || hit.ID == windowRecentScope.At(3) {
			t.Fatalf("other view leaked into Window menu: %+v", hit)
		}
	}
	clickID(t, driver, windowRecentScope.At(1))
	if state.space.Current() != otherRight || state.paneDocs[0] != left || state.activePane != 1 {
		t.Fatalf("Window list activated wrong split view: current=%p panes=%+v", state.space.Current(), state.paneDocs)
	}
	if got := driver.Snapshot().Focused; got != rightEditorID {
		t.Errorf("focus after Recent Window selection = %v, want right editor", got)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if otherRight.Editor.String() != "x" || left.Editor.String() != "" || right.Editor.String() != "" {
		t.Fatal("first key did not edit only the selected split document")
	}
}

func TestWindowRecentSelectingCurrentClonePreservesOwner(t *testing.T) {
	state, driver := startDriver(t)
	shared := state.space.Current()
	state.moveToOtherView(true)
	if state.activePane != 1 || state.paneDocs[0] != shared || state.paneDocs[1] != shared || state.paneOwner[shared] != 0 {
		t.Fatal("expected the active pane to hold a clone owned by the left pane")
	}
	clickID(t, driver, menuScope.At(11))
	clickID(t, driver, windowRecentScope.At(0))
	if state.activePane != 1 || state.paneDocs[0] != shared || state.paneDocs[1] != shared || state.paneOwner[shared] != 0 {
		t.Fatal("activating the clone changed its pane ownership")
	}
}
