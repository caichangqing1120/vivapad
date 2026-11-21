package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/document"
)

func syncScrollDriver(t *testing.T) (*model, *app.Driver, *document.Document, *document.Document) {
	t.Helper()
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Name = "left.txt"
	left.Editor.Insert(strings.Repeat(strings.Repeat("left ", 100)+"\n", 160))
	left.Editor.ClearHistory()
	left.Editor.MoveTo(0, false)
	right := state.space.NewDocument()
	right.Name = "right.txt"
	right.Editor.Insert(strings.Repeat(strings.Repeat("right ", 100)+"\n", 160))
	right.Editor.ClearHistory()
	right.Editor.MoveTo(0, false)
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(left))
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	return state, driver, left, right
}

func syncScrollHit(t *testing.T, driver *app.Driver, id gfx.ID) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	for index := range menuCommands[3] {
		if id == menuItemScope.At(index) {
			if viewMenuControls(driver) {
				return revealViewMenuItem(t, driver, index)
			}
		}
	}
	t.Fatalf("missing scroll hit %v", id)
	return gfx.Rect{}
}

func TestSynchronizedScrollingPreservesOffsetsAndRoutesBothViews(t *testing.T) {
	state, driver, left, right := syncScrollDriver(t)
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	main.SetScrollY(200)
	sub.SetScrollY(80)
	main.SetScrollX(100)
	sub.SetScrollX(40)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	for _, index := range []int{20, 21} {
		bounds := syncScrollHit(t, driver, menuItemScope.At(index))
		if bounds.Width < 240 || bounds.Y < menuHeight || bounds.Y+bounds.Height > 400-statusHeight {
			t.Fatalf("sync command %d is truncated or overlaps status: %+v", index, bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(20))
	if !state.syncVertical || main.ScrollY() != 200 || sub.ScrollY() != 80 {
		t.Fatal("vertical toggle snapped previously independent views")
	}
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(21))
	if !state.syncHorizontal || main.ScrollX() != 100 || sub.ScrollX() != 40 {
		t.Fatal("horizontal toggle snapped previously independent views")
	}
	leftBounds := syncScrollHit(t, driver, editorID)
	if err := driver.Scroll(gfx.ScrollEvent{X: leftBounds.X + 30, Y: leftBounds.Y + 50, DeltaY: 120}); err != nil {
		t.Fatal(err)
	}
	if main.ScrollY() <= 200 || main.ScrollY()-sub.ScrollY() != 120 || main.ScrollX()-sub.ScrollX() != 60 {
		t.Fatalf("left wheel offsets: main=(%v,%v) sub=(%v,%v)", main.ScrollX(), main.ScrollY(), sub.ScrollX(), sub.ScrollY())
	}
	rightBounds := syncScrollHit(t, driver, rightEditorID)
	if err := driver.Scroll(gfx.ScrollEvent{X: rightBounds.X + 30, Y: rightBounds.Y + 50, DeltaX: 120}); err != nil {
		t.Fatal(err)
	}
	if sub.ScrollX() <= 40 || main.ScrollX()-sub.ScrollX() != 60 || main.ScrollY()-sub.ScrollY() != 120 {
		t.Fatalf("right wheel offsets: main=(%v,%v) sub=(%v,%v)", main.ScrollX(), main.ScrollY(), sub.ScrollX(), sub.ScrollY())
	}
	if left.Editor.Caret() != 0 || right.Editor.Caret() != 0 || left.Editor.CanUndo() || right.Editor.CanUndo() {
		t.Fatal("synchronized scrolling changed caret or undo history")
	}
}

func TestSynchronizedScrollingScrollbarMapAndDisabledMode(t *testing.T) {
	state, driver, _, _ := syncScrollDriver(t)
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	state.command("Synchronize Vertical Scrolling")
	state.command("Synchronize Horizontal Scrolling")
	vertical := syncScrollHit(t, driver, scrollbarID)
	if err := driver.Click(vertical.X+vertical.Width/2, vertical.Y+vertical.Height*0.65); err != nil {
		t.Fatal(err)
	}
	if main.ScrollY() <= 0 || sub.ScrollY() != main.ScrollY() {
		t.Fatalf("vertical scrollbar not synchronized: %v, %v", main.ScrollY(), sub.ScrollY())
	}
	horizontal := syncScrollHit(t, driver, rightHorizontalID)
	if err := driver.Click(horizontal.X+horizontal.Width*0.45, horizontal.Y+horizontal.Height/2); err != nil {
		t.Fatal(err)
	}
	if sub.ScrollX() <= 0 || main.ScrollX() != sub.ScrollX() {
		t.Fatalf("horizontal scrollbar not synchronized: %v, %v", main.ScrollX(), sub.ScrollX())
	}
	state.command("Document Map")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	mapBounds := documentMapHit(t, driver)
	if err := driver.Click(mapBounds.X+mapBounds.Width/2, mapBounds.Y+mapBounds.Height-15); err != nil {
		t.Fatal(err)
	}
	if main.ScrollY() != sub.ScrollY() {
		t.Fatal("Document Map movement left split views unsynchronized")
	}
	state.command("Synchronize Vertical Scrolling")
	before := sub.ScrollY()
	main.SetScrollY(0)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if sub.ScrollY() != before {
		t.Fatal("disabled vertical synchronization moved peer pane")
	}
	state.command("Split View")
	state.command("Synchronize Horizontal Scrolling")
	state.command("Synchronize Vertical Scrolling")
	if state.splitEnabled {
		t.Fatal("single-view state was changed by synchronization toggles")
	}
}

func TestSynchronizedScrollingWrapClearsHorizontalOffset(t *testing.T) {
	state, driver, _, _ := syncScrollDriver(t)
	state.command("Synchronize Horizontal Scrolling")
	state.codeEditorForPane(0).SetScrollX(200)
	state.codeEditorForPane(1).SetScrollX(200)
	state.command("Word Wrap")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.codeEditorForPane(0).ScrollX() != 0 || state.codeEditorForPane(1).ScrollX() != 0 {
		t.Fatal("wrapped synchronized editors retained horizontal offset")
	}
}

func TestSynchronizedScrollingTracksIndependentAxesDuringRepaint(t *testing.T) {
	state, driver, _, _ := syncScrollDriver(t)
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	state.command("Synchronize Vertical Scrolling")
	state.command("Synchronize Horizontal Scrolling")
	main.SetScrollY(100)
	sub.SetScrollX(120)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if sub.ScrollY() != 100 || main.ScrollX() != 120 || sub.ScrollX() != 120 {
		t.Fatalf("independent paint axes lost: main=(%v,%v) sub=(%v,%v)", main.ScrollX(), main.ScrollY(), sub.ScrollX(), sub.ScrollY())
	}
}

func TestCaptureSynchronizedScrollingWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SYNC_SCROLL_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SYNC_SCROLL_SHOTS to capture synchronized views")
	}
	state := newModel()
	left := state.space.Current()
	left.Name = "main.log"
	left.Editor.Insert(strings.Repeat("main: "+strings.Repeat("wide line ", 22)+"\n", 65))
	right := state.space.NewDocument()
	right.Name = "sub.log"
	right.Editor.Insert(strings.Repeat("sub:  "+strings.Repeat("another column ", 18)+"\n", 70))
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(left))
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	state.codeEditorForPane(0).SetScrollY(160)
	state.codeEditorForPane(1).SetScrollY(80)
	state.command("Synchronize Vertical Scrolling")
	state.command("Synchronize Horizontal Scrolling")
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800},
		{"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), menuScope.At(3))
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "sync-scroll-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
