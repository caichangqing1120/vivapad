package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestFindResultReleaseDoesNotActivateReplacement(t *testing.T) {
	for _, change := range []string{"close source", "new search"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			firstPath := filepath.Join(dir, "first.txt")
			secondPath := filepath.Join(dir, "second.txt")
			for _, name := range []string{firstPath, secondPath} {
				if err := os.WriteFile(name, []byte("cat here\ndog there\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := newModelWithFiles([]string{firstPath, secondPath})
			first, second := state.space.Documents[0], state.space.Documents[1]
			state.space.Select(1)
			state.query = text.NewEditor("cat")
			state.findAllOpenDocuments()
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			firstRow := func() gfx.Rect {
				t.Helper()
				for _, hit := range driver.Snapshot().Hits {
					if hit.ID.Scope() == resultScope {
						return hit.Bounds
					}
				}
				t.Fatal("missing search result row")
				return gfx.Rect{}
			}
			pressed := firstRow()
			x, y := pressed.X+pressed.Width/2, pressed.Y+pressed.Height/2
			if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
				t.Fatal(err)
			}
			want := text.Range{Start: 0, End: 3}
			switch change {
			case "close source":
				state.closeTab(state.documentIndex(first))
			case "new search":
				state.query = text.NewEditor("dog")
				state.findAllCurrent()
				want = text.Range{Start: 9, End: 12}
			}
			state.invalidator.Invalidate()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if len(state.fileResults.Matches) != 1 || firstRow() != pressed || state.space.Current() != second {
				t.Fatal("replacement row is not beneath the held press")
			}
			before := second.Editor.Selection()
			if err := driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
				t.Fatal(err)
			}
			if state.resultSelected != -1 || second.Editor.Selection() != before {
				t.Errorf("stale release activated replacement: result %d, selection %+v", state.resultSelected, second.Editor.Selection())
			}
			if err := driver.Click(x, y); err != nil {
				t.Fatal(err)
			}
			if state.resultSelected != 0 || second.Editor.Selection() != want || driver.Snapshot().Focused.Scope() != resultScope {
				t.Errorf("fresh click did not select the replacement result: result %d, selection %+v, focus %v", state.resultSelected, second.Editor.Selection(), driver.Snapshot().Focused)
			}
		})
	}
}

func TestFindResultKeepsFocusWhenAnotherSourceCloses(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.txt")
	secondPath := filepath.Join(dir, "second.txt")
	for _, name := range []string{firstPath, secondPath} {
		if err := os.WriteFile(name, []byte("cat here\ncat there\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{firstPath, secondPath})
	state.query = text.NewEditor("cat")
	state.findAllOpenDocuments()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	row := state.resultRowID(0)
	clickID(t, driver, row)
	if driver.Snapshot().Focused != row || state.resultSelected != 0 {
		t.Fatal("first result did not take keyboard focus")
	}
	state.closeTab(1)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.fileResults.Matches) != 2 || driver.Snapshot().Focused != row {
		t.Errorf("removing the other source lost result focus: focus %v", driver.Snapshot().Focused)
	}
	if err := driver.Key(gfx.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if state.resultSelected != 1 || driver.Snapshot().Focused != resultsPaneID {
		t.Errorf("End did not navigate the surviving results: result %d, focus %v", state.resultSelected, driver.Snapshot().Focused)
	}
}
