package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/external"
)

func TestViewNamedBrowserUsesSavedActivePane(t *testing.T) {
	path := filepath.Join(t.TempDir(), "right # draft.html")
	if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	right := state.space.Current()
	right.Editor.Insert("unsaved ")
	left := state.space.NewDocument()
	left.Editor.Insert("left")
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(right))
	var calls int
	state.openNamedBrowser = func(browser external.NamedBrowser, got string) error {
		calls++
		if browser != external.Edge || got != right.Path {
			t.Fatalf("launch = %q %q, want Edge and %q", browser, got, right.Path)
		}
		return errors.New("browser unavailable")
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	open := func() {
		clickID(t, driver, menuScope.At(3))
		clickID(t, driver, menuItemScope.At(len(menuCommands[3])-1))
	}
	open()
	if err := driver.Key(gfx.KeyEscape, 0); err != nil || state.namedBrowserMenuOpen || state.menuOpen != 3 {
		t.Fatalf("Escape did not return to View: %v", err)
	}
	clickID(t, driver, menuItemScope.At(len(menuCommands[3])-1))
	clickID(t, driver, namedBrowserScope.At(2))
	requireEditorCommandFocus(t, driver, rightEditorID)
	if calls != 1 || !strings.Contains(state.notice, "browser unavailable") || right.Editor.String() != "unsaved saved" || !right.Dirty() || left.Editor.String() != "left" {
		t.Fatalf("browser command changed editor state: calls=%d, notice=%q", calls, state.notice)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "saved" {
		t.Fatalf("browser command wrote source: %q, %v", data, err)
	}
}
