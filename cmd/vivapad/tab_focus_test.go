package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/recovery"
)

func TestRecoveredTabClickFocusesEditorAfterOpeningExplicitFile(t *testing.T) {
	root := t.TempDir()
	savedPath := filepath.Join(root, "saved.txt")
	if err := os.WriteFile(savedPath, []byte("saved\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := recovery.Save(filepath.Join(filepath.Dir(sessionPath), "recovery.json"), recovery.State{Active: 0, Tabs: []recovery.Tab{{
		Name: "recovered draft", Content: "draft", SelectionStart: 5, SelectionEnd: 5,
	}}}); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{savedPath}, sessionPath)
	saved := state.space.Current()
	if saved.Name != "saved.txt" || len(state.space.Documents) != 2 {
		t.Fatalf("unexpected startup state: active=%q, tabs=%d", saved.Name, len(state.space.Documents))
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, tabScope.At(1))
	if got := state.space.Current().Editor.String(); got != "draft" {
		t.Fatalf("selected wrong tab: %q", got)
	}
	if got := driver.Snapshot().Focused; got != editorID {
		t.Fatalf("recovered tab click left focus %v; want editor %v", got, editorID)
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if got := state.space.Current().Editor.String(); got != "draftx" || saved.Editor.String() != "saved\n" {
		t.Fatalf("first key changed wrong text: recovered=%q, saved=%q", got, saved.Editor.String())
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || state.space.Current().Editor.String() != "draft" || !state.space.Current().Dirty() {
		t.Fatalf("Undo after first key = %q, dirty=%t, handled=%t, err=%v", state.space.Current().Editor.String(), state.space.Current().Dirty(), handled, err)
	}
	if handled, err := driver.ShortcutChord(primary()|gfx.ModifierShift, 'z'); err != nil || !handled || state.space.Current().Editor.String() != "draftx" {
		t.Fatalf("Redo after first key = %q, handled=%t, err=%v", state.space.Current().Editor.String(), handled, err)
	}
	if data, err := os.ReadFile(savedPath); err != nil || string(data) != "saved\r\n" {
		t.Fatalf("recovered tab edits changed the explicit file: %q, %v", data, err)
	}
}

func TestSplitTabClickFocusesSelectedPane(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.toggleSplit()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, step := range []struct {
		tab, editor gfx.ID
		pane        int
	}{{leftTabScope.At(0), editorID, 0}, {rightTabScope.At(1), rightEditorID, 1}} {
		clickID(t, driver, step.tab)
		if state.activePane != step.pane || driver.Snapshot().Focused != step.editor {
			t.Fatalf("split tab click left pane=%d focus=%v; want pane=%d focus=%v", state.activePane, driver.Snapshot().Focused, step.pane, step.editor)
		}
		if err := driver.Type("x"); err != nil {
			t.Fatal(err)
		}
	}
	if left.Editor.String() != "leftx" || right.Editor.String() != "rightx" {
		t.Fatalf("split edits: left=%q right=%q", left.Editor.String(), right.Editor.String())
	}
}

func TestTabClickRequestsEditorFocusDuringTheClick(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "single pane"
		if split {
			name = "split panes"
		}
		t.Run(name, func(t *testing.T) {
			state := newModel()
			left := state.space.Current()
			left.Editor.Insert("left")
			right := state.space.NewDocument()
			right.Editor.Insert("right")
			steps := []struct {
				tab, editor gfx.ID
			}{{tabScope.At(0), editorID}, {tabScope.At(1), editorID}}
			if split {
				state.toggleSplit()
				steps[0].tab = leftTabScope.At(0)
				steps[1].tab, steps[1].editor = rightTabScope.At(1), rightEditorID
			}
			config := newConfig(state)
			paint := config.Paint
			requests := make(map[gfx.ID]gfx.ID)
			config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
				paint(canvas, size)
				for _, step := range steps {
					if canvas.Clicked(step.tab) {
						requests[step.tab] = canvas.FocusRequest()
					}
				}
			}
			driver, err := app.NewDriver(config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			for _, step := range steps {
				clickID(t, driver, step.tab)
				if got := requests[step.tab]; got != step.editor {
					t.Errorf("click on %v requested focus %v during its event; want %v", step.tab, got, step.editor)
				}
			}
		})
	}
}
