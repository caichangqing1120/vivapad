package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestCaptureLocaleRectangleWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_LOCALE_RECT_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_LOCALE_RECT_SHOTS to capture Locale rectangle sorting")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "locale-column-sort.txt"
	before := "outside keep\nz 22 keep\na 1 keep\nm 3 keep\ntail keep"
	want := "outside keep\na 1 keep\nm 3 keep\nz 22 keep\ntail keep"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 8, End: 12})
	state.styleToken(0, true)
	state.query.Insert("keep")
	state.command("Mark All")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	clickID(t, session.Driver(), editorID)
	anchor, caret := len("outside keep\nz "), len("outside keep\nz 22 keep\na 1 keep\nm 3 ")
	doc.Editor.MoveTo(caret, false)
	state.rectangles[0] = rectangleSelection{doc: doc, anchor: anchor, caret: caret, value: before, active: true}
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s Locale rectangle layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "locale-rectangle-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("before-compact", 640, 400)
	state.command("Line Operations")
	for count := 0; count < len(lineCommands); count++ {
		if err := session.Resize(640, 400); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, hit := range session.Snapshot().Hits {
			found = found || hit.ID == lineScope.At(len(lineCommands)-3)
		}
		if found {
			break
		}
		clickID(t, session.Driver(), lineNextID)
	}
	capture("menu-compact", 640, 400)
	clickID(t, session.Driver(), lineScope.At(len(lineCommands)-3))
	if doc.Editor.String() != want || !state.rectangles[0].active {
		t.Fatalf("captured Locale command lost its rectangle: %q", doc.Editor.String())
	}
	capture("after-compact", 640, 400)
	capture("after-desktop", 1280, 800)
	state.command("Clone to Other View")
	state.command("Undo")
	if doc.Editor.String() != before || !state.rectangles[0].active || state.rectangles[1].active {
		t.Fatal("captured Locale clone Undo lost owner")
	}
	capture("undo-clone-compact", 640, 400)
	state.command("Redo")
	capture("redo-clone-compact", 640, 400)
	short := state.space.NewDocument()
	state.showInActivePane(short)
	short.Name = "virtual-locale.txt"
	short.Editor.Insert("z 20\na\nm 3")
	short.Editor.ClearHistory()
	short.Editor.MoveTo(short.Editor.RuneCount(), false)
	state.rectangles[state.activePane] = rectangleSelection{doc: short, anchor: 2, caret: short.Editor.Caret(), caretVirtual: 1, value: short.Editor.String(), active: true}
	state.command("Sort In Locale Order Ascending")
	if short.Editor.String() != "a\nm 3\nz 20" || state.rectangles[state.activePane].anchorVirtual != 1 {
		t.Fatal("captured Locale virtual-space endpoint was not restored")
	}
	capture("virtual-compact", 640, 400)
}
