package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"vivapad/internal/document"
)

func encodeMixedMoveFixture(t *testing.T, value string, format document.Encoding) []byte {
	t.Helper()
	var data []byte
	var err error
	switch format {
	case document.UTF8:
		return []byte(value)
	case document.UTF8BOM:
		return append([]byte{0xef, 0xbb, 0xbf}, value...)
	case document.UTF16LE:
		data, err = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(value))
	case document.UTF16BE:
		data, err = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(value))
	case document.Windows1252:
		data, err = charmap.Windows1252.NewEncoder().Bytes([]byte(value))
	default:
		t.Fatalf("unsupported mixed-EOL fixture encoding %d", format)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMoveLinesKeepsMixedEOLBytesWithMovedLine(t *testing.T) {
	for _, test := range []struct {
		name, original, moved string
		selection             text.Range
		up                    bool
	}{
		{"middle down", "a\r\nb\nc\rd", "a\r\nc\rb\nd", text.Range{Start: 2, End: 2}, false},
		{"middle up", "a\r\nb\nc\rd", "b\na\r\nc\rd", text.Range{Start: 2, End: 2}, true},
		{"following line up", "a\r\nb\nc\rd", "a\r\nc\rb\nd", text.Range{Start: 4, End: 4}, true},
		{"two-line block up", "a\r\nb\nc\rd", "b\nc\ra\r\nd", text.Range{Start: 2, End: 5}, true},
		{"identical first down", "same\r\nsame\nend", "same\nsame\r\nend", text.Range{Start: 0, End: 0}, false},
		{"identical second up", "same\r\nsame\nend", "same\nsame\r\nend", text.Range{Start: 5, End: 5}, true},
		{"unterminated EOF", "a\r\nb\nc", "a\r\nc\r\nb", text.Range{Start: 2, End: 2}, false},
		{"Unicode up", "猫\r\n犬\nend", "犬\n猫\r\nend", text.Range{Start: 2, End: 2}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "mixed.txt")
			original, moved := []byte(test.original), []byte(test.moved)
			if err := os.WriteFile(file, original, 0o600); err != nil {
				t.Fatal(err)
			}
			state := newStartupModel([]string{file}, filepath.Join(root, "session.json"))
			doc := state.space.Current()
			copyBytes := func() []byte {
				t.Helper()
				copyPath := filepath.Join(root, "copy.txt")
				if err := doc.SaveCopy(copyPath); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(copyPath)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			doc.Editor.SetSelection(test.selection)
			state.moveLinesWithBookmarks(doc, test.up)
			if !doc.Dirty() || doc.Editor.AtSavePoint() {
				t.Fatal("line move with changed physical separators stayed at the saved boundary")
			}
			if got := copyBytes(); !bytes.Equal(got, moved) {
				t.Fatalf("moved physical separators = %q, want %q (labels %q)", got, moved, doc.EOLSequence())
			}
			if !doc.Editor.Undo() || !bytes.Equal(copyBytes(), original) || doc.Dirty() || !doc.Editor.AtSavePoint() {
				t.Fatal("Undo did not restore source separators")
			}
			if !doc.Editor.Redo() || !bytes.Equal(copyBytes(), moved) || !doc.Dirty() || doc.Editor.AtSavePoint() {
				t.Fatal("Redo did not restore moved-line separators")
			}
		})
	}
}

func TestMoveLinesMixedEOLRefusalLeavesSourceUntouched(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "mixed.txt")
	original := []byte("a\r\nb\nc\rd")
	if err := os.WriteFile(file, original, 0o600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{file}, filepath.Join(root, "session.json"))
	doc := state.space.Current()
	doc.Editor.MoveTo(2, false)
	beforeSelection, beforeLabels := doc.Editor.Selection(), doc.EOLSequence()
	doc.Editor.Restrict(nil, 1)
	state.moveLinesWithBookmarks(doc, false)
	copyPath := filepath.Join(root, "copy.txt")
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(copyPath)
	if err != nil || !bytes.Equal(data, original) || doc.EOLSequence() != beforeLabels ||
		doc.Editor.Selection() != beforeSelection || doc.Dirty() || doc.Editor.CanUndo() ||
		!strings.Contains(state.notice, "restricted") {
		t.Fatalf("rejected move changed source: bytes=%q labels=%q selection=%+v dirty=%v undo=%v notice=%q err=%v",
			data, doc.EOLSequence(), doc.Editor.Selection(), doc.Dirty(), doc.Editor.CanUndo(), state.notice, err)
	}
}

func TestMoveLinesRefusesAnUnboundEOLSourceBeforeEditing(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "mixed.txt")
	if err := os.WriteFile(file, []byte("a\r\nb\nc\rd"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{file}, filepath.Join(root, "session.json"))
	doc := state.space.Current()
	doc.Editor = text.NewEditor(doc.Editor.String())
	if !doc.Editor.TrackLFLabels([]byte("WLM"), 'W', nil) {
		t.Fatal("replacement editor setup failed")
	}
	doc.Editor.MoveTo(2, false)
	state.moveLinesWithBookmarks(doc, false)
	if doc.Editor.String() != "a\nb\nc\nd" || doc.Editor.CanUndo() || !strings.Contains(state.notice, "no live EOL label source") {
		t.Fatalf("unbound source was edited: text=%q undo=%v notice=%q", doc.Editor.String(), doc.Editor.CanUndo(), state.notice)
	}
}

func TestMoveLinesMixedEOLPreservesPhysicalBytesAcrossEncodings(t *testing.T) {
	for _, test := range []struct {
		name            string
		format          document.Encoding
		original, moved string
		caret           int
		explicit        bool
	}{
		{"UTF-8", document.UTF8, "a\r\nb\nc\rd", "a\r\nc\rb\nd", 2, false},
		{"UTF-8 BOM", document.UTF8BOM, "a\r\nb\nc\rd", "a\r\nc\rb\nd", 2, false},
		{"UTF-16 LE", document.UTF16LE, "猫\r\n犬\nc\rd", "猫\r\nc\r犬\nd", 2, false},
		{"UTF-16 BE", document.UTF16BE, "猫\r\n犬\nc\rd", "猫\r\nc\r犬\nd", 2, false},
		{"Windows-1252", document.Windows1252, "café\r\nniño\nx\rd", "café\r\nx\rniño\nd", 5, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "mixed.txt")
			original := encodeMixedMoveFixture(t, test.original, test.format)
			moved := encodeMixedMoveFixture(t, test.moved, test.format)
			if err := os.WriteFile(file, original, 0o600); err != nil {
				t.Fatal(err)
			}
			var state *model
			if test.explicit {
				state = newModel()
				if err := state.space.OpenWithEncoding(file, test.format); err != nil {
					t.Fatal(err)
				}
			} else {
				state = newStartupModel([]string{file}, filepath.Join(root, "session.json"))
			}
			doc := state.space.Current()
			if doc.Encoding() != test.format {
				t.Fatalf("opened encoding = %d, want %d", doc.Encoding(), test.format)
			}
			copyBytes := func() []byte {
				t.Helper()
				copyPath := filepath.Join(root, "copy.txt")
				if err := doc.SaveCopy(copyPath); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(copyPath)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			if data := copyBytes(); !bytes.Equal(data, original) {
				t.Fatalf("unmodified Save Copy = %x, want %x", data, original)
			}
			doc.Editor.MoveTo(test.caret, false)
			state.moveLinesWithBookmarks(doc, false)
			if data := copyBytes(); !bytes.Equal(data, moved) || !doc.Dirty() {
				t.Fatalf("moved Save Copy = %x, want %x, dirty=%v, notice=%q", data, moved, doc.Dirty(), state.notice)
			}
			if !doc.Editor.Undo() || !bytes.Equal(copyBytes(), original) || doc.Dirty() {
				t.Fatal("Undo did not restore the original encoding and separators")
			}
			if !doc.Editor.Redo() || !bytes.Equal(copyBytes(), moved) || !doc.Dirty() {
				t.Fatal("Redo did not restore the moved encoding and separators")
			}
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, moved) || doc.Dirty() {
				t.Fatalf("saved move = %x, want %x, dirty=%v, err=%v", data, moved, doc.Dirty(), err)
			}
			if !doc.Editor.Undo() || !bytes.Equal(copyBytes(), original) || !doc.Dirty() {
				t.Fatal("Undo after Save did not recover original bytes and dirty state")
			}
			if !doc.Editor.Redo() || !bytes.Equal(copyBytes(), moved) || doc.Dirty() {
				t.Fatal("Redo after Save did not recover moved bytes and saved boundary")
			}
		})
	}
}
