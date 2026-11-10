package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestDirtyFollowsSavedUndoBoundaryNotOnlyTextEquality(t *testing.T) {
	doc := New("point.txt")
	doc.Editor.Insert("base")
	path := filepath.Join(t.TempDir(), "point.txt")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("!")
	doc.Editor.DeleteBackward()
	if doc.Editor.String() != "base" || !doc.Dirty() {
		t.Fatal("net-zero edits were falsely treated as the saved Undo position")
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "base!" || !doc.Dirty() || !doc.Editor.Undo() || doc.Dirty() {
		t.Fatal("Undo did not cross the saved boundary correctly")
	}
	if !doc.Editor.Redo() || !doc.Dirty() || !doc.Editor.Redo() || doc.Editor.String() != "base" || !doc.Dirty() {
		t.Fatal("Redo confused equal bytes with a save point")
	}
}

func TestSavePointSaveCopyAndFailureDoNotResetTheSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.txt")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("!")
	doc.Editor.DeleteBackward()
	if err := doc.SaveCopy(filepath.Join(root, "copy.txt")); err != nil || !doc.Dirty() {
		t.Fatal("Save Copy marked equal-result edits saved")
	}
	if err := doc.Save(filepath.Join(root, "missing", "failed.txt")); err == nil || !doc.Dirty() {
		t.Fatal("failed Save moved the source save point")
	}
	if err := doc.Save(path); err != nil || doc.Dirty() {
		t.Fatal("successful Save did not mark the current boundary")
	}
	if !doc.Editor.Undo() || !doc.Dirty() || !doc.Editor.Redo() || doc.Dirty() {
		t.Fatal("Save deleted history or lost the newly saved boundary")
	}
}

func TestEqualTargetsAreDirtyUndoableAndDoNotChangeSavedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equal.txt")
	if err := os.WriteFile(path, []byte("cat keep cat"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	count, _, err := doc.ReplaceAllWithEdits("cat", "cat", SearchOptions{})
	if err != nil || count != 2 || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatal("equal replacement did not retain its real history")
	}
	if saved, err := os.ReadFile(path); err != nil || string(saved) != "cat keep cat" {
		t.Fatal("in-memory equal targets changed disk bytes")
	}
	if !doc.Editor.Undo() || doc.Dirty() || !doc.Editor.Redo() || !doc.Dirty() {
		t.Fatal("equal target Undo/Redo lost the save point")
	}
}

func TestSavePointBoundaryDoesNotCoalesceTypingAcrossSave(t *testing.T) {
	doc := New("boundary.txt")
	doc.Editor.Insert("a")
	if err := doc.Save(filepath.Join(t.TempDir(), "boundary.txt")); err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("b")
	doc.Editor.Insert("c")
	if !doc.Editor.Undo() || doc.Editor.String() != "a" || doc.Dirty() {
		t.Fatal("typing after Save coalesced into the saved history unit")
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "" || !doc.Dirty() || !doc.Editor.Redo() || doc.Dirty() {
		t.Fatal("saved boundary was lost while navigating older history")
	}
}

func TestSavePointResetKeepsSavedByteAndFormatSafeguards(t *testing.T) {
	doc := New("reset.txt")
	doc.Editor.Insert("a")
	if err := doc.Save(filepath.Join(t.TempDir(), "reset.txt")); err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{End: 1}, "b")
	doc.Editor.ClearHistory()
	if !doc.Editor.AtSavePoint() || !doc.Dirty() {
		t.Fatal("history reset bypassed the document saved-byte safeguard")
	}
	doc.Editor.ReplaceRange(text.Range{End: 1}, "a")
	doc.Editor.ClearHistory()
	if doc.Dirty() {
		t.Fatal("matching bytes at an explicitly reset point remained dirty")
	}
	doc.SetUTF8BOM(true)
	if !doc.Dirty() {
		t.Fatal("save point hid a pending format change")
	}
	doc.SetUTF8BOM(false)
	doc.MarkRecovered()
	if !doc.Dirty() {
		t.Fatal("save point hid recovered-buffer protection")
	}
}
