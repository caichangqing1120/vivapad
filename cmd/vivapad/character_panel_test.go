package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func TestCharacterPanelCodePageAndRepresentations(t *testing.T) {
	for _, test := range []struct {
		encoding document.Encoding
		value    byte
		want     [3]string
		html     [3]string
	}{
		{document.UTF8, 0x80, [3]string{"128", "80", "\u20ac"}, [3]string{"&euro;", "&#8364;", "&#x20ac;"}},
		{document.Windows1251, 0xc0, [3]string{"192", "C0", "\u0410"}, [3]string{}},
		{document.GB18030, 0x81, [3]string{"129", "81", ""}, [3]string{}},
		{document.Windows1252, 0x81, [3]string{"129", "81", ""}, [3]string{}},
		{document.UTF16LE, 0, [3]string{"0", "00", ""}, [3]string{}},
		{document.UTF8, 0x0d, [3]string{"13", "0D", "\n"}, [3]string{}},
		{document.UTF8, '<', [3]string{"60", "3C", "<"}, [3]string{"&lt;", "&#60;", "&#x3c;"}},
	} {
		row := makeCharacterRow(test.encoding, test.value)
		if row.values != test.want || row.html != test.html {
			t.Errorf("encoding %v byte %02X: got %+v, want %v and %v", test.encoding, test.value, row, test.want, test.html)
		}
	}
}

func TestCompactCharacterPanelInsertsAndUndoPreservesSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "draft.txt")
	if err := os.WriteFile(file, []byte("original\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	doc := state.space.Current()
	left := state.space.NewDocument()
	left.Editor.Insert("left draft")
	state.paneOwner[left], state.paneOwner[doc] = 0, 1
	state.paneDocs = [2]*document.Document{left, doc}
	state.splitEnabled, state.activePane = true, 1
	state.space.Select(state.documentIndex(doc))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	for n := 0; n < len(menuCommands[1]); n++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.ID == menuItemScope.At(len(menuCommands[1])-1)
		}
		if found {
			break
		}
		clickID(t, driver, editMenuNextID)
	}
	clickID(t, driver, menuItemScope.At(len(menuCommands[1])-1))
	if state.activeDock() != "characters" {
		t.Fatal("Character Panel did not open")
	}
	cell := characterCellScope.At(0x80*3 + 2)
	for n := 0; n < 256; n++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.ID == cell
		}
		if found {
			break
		}
		clickID(t, driver, characterNextID)
	}
	clickID(t, driver, cell)
	requireEditorCommandFocus(t, driver, rightEditorID)
	if got := doc.Editor.String(); got != "\u20acoriginal\n" || doc.Encoding() != document.UTF8 {
		t.Fatalf("inserted character: %q, encoding %v", got, doc.Encoding())
	}
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "original\n" {
		t.Fatalf("one Undo: %q, %v", doc.Editor.String(), err)
	}
	clickID(t, driver, characterHTMLID)
	clickID(t, driver, characterCellScope.At(0x80*3))
	requireEditorCommandFocus(t, driver, rightEditorID)
	if doc.Editor.String() != "&euro;original\n" || left.Editor.String() != "left draft" {
		t.Fatalf("HTML insert affected wrong document: %q, %q", left.Editor.String(), doc.Editor.String())
	}
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil || doc.Editor.String() != "original\n" {
		t.Fatalf("entity Undo: %q, %v", doc.Editor.String(), err)
	}
	state.readOnly[doc] = true
	clickID(t, driver, characterCellScope.At(0x80*3))
	if doc.Editor.String() != "original\n" || doc.Dirty() {
		t.Fatal("read-only insertion changed the source")
	}
	if disk, err := os.ReadFile(file); err != nil || string(disk) != "original\r\n" {
		t.Fatalf("source file = %q, %v", disk, err)
	}
}
