package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestViewTabNumericFallbackAndDirectionalNavigation(t *testing.T) {
	state, _ := startDriver(t)
	first := state.space.Current()
	second, third := state.space.NewDocument(), state.space.NewDocument()
	for _, doc := range []*document.Document{first, second, third} {
		doc.Editor.Insert(doc.Name)
		doc.Editor.ClearHistory()
	}
	state.command("9th Tab")
	if state.space.Current() != third {
		t.Fatal("unavailable ninth tab did not select the last tab")
	}
	state.command("First Tab")
	if state.space.Current() != first {
		t.Fatal("First Tab did not select first")
	}
	state.command("Previous Tab")
	if state.space.Current() != third {
		t.Fatal("Previous Tab did not wrap to last")
	}
	state.command("Next Tab")
	if state.space.Current() != first {
		t.Fatal("Next Tab did not wrap to first")
	}
	state.command("2nd Tab")
	if state.space.Current() != second {
		t.Fatal("numeric second tab did not select second")
	}
	state.command("Last Tab")
	if state.space.Current() != third {
		t.Fatal("Last Tab did not select last")
	}
	for _, doc := range []*document.Document{first, second, third} {
		if doc.Editor.String() != doc.Name || doc.Editor.CanUndo() {
			t.Fatalf("tab navigation changed %s: %q", doc.Name, doc.Editor.String())
		}
	}
}

func TestViewTabFirstNineCommandsAddressEachVisiblePosition(t *testing.T) {
	state, _ := startDriver(t)
	docs := []*document.Document{state.space.Current()}
	for range 9 {
		docs = append(docs, state.space.NewDocument())
	}
	for index, command := range viewTabCommands[:9] {
		state.command(command)
		if state.space.Current() != docs[index] {
			t.Fatalf("%q selected %q, want %q", command, state.space.Current().Name, docs[index].Name)
		}
	}
}

func TestViewTabReorderingKeepsSelectionAndText(t *testing.T) {
	state, _ := startDriver(t)
	first := state.space.Current()
	second, third, fourth := state.space.NewDocument(), state.space.NewDocument(), state.space.NewDocument()
	for _, doc := range []*document.Document{first, second, third, fourth} {
		doc.Editor.Insert(doc.Name)
		doc.Editor.ClearHistory()
	}
	state.space.Select(2)
	state.showInActivePane(third)
	assertOrder := func(want ...*document.Document) {
		t.Helper()
		for index, doc := range want {
			if state.space.Documents[index] != doc {
				t.Fatalf("tab order at %d: got %q, want %q", index, state.space.Documents[index].Name, doc.Name)
			}
		}
		if state.space.Current() != third || third.Editor.String() != third.Name || third.Editor.CanUndo() {
			t.Fatal("tab move changed selected document or its history")
		}
	}
	state.command("Move Tab Forward")
	assertOrder(first, second, fourth, third)
	state.command("Move to Start")
	assertOrder(third, first, second, fourth)
	state.command("Move to End")
	assertOrder(first, second, fourth, third)
	state.command("Move Tab Backward")
	assertOrder(first, second, third, fourth)
}

func TestViewTabReorderingRespectsPinnedPrefixAndSplitPane(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	second, third, fourth := state.space.NewDocument(), state.space.NewDocument(), state.space.NewDocument()
	state.pinned[first], state.pinned[second] = true, true
	state.space.Select(2)
	state.showInActivePane(third)
	state.command("Move Tab Backward")
	if state.space.Documents[2] != third {
		t.Fatal("unpinned tab crossed pinned prefix")
	}
	state.space.Select(1)
	state.showInActivePane(second)
	state.command("Move Tab Forward")
	if state.space.Documents[1] != second {
		t.Fatal("pinned tab crossed into unpinned group")
	}
	state.pinned = make(map[*document.Document]bool)
	state.paneOwner[first], state.paneOwner[second] = 0, 0
	state.paneOwner[third], state.paneOwner[fourth] = 1, 1
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = first, third
	state.activePane = 1
	state.space.Select(2)
	state.command("Next Tab")
	if state.paneDocs[1] != fourth || state.paneDocs[0] != first {
		t.Fatal("right-pane Next Tab switched the left view")
	}
	state.command("Move to Start")
	if tabs := state.paneTabs(1); tabs[0] != fourth || tabs[1] != third {
		t.Fatalf("right-pane move = %+v", tabs)
	}
	if tabs := state.paneTabs(0); tabs[0] != first || tabs[1] != second {
		t.Fatalf("left-pane order changed = %+v", tabs)
	}
}

func TestViewTabMenuAllCommandsReachableAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.space.NewDocument()
	state.space.NewDocument()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(10))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == viewTabScope.At(0) {
			if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			if state.viewTabOffset == 0 {
				t.Fatal("Tab submenu did not respond to the wheel")
			}
			if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: -120}); err != nil {
				t.Fatal(err)
			}
			if state.viewTabOffset != 0 {
				t.Fatal("Tab submenu did not scroll back")
			}
			break
		}
	}
	seen := make([]bool, len(viewTabCommands))
	for step := 0; step < len(viewTabCommands); step++ {
		for _, hit := range driver.Snapshot().Hits {
			for index := range seen {
				if hit.ID == viewTabScope.At(index) {
					seen[index] = true
					if hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
						t.Fatalf("tab command %d clipped: %+v", index, hit.Bounds)
					}
				}
			}
		}
		if seen[len(seen)-1] {
			break
		}
		clickID(t, driver, viewTabNextID)
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("tab command %d unreachable", index)
		}
	}
	for state.viewTabOffset > 0 {
		clickID(t, driver, viewTabPreviousID)
	}
	clickID(t, driver, viewTabScope.At(8))
	if state.space.Current() != state.space.Documents[2] {
		t.Fatal("compact 9th Tab did not select the last document")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSharedCloneMoveCannotReorderOtherView(t *testing.T) {
	state, driver := startDriver(t)
	first := state.space.Current()
	second, right := state.space.NewDocument(), state.space.NewDocument()
	state.paneOwner[first], state.paneOwner[second], state.paneOwner[right] = 0, 0, 1
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = second, first
	state.activePane = 1
	state.space.Select(0)
	state.command("Move to Start")
	if tabs := state.paneTabs(0); tabs[0] != first || tabs[1] != second {
		t.Fatalf("shared clone moved source view's order: %+v", tabs)
	}
	if state.notice != "Shared clone order follows its owning view" {
		t.Fatalf("shared clone limitation was not reported: %q", state.notice)
	}
}

func TestViewTabOrderPersistsForSavedDocuments(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 3)
	for index, name := range []string{"first.txt", "second.txt", "third.txt"} {
		paths[index] = filepath.Join(root, name)
		if err := os.WriteFile(paths[index], []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	privatePath := filepath.Join(root, "settings", "session.json")
	canonicalLast, err := filepath.EvalSymlinks(paths[2])
	if err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(paths, privatePath)
	state.command("Move to Start")
	if state.space.Documents[0].Path != canonicalLast {
		t.Fatalf("move-to-start before session = %+v", state.space.Documents)
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	restored := newStartupModel(nil, privatePath)
	if !allowed || len(restored.space.Documents) != 3 || restored.space.Documents[0].Path != canonicalLast {
		t.Fatalf("tab order not restored: allowed=%v docs=%+v", allowed, restored.space.Documents)
	}
}
