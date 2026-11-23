package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestHideLinesClipsEdgesAndMergesAdjacentRanges(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	value := "first\nsecond\nthird\nfourth\nfifth\nsixth"
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Hide Lines")
	if got := state.hiddenLines[doc]; !reflect.DeepEqual(got, []hiddenLineRange{{Start: 1, End: 4}}) {
		t.Fatalf("edge-clipped range = %+v", got)
	}
	doc.GoToLine(5)
	state.command("Hide Lines")
	if got := state.hiddenLines[doc]; !reflect.DeepEqual(got, []hiddenLineRange{{Start: 1, End: 4}}) {
		t.Fatalf("adjacent merge = %+v", got)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != value || doc.Editor.CanUndo() || !doc.Dirty() {
		t.Fatal("hiding lines changed the document")
	}
	clickID(t, driver, hideLineScope.At(0))
	if len(state.hiddenLines[doc]) != 0 || doc.Editor.String() != value || doc.Editor.CanUndo() {
		t.Fatal("gutter restore changed text or left lines hidden")
	}
	doc.GoToLine(2)
	state.command("Hide Lines")
	doc.GoToLine(3)
	state.command("Hide Lines")
	if got := state.hiddenLines[doc]; !reflect.DeepEqual(got, []hiddenLineRange{{Start: 1, End: 2}}) {
		t.Fatalf("adjacent hidden spans did not merge: %+v", got)
	}
}

func TestHideLinesShareCloneAndClearAfterEdit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("head\ninside\nmore\nlast")
	doc.Editor.ClearHistory()
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{doc, doc}
	state.activePane = 1
	doc.GoToLine(2)
	state.command("Hide Lines")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []gfx.ID{hideLineScope.At(0), rightHideLineScope.At(0)} {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.ID == id
		}
		if !found {
			t.Fatalf("clone missing hide marker %v", id)
		}
	}
	doc.Editor.Insert("x")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(state.hiddenLines[doc]) != 0 || !strings.Contains(doc.Editor.String(), "inside") {
		t.Fatal("edit retained stale hidden line numbers")
	}
}

func TestHideLinesViewCommandFitsCompactWindow(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree")
	doc.GoToLine(2)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	row := syncScrollHit(t, driver, menuItemScope.At(viewCommandIndex("Hide Lines")))
	if row.Y+row.Height > 400-statusHeight || row.Height < 12 {
		t.Fatalf("Hide Lines row clipped: %+v", row)
	}
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Hide Lines")))
	if len(state.hiddenLines[doc]) != 1 {
		t.Fatal("View menu did not hide the caret line")
	}
}

func TestHideLinesCoexistsWithSourceFolds(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("{\n  \"a\": 1,\n  \"b\": 2\n}\nlast")
	state.command("JSON")
	doc.GoToLine(3)
	state.command("Hide Lines")
	state.command("Fold All")
	if len(state.hiddenLines[doc]) != 1 || len(state.visualFoldRanges(doc)) != 1 {
		t.Fatal("fold and hidden lines were not combined")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, leftFoldScope.At(0))
	if len(state.hiddenLines[doc]) != 1 {
		t.Fatal("expanding syntax fold lost hidden span")
	}
	clickID(t, driver, hideLineScope.At(1))
	if len(state.hiddenLines[doc]) != 0 || doc.Editor.String() != "{\n  \"a\": 1,\n  \"b\": 2\n}\nlast" {
		t.Fatal("restoring hidden lines changed the folded source")
	}
}

func TestCaptureHideLinesWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_HIDE_LINES_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_HIDE_LINES_SHOTS to capture hidden lines")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "log.txt"
	doc.Editor.Insert("start\nfirst\nsecond\nthird\nlast")
	doc.Editor.SetSelection(text.Range{Start: 6, End: 18})
	state.command("Hide Lines")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), menuScope.At(3))
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "hide-lines-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
