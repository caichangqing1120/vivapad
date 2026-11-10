package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestCaptureSourceLineTargetsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_LINE_TARGET_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_LINE_TARGET_SHOTS to capture source line mutations")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "line-targets.txt"
	doc.Editor.Insert(strings.Repeat("alpha keep beta ", 8) + "\nfooter keep")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
	state.styleToken(0, true)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 5})
	state.styleToken(1, false)
	state.query.Insert("footer")
	state.command("Mark All")
	doc.GoToLine(2)
	state.toggleBookmark()
	doc.Editor.MoveTo(0, false)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.command("Split Lines")
	doc.Editor.MoveTo(0, false)
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s source line layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "line-targets-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("split-desktop", 1280, 800)
	capture("split-compact", 640, 400)
	state.command("Undo")
	doc.Editor.MoveTo(0, false)
	capture("undo-compact", 640, 400)
	duplicate := state.space.NewDocument()
	state.showInActivePane(duplicate)
	duplicate.Name = "duplicate-targets.txt"
	duplicate.Editor.Insert("keep\nkeep\nkeep")
	duplicate.Editor.ClearHistory()
	duplicate.Editor.SetSelection(text.Range{Start: 0, End: 4})
	state.styleToken(1, false)
	duplicate.Editor.SetSelection(text.Range{Start: 10, End: 14})
	state.styleToken(0, false)
	duplicate.Editor.MoveTo(0, false)
	state.command("Remove Consecutive Duplicates")
	capture("consecutive-compact", 640, 400)
	state.command("Undo")
	duplicate.Editor.MoveTo(0, false)
	capture("consecutive-undo-compact", 640, 400)
	state.command("Clone to Other View")
	capture("clone-compact", 640, 400)
}
