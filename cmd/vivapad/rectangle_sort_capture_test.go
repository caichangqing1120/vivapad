package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestCaptureRectangleSortWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_RECT_SORT_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_RECT_SORT_SHOTS to capture rectangle sorting")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "column-sort.txt"
	before := "outside keep\nz 22 keep\na 1 keep\nm 3 keep\ntail keep"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
	state.styleToken(0, true)
	state.query.Insert("keep")
	state.command("Mark All")
	anchor, caret := len("outside keep\nz "), len("outside keep\nz 22 keep\na 1 keep\nm 3 ")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	clickID(t, session.Driver(), editorID)
	doc.Editor.MoveTo(caret, false)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, value: before, active: true}
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s rectangle sort layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "rectangle-sort-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("before-compact", 640, 400)
	state.command("Line Operations")
	capture("menu-compact", 640, 400)
	clickID(t, session.Driver(), lineScope.At(0))
	if doc.Editor.String() != "outside keep\na 1 keep\nz 22 keep\nm 3 keep\ntail keep" {
		t.Fatalf("captured menu did not sort by key: %q", doc.Editor.String())
	}
	capture("after-compact", 640, 400)
	capture("after-desktop", 1280, 800)
	state.command("Clone to Other View")
	state.command("Undo")
	if doc.Editor.String() != before || !state.rectangles[0].active || state.rectangles[1].active {
		t.Fatal("captured clone Undo lost the owning rectangle")
	}
	capture("undo-clone-compact", 640, 400)
	state.command("Redo")
	capture("redo-clone-compact", 640, 400)
}
