package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestCaptureSearchMenusWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SEARCH_MENU_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SEARCH_MENU_SHOTS to capture Search menus")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	session, err := headless.Open(newConfig(state), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	state.space.Current().Editor.Insert("one\ntwo\n")
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		state.menuOpen = -1
		state.closeSearchSubmenus()
		state.searchMenuOffset = 0
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), menuScope.At(2))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Search menu layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "search-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, session.Driver(), searchCommandID(t, "Change History"))
		if err := session.Driver().Key(gfx.KeyDown, 0); err != nil {
			t.Fatal(err)
		}
		if session.Snapshot().Focused != changeHistoryScope.At(0) {
			t.Fatal("keyboard did not focus the first Change History action")
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Change History layout: %+v", size.name, problems)
		}
		row := syncScrollHit(t, session.Driver(), changeHistoryScope.At(0))
		image, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		pixel := image.RGBAAt(int(row.X+2), int(row.Y+5))
		if pixel.R != 252 || pixel.G != 236 || pixel.B != 239 || pixel.A != 255 {
			t.Fatalf("focused menu row has no visible highlight: %+v", pixel)
		}
		if err := session.Screenshot(filepath.Join(output, "search-history-keyboard-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s %dx%d: Search and Change History layout pass; focused row has opaque highlight pixels", size.name, size.width, size.height)
	}
}
