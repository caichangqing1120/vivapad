package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestMacEOLConversionMenuFitsCompactAndSavesExactBytes(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("first\nsecond"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(21))
	for index := range eolCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == eolScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("EOL row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("EOL row %d missing", index)
		}
	}
	clickID(t, driver, eolScope.At(2))
	if doc.LineEnding() != document.CR || doc.Editor.String() != "first\nsecond" || !doc.Dirty() {
		t.Fatalf("CR menu = text %q ending %v dirty %v", doc.Editor.String(), doc.LineEnding(), doc.Dirty())
	}
	state.command("Read-Only on Current Document")
	state.command("Windows (CR LF)")
	if doc.LineEnding() != document.CR || state.notice != "Read-only document" {
		t.Fatalf("read-only conversion changed to %v notice=%q", doc.LineEnding(), state.notice)
	}
	state.command("Read-Only on Current Document")
	path := filepath.Join(t.TempDir(), "mac.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, saveID)
	if data, err := os.ReadFile(path); err != nil || string(data) != "first\rsecond" || doc.Dirty() {
		t.Fatalf("saved CR = %q, %v dirty=%v", data, err, doc.Dirty())
	}
	state.command("Show End of Line")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, label := range driver.Snapshot().Texts {
		seen = seen || label.Value == "CR"
	}
	if !seen {
		t.Fatal("visible EOL did not show CR")
	}
}

func TestMacEOLClipboardAndRecoveryPreserveLogicalLines(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree")
	doc.SetLineEnding(document.CR)
	doc.Editor.MoveTo(0, false)
	state.command("Toggle Bookmark")
	doc.Editor.MoveTo(4, false)
	state.command("Toggle Bookmark")
	state.command("Copy Bookmarked Lines")
	if got := driver.Clipboard(); got != "one\rtwo\r" {
		t.Fatalf("CR bookmark clipboard = %q", got)
	}
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	stored := recovery.State{Active: 0, Tabs: []recovery.Tab{{Name: "new 1", Content: "one\ntwo", Encoding: document.UTF8, LineEnding: document.CR}}}
	if err := recovery.Save(filepath.Join(filepath.Dir(path), "recovery.json"), stored); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, path)
	if restored.space.Current().LineEnding() != document.CR || restored.space.Current().Editor.String() != "one\ntwo" {
		t.Fatalf("CR recovery restored %q ending=%v", restored.space.Current().Editor.String(), restored.space.Current().LineEnding())
	}
}

func TestNPPSessionReopensCRFileWithByteSelection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mac.txt")
	if err := os.WriteFile(path, []byte("猫\rabc"), 0600); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Main: []session.NPPFile{{Path: path, Encoding: -1, StartPos: 4, EndPos: 7}}}); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	doc := state.space.Current()
	if doc.LineEnding() != document.CR || doc.Editor.String() != "猫\nabc" || doc.Editor.Selection() != (text.Range{Start: 2, End: 5}) || nppBytePosition(doc, 2) != 4 {
		t.Fatalf("NPP CR import: ending=%v value=%q selection=%+v pos=%d", doc.LineEnding(), doc.Editor.String(), doc.Editor.Selection(), nppBytePosition(doc, 2))
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}
