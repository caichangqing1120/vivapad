package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestRectangleRowsBackwardAcrossWideAndShortLines(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("abcde\nxy\n猫123\n")
	start := doc.Editor.RuneIndex(strings.Index(doc.Editor.String(), "猫123")) + 3
	rows := doc.RectangleRows(Rectangle{Anchor: start, Caret: 2, TabWidth: 4})
	if len(rows) != 3 {
		t.Fatalf("rectangle rows = %+v", rows)
	}
	for index, want := range []string{"cd", "", "12"} {
		selection := rows[index].Range
		got := string([]rune(doc.Editor.String())[selection.Start:selection.End])
		if got != want {
			t.Fatalf("row %d selected %q, want %q", index, got, want)
		}
	}
	if rows[1].LeftPadding != 0 || rows[1].VirtualCells != 0 {
		t.Fatalf("short line rectangle = %+v", rows[1])
	}
}

func TestRectangleColumnEditReplacesOnlySelectedRowsAsOneUndo(t *testing.T) {
	doc := New("new 1")
	before := "abcde\nxy\n猫123\nlast"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	end := doc.Editor.RuneIndex(strings.Index(doc.Editor.String(), "猫123")) + 3
	changed, err := doc.EditRectangle(Rectangle{Anchor: 2, Caret: end, TabWidth: 4}, ColumnEdit{Text: "X"})
	if err != nil || !changed || doc.Editor.String() != "abXe\nxyX\n猫X3\nlast" {
		t.Fatalf("rectangle replacement = %q, %v, %v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("rectangle replacement Undo = %q", doc.Editor.String())
	}
}

func TestRectangleZeroWidthPadsVirtualCaretsAndKeepsMixedEOL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("abcd\r\nx\n猫2\r"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 4})
	end := doc.Editor.RuneIndex(strings.Index(doc.Editor.String(), "猫2")) + 2
	rows := doc.RectangleRows(Rectangle{Anchor: 4, Caret: end, CaretVirtual: 1, TabWidth: 4})
	if len(rows) != 3 || rows[1].LeftPadding != 3 || rows[1].VirtualCells != 3 {
		t.Fatalf("virtual rectangle rows = %+v", rows)
	}
	changed, err := doc.EditRectangle(Rectangle{Anchor: 4, Caret: end, CaretVirtual: 1, TabWidth: 4}, ColumnEdit{Number: true, Initial: "7", Increment: "1", Repeat: "1", Base: 10})
	if err != nil || !changed {
		t.Fatalf("virtual number insertion = %v, %v", changed, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "abcd7\r\nx   8\n猫2 9\r" {
		t.Fatalf("virtual rectangle save = %q, %v", data, err)
	}
}

func TestRectangleInvalidBatchLeavesDocumentUntouched(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("ab\ncd")
	doc.Editor.ClearHistory()
	for _, rectangle := range []Rectangle{{Anchor: -1, Caret: 2, TabWidth: 4}, {Anchor: 1, Caret: 100, TabWidth: 4}, {Anchor: 1, Caret: 2, TabWidth: 0}} {
		if changed, err := doc.EditRectangle(rectangle, ColumnEdit{Text: "X"}); err == nil || changed || doc.Editor.String() != "ab\ncd" || doc.Editor.CanUndo() {
			t.Fatalf("invalid rectangle %+v changed document: %v, %v, %q", rectangle, changed, err, doc.Editor.String())
		}
	}
}

func TestRectangleDeleteSelectedAndZeroWidthGraphemes(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("ab\ncd\nef")
	doc.Editor.ClearHistory()
	changed, err := doc.DeleteRectangle(Rectangle{Anchor: 0, Caret: 4, TabWidth: 4}, true)
	if err != nil || !changed || doc.Editor.String() != "b\nd\nef" {
		t.Fatalf("delete selected rectangle = %q, %v, %v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "ab\ncd\nef" || doc.Editor.CanUndo() {
		t.Fatalf("delete selected rectangle Undo = %q", doc.Editor.String())
	}
	doc = New("new 2")
	doc.Editor.Insert("a\u0301b\ncx")
	doc.Editor.ClearHistory()
	changed, err = doc.DeleteRectangle(Rectangle{Anchor: 2, Caret: 5, TabWidth: 4}, true)
	if err != nil || !changed || doc.Editor.String() != "b\nx" {
		t.Fatalf("grapheme rectangle Backspace = %q, %v, %v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\u0301b\ncx" {
		t.Fatalf("grapheme rectangle Undo = %q", doc.Editor.String())
	}
}

func TestRectangleCopyUsesRawRowsAndLeavesZeroWidthEmpty(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("abcde\nxy\n猫123\nlast")
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	value, err := doc.RectangleText(Rectangle{Anchor: 2, Caret: 12, TabWidth: 4})
	if err != nil || value != "cd\n\n12\n" || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("rectangle copy = %q, %v", value, err)
	}
	value, err = doc.RectangleText(Rectangle{Anchor: 2, Caret: 11, CaretVirtual: 0, TabWidth: 4})
	if err != nil || value == "" {
		t.Fatalf("nonempty rectangle unexpectedly empty: %q, %v", value, err)
	}
	value, err = doc.RectangleText(Rectangle{Anchor: 2, Caret: 10, TabWidth: 4})
	if err != nil || value != "" {
		t.Fatalf("zero-width rectangle copy = %q, %v", value, err)
	}
	one := New("new 2")
	one.Editor.Insert("abc")
	if value, err := one.RectangleText(Rectangle{Anchor: 0, Caret: 1, TabWidth: 4}); err != nil || value != "a" {
		t.Fatalf("single-row rectangle copy = %q, %v", value, err)
	}
}

func TestRectangleTabBoundariesSnapToNearestCharacter(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("a\tb\nab")
	doc.Editor.ClearHistory()
	rect := Rectangle{Anchor: 6, Caret: 1, TabWidth: 4}
	rows := doc.RectangleRows(rect)
	if len(rows) != 2 || rows[0].Range != (text.Range{Start: 1, End: 1}) || rows[0].LeftPadding != 0 ||
		rows[1].Range != (text.Range{Start: 5, End: 6}) {
		t.Fatalf("nearest tab boundaries = %+v", rows)
	}
	if copied, err := doc.RectangleText(rect); err != nil || copied != "\nb\n" {
		t.Fatalf("tab rectangle copy = %q, %v", copied, err)
	}
	if changed, err := doc.EditRectangle(rect, ColumnEdit{Text: "X"}); err != nil || !changed || doc.Editor.String() != "aX\tb\naX" {
		t.Fatalf("tab rectangle edit = %q, %v, %v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "a\tb\nab" || doc.Editor.CanUndo() {
		t.Fatalf("tab rectangle Undo = %q", doc.Editor.String())
	}
}

func TestRectangleCopyUsesDocumentEOLAndKeepsWholeSelectedTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tab.txt")
	if err := os.WriteFile(path, []byte("a\tb\r\nabc"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := doc.RectangleText(Rectangle{Anchor: 1, Caret: 7, TabWidth: 4})
	if err != nil || value != "\t\r\nbc\r\n" {
		t.Fatalf("raw tab copy = %q, %v", value, err)
	}
}

func TestRectangleMultilinePasteDistributesAndExtendsRows(t *testing.T) {
	for _, scenario := range []struct {
		before, paste, want string
		anchor, caret       int
	}{
		{"ab\ncd\nef", "X\nY", "aXb\ncYd\nef", 1, 4},
		{"ab\ncd\nef", "ONE\r\nTWO\n", "aONE\ncTWO\nef", 1, 5},
		{"ab\ncd\nef", "X\nY\nZ", "aXb\ncYd\neZf", 1, 4},
		{"ab\nc", "X\nY\nZ", "aXb\ncY\n Z", 1, 4},
	} {
		doc := New("new 1")
		doc.Editor.Insert(scenario.before)
		doc.Editor.ClearHistory()
		changed, err := doc.PasteRectangleLines(Rectangle{Anchor: scenario.anchor, Caret: scenario.caret, TabWidth: 4}, scenario.paste)
		if err != nil || !changed || doc.Editor.String() != scenario.want {
			t.Fatalf("paste %q into %q = %q, %v, %v", scenario.paste, scenario.before, doc.Editor.String(), changed, err)
		}
		if !doc.Editor.Undo() || doc.Editor.String() != scenario.before || doc.Editor.CanUndo() {
			t.Fatalf("multiline paste Undo = %q", doc.Editor.String())
		}
	}
}

func TestRectangleMultilinePastePreservesMixedEOLWhenAppending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("ab\r\nc"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := doc.PasteRectangleLines(Rectangle{Anchor: 1, Caret: 4, TabWidth: 4}, "X\nY\nZ"); err != nil || !changed {
		t.Fatalf("mixed paste = %v, %v", changed, err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "aXb\r\ncY\r\n Z" {
		t.Fatalf("mixed paste save = %q, %v", data, err)
	}
}
