package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func documentMapHit(t *testing.T, driver *app.Driver) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == documentMapID {
			return hit.Bounds
		}
	}
	t.Fatal("Document Map hit is missing")
	return gfx.Rect{}
}

func TestDocumentMapClickDragAndCloseDoNotEditDocument(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	value := strings.Repeat("line with some text\n", 220) + "last"
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	before := doc.Editor.Selection()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(17))
	if !state.documentMapVisible {
		t.Fatal("View command did not open Document Map")
	}
	mapBounds := documentMapHit(t, driver)
	if mapBounds.X < 640 || mapBounds.X+mapBounds.Width > 1280 || mapBounds.Y < menuHeight+toolbarHeight+tabHeight || mapBounds.Y+mapBounds.Height > 800-statusHeight {
		t.Fatalf("right dock bounds = %+v", mapBounds)
	}
	if err := driver.Click(mapBounds.X+mapBounds.Width/2, mapBounds.Y+mapBounds.Height-12); err != nil {
		t.Fatal(err)
	}
	if state.codeEditor().ScrollY() <= 0 || doc.Editor.Caret() != 0 || doc.Editor.Selection() != before || doc.Editor.CanUndo() || doc.Editor.String() != value {
		t.Fatalf("map click changed editor state: scroll=%v caret=%d", state.codeEditor().ScrollY(), doc.Editor.Caret())
	}
	if err := driver.Press(mapBounds.X+mapBounds.Width/2, mapBounds.Y+44, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(mapBounds.X+mapBounds.Width/2, mapBounds.Y+mapBounds.Height-12); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(mapBounds.X+mapBounds.Width/2, mapBounds.Y+mapBounds.Height-12, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.codeEditor().ScrollY() <= 0 || doc.Editor.Caret() != 0 {
		t.Fatal("map drag moved caret or failed to scroll")
	}
	clickID(t, driver, documentMapCloseID)
	if state.documentMapVisible {
		t.Fatal("map close control left dock visible")
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == documentMapID {
			t.Fatal("closed Document Map retained a live hit region")
		}
	}
}

func TestDocumentMapWheelAndSplitFollowActivePaneAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert(strings.Repeat("left\n", 110))
	left.Editor.MoveTo(0, false)
	right := state.space.NewDocument()
	right.Editor.Insert(strings.Repeat("right\n", 120))
	right.Editor.MoveTo(0, false)
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.folderVisible = true
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, 17)
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(17) && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("Document Map menu row overlaps status: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(17))
	bounds := documentMapHit(t, driver)
	if bounds.X < state.folderPanelWidth(640)+200 || bounds.X+bounds.Width > 640 || bounds.Y+bounds.Height > 400-statusHeight {
		t.Fatalf("compact split/map/workspace bounds = %+v", bounds)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: bounds.X + bounds.Width/2, Y: bounds.Y + 80, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if state.codeEditorForPane(1).ScrollY() <= 0 || state.codeEditorForPane(0).ScrollY() != 0 {
		t.Fatalf("map wheel routed to wrong pane: left=%v right=%v", state.codeEditorForPane(0).ScrollY(), state.codeEditorForPane(1).ScrollY())
	}
	state.activePane = 0
	state.space.Select(state.documentIndex(left))
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	bounds = documentMapHit(t, driver)
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height-10); err != nil {
		t.Fatal(err)
	}
	if state.codeEditorForPane(0).ScrollY() <= 0 || left.Editor.Caret() != 0 || right.Editor.Caret() != 0 {
		t.Fatal("map did not follow newly active pane")
	}
}

func TestDocumentMapViewportProjectionBounds(t *testing.T) {
	bounds := gfx.Rect{X: 100, Y: 50, Width: 120, Height: 270}
	for _, test := range []struct {
		scroll, maxScroll, viewHeight float32
		wantTop, wantHeight           float32
	}{
		{0, 0, 240, 50, 270},
		{0, 720, 240, 50, 67.5},
		{360, 720, 240, 151.25, 67.5},
		{720, 720, 240, 252.5, 67.5},
	} {
		got := documentMapViewport(bounds, test.scroll, test.maxScroll, test.viewHeight)
		if got.Y != test.wantTop || got.Height != test.wantHeight || got.X < bounds.X || got.Y+got.Height > bounds.Y+bounds.Height {
			t.Fatalf("viewport(%v,%v,%v) = %+v", test.scroll, test.maxScroll, test.viewHeight, got)
		}
	}
}

func TestDocumentMapPreviewBoundsLongLinesAndStopsAtNewline(t *testing.T) {
	value := strings.Repeat("a", 1<<20) + "\nnext"
	if preview := documentMapPreview(value, 200); len(preview) != 256 || preview[0] != 'a' {
		t.Fatalf("long-line sample = %q (%d bytes)", preview, len(preview))
	}
	if preview := documentMapPreview(value, 1<<20-2); preview != "aa" {
		t.Fatalf("sample crossed a newline: %q", preview)
	}
}

func TestDocumentMapVisibilitySurvivesRestartAndOldSessionDefaultsOff(t *testing.T) {
	root := t.TempDir()
	file, sessionPath := filepath.Join(root, "source.txt"), filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(file, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{file}, sessionPath)
	state.command("Document Map")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || !stored.DocumentMapVisible {
		t.Fatalf("saved map visibility = %+v, %v", stored, err)
	}
	if restored := newStartupModel(nil, sessionPath); !restored.documentMapVisible {
		t.Fatal("normal restart lost Document Map")
	}
	if explicit := newStartupModel([]string{file}, sessionPath); !explicit.documentMapVisible {
		t.Fatal("explicit-argument restart lost Document Map")
	}
}

func TestDocumentMapTracksFoldedAndWrappedVisualRows(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("func main() {\n" + strings.Repeat("  if true { doLongWorkWithManyArguments(1, 2, 3, 4, 5, 6) }\n", 60) + "}\n" + strings.Repeat("tail with a long visual line and more words\n", 80))
	doc.Editor.MoveTo(0, false)
	state.command("Go")
	state.wrap = true
	state.documentMapVisible = true
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	before := state.codeEditor().MaxScrollY()
	state.command("Fold All")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if after := state.codeEditor().MaxScrollY(); after >= before || after <= 0 {
		t.Fatalf("map did not follow folded wrapped height: before=%v after=%v", before, after)
	}
	bounds := documentMapHit(t, driver)
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height-10); err != nil {
		t.Fatal(err)
	}
	if state.codeEditor().ScrollY() <= 0 || doc.Editor.Caret() != 0 {
		t.Fatal("folded map did not scroll visual rows safely")
	}
}

func TestDocumentMapViewportKeepsMiniatureRowsVisible(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert(strings.Repeat("source line\n", 200))
	doc.Editor.MoveTo(0, false)
	state.documentMapVisible = true
	var canvas gfx.Canvas
	state.paint(&canvas, gfx.Size{Width: 640, Height: 400})
	fillIndex, sampleIndex := -1, -1
	for index, rectangle := range canvas.Rectangles() {
		if rectangle.Color == gfx.RGB(255, 246, 247) {
			fillIndex = index
		}
		if rectangle.Color == gfx.RGB(158, 165, 172) && rectangle.Bounds.Y < state.documentMapContent.Y+50 && sampleIndex < 0 {
			sampleIndex = index
		}
	}
	if fillIndex < 0 || sampleIndex <= fillIndex {
		t.Fatalf("viewport covered its miniature lines: fill=%d sample=%d", fillIndex, sampleIndex)
	}
}
