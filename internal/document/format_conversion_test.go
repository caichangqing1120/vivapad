package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBatchFormatUndoInterleavesWithTextUndoAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("A\nB\rC\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("x")
	if err := doc.ConvertFormat(UTF8BOM, new(LF)); err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("y")
	if got := doc.Editor.String(); got != "xyA\nB\nC\n" {
		t.Fatalf("conversion moved the caret: %q", got)
	}
	if !doc.Undo() || doc.Editor.String() != "xA\nB\nC\n" || doc.Encoding() != UTF8BOM {
		t.Fatal("text edit should undo before the format edit")
	}
	if !doc.Undo() || doc.Editor.String() != "xA\nB\nC\n" || doc.Encoding() != UTF8 || doc.EOLSequence() != "LMW" {
		t.Fatalf("format undo failed: %+v", doc.FileFormat())
	}
	if !doc.Undo() || doc.Dirty() {
		t.Fatal("undo to original text and format should be clean")
	}
	if !doc.Redo() || !doc.Redo() || doc.EOLSequence() != "" || doc.Encoding() != UTF8BOM || !doc.Dirty() {
		t.Fatal("format redo failed")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "\xef\xbb\xbfxA\nB\nC\n" {
		t.Fatalf("save after conversion: %q, %v", got, err)
	}
	if !doc.Undo() || !doc.Dirty() || !doc.Redo() || doc.Dirty() {
		t.Fatal("format undo/redo did not respect the new save point")
	}
}

func TestBatchFormatUndoForEmptyBufferAndBranchedRedo(t *testing.T) {
	doc := New("untitled")
	if err := doc.ConvertFormat(UTF16LE, new(CR)); err != nil {
		t.Fatal(err)
	}
	if !doc.CanUndo() || !doc.Undo() || doc.Dirty() || doc.Encoding() != UTF8 || doc.LineEnding() != LF || !doc.CanRedo() {
		t.Fatal("empty buffer format edit does not undo cleanly")
	}
	doc.Editor.Insert("abc")
	if doc.CanRedo() || !doc.Undo() || !doc.CanRedo() {
		t.Fatal("new text should discard format redo but retain its own redo")
	}
	if err := doc.ConvertFormat(GB18030, nil); err != nil {
		t.Fatal(err)
	}
	if doc.CanRedo() || !doc.Undo() || !doc.Redo() || doc.CanRedo() || doc.Editor.String() != "" || doc.Encoding() != GB18030 {
		t.Fatal("format edit should replace the text redo branch")
	}
}

func TestBatchFormatKeepEncodingRejectsUnrepresentableText(t *testing.T) {
	doc := New("untitled")
	doc.SetEncoding(Windows1252)
	doc.Editor.Insert("中\n")
	if err := doc.ConvertFormat(Windows1252, new(CRLF)); err == nil {
		t.Fatal("conversion accepted output this encoding cannot save")
	}
	if doc.LineEnding() != LF || doc.Encoding() != Windows1252 || !doc.Undo() || doc.Editor.String() != "" {
		t.Fatal("rejected conversion changed format or undo history")
	}
}
