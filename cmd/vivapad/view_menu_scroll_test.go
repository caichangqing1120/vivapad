package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestCompactViewMenuScrollReachesEveryCommandAtReadableHeight(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	if state.viewMenuVisible >= len(menuCommands[3]) || state.viewMenuVisible < 10 {
		t.Fatalf("View menu visible count = %d", state.viewMenuVisible)
	}
	seen := make([]bool, len(menuCommands[3]))
	for step := range len(menuCommands[3]) {
		for _, hit := range driver.Snapshot().Hits {
			for index := range seen {
				if hit.ID == menuItemScope.At(index) {
					seen[index] = true
					if hit.Bounds.Height < 20 || hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
						t.Fatalf("View row %d unreadable or clipped: %+v", index, hit.Bounds)
					}
				}
			}
		}
		if seen[len(seen)-1] {
			break
		}
		if step == 0 {
			bounds := syncScrollHit(t, driver, menuItemScope.At(1))
			if err := driver.Scroll(gfx.ScrollEvent{X: bounds.X + 20, Y: bounds.Y + 8, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			if state.viewMenuOffset == 0 {
				t.Fatal("View menu ignored wheel")
			}
		} else {
			clickID(t, driver, viewMenuNextID)
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("View item %d unreachable", index)
		}
	}
	if state.space.Current().Dirty() || state.menuOpen != 3 {
		t.Fatal("View menu scroll executed a command")
	}
	for state.viewMenuOffset > 0 {
		clickID(t, driver, viewMenuPreviousID)
	}
	clickID(t, driver, menuItemScope.At(10))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == viewTabPanelID {
			return
		}
	}
	t.Fatal("View > Tab submenu disappeared after scrolling")
}

func TestCaptureViewMenuScrollWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_VIEW_MENU_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_VIEW_MENU_SHOTS to capture View menu")
	}
	session, err := headless.Open(newConfig(newModel()), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact-top", 640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(3))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "view-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		if size.name == "desktop" {
			clickID(t, session.Driver(), menuScope.At(3))
		}
	}
	revealViewMenuItem(t, session.Driver(), len(menuCommands[3])-1)
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact bottom layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "view-menu-compact-bottom.png")); err != nil {
		t.Fatal(err)
	}
}
