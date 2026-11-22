package main

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestBookmarkSubmenuKeyboardNavigationAndActivation(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("first\nsecond")
	state.command("Clone to Other View")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	doc.GoToLine(2)
	clickID(t, driver, menuScope.At(2))
	parent := searchCommandID(t, "Bookmark")
	clickID(t, driver, parent)
	steps := []struct {
		key  gfx.Key
		want gfx.ID
	}{
		{gfx.KeyDown, bookmarkScope.At(0)},
		{gfx.KeyUp, bookmarkScope.At(9)},
		{gfx.KeyDown, bookmarkScope.At(0)},
		{gfx.KeyEnd, bookmarkScope.At(9)},
		{gfx.KeyHome, bookmarkScope.At(0)},
		{gfx.KeyLeft, parent},
		{gfx.KeyRight, bookmarkScope.At(0)},
		{gfx.KeyEscape, parent},
		{gfx.KeyEnter, bookmarkScope.At(0)},
	}
	for _, step := range steps {
		if err := driver.Key(step.key, 0); err != nil {
			t.Fatal(err)
		}
		if focus := driver.Snapshot().Focused; focus != step.want || state.menuOpen != 2 || state.bookmarkMenuOpen != (step.want != parent) {
			t.Fatalf("key %v: focus=%v, want %v, Search=%d, Bookmark=%t", step.key, focus, step.want, state.menuOpen, state.bookmarkMenuOpen)
		}
		if doc.Editor.String() != "first\nsecond" || len(state.bookmarkLines(doc)) != 0 {
			t.Fatal("navigation changed the document or its bookmarks")
		}
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if state.menuOpen >= 0 || state.bookmarkMenuOpen || driver.Snapshot().Focused != rightEditorID || !slices.Equal(state.bookmarkLines(doc), []int{1}) {
		t.Fatalf("activation did not bookmark the active line and restore editor focus: marks=%v, focus=%v", state.bookmarkLines(doc), driver.Snapshot().Focused)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "first\nxsecond" {
		t.Fatalf("first key after bookmark activation: %q", got)
	}
}

func TestBookmarkKeyboardSkipsReadOnlyMutations(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("first\nsecond")
	doc.GoToLine(1)
	state.command("Toggle Bookmark")
	state.command("Read-Only on Current Document")
	clickID(t, driver, menuScope.At(2))
	clickID(t, driver, searchCommandID(t, "Bookmark"))
	for _, key := range []gfx.Key{gfx.KeyDown, gfx.KeyDown, gfx.KeyDown, gfx.KeyDown, gfx.KeyDown} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
	}
	if got := driver.Snapshot().Focused; got != bookmarkScope.At(5) {
		t.Fatalf("expected Copy after skipping disabled Cut, focused %v", got)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Clipboard() != "first\n" || doc.Editor.String() != "first\nsecond" || !state.isReadOnly(doc) {
		t.Fatal("read-only Copy did not preserve the protected editor")
	}
}
