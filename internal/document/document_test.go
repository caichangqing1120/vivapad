package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestOpenSavePreservesCRLFAndBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	input := "\xef\xbb\xbffirst\r\n中文\r\n"
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != "first\n中文\n" {
		t.Fatalf("editor text = %q", got)
	}
	if doc.Editor.Caret() != 0 {
		t.Fatalf("opened file caret = %d, want start", doc.Editor.Caret())
	}
	if doc.Dirty() {
		t.Fatal("newly opened document is dirty")
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("last")
	if !doc.Dirty() {
		t.Fatal("editing did not mark dirty")
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(output), input+"last"; got != want {
		t.Fatalf("saved = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 || doc.Dirty() {
		t.Fatalf("permissions = %v; dirty = %v", info.Mode().Perm(), doc.Dirty())
	}
}

func TestOpenRejectsBinaryAndInvalidUTF8(t *testing.T) {
	for name, data := range map[string][]byte{
		"binary": {'a', 0, 'b'}, "invalid": {0xff},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("accepted unsupported content")
			}
		})
	}
}

func TestLFWithoutFinalNewlineSurvivesSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.NewlineLabel() != "Unix (LF)" {
		t.Fatalf("newline label = %q", doc.NewlineLabel())
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("!")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if output, err := os.ReadFile(path); err != nil || string(output) != "one\ntwo!" {
		t.Fatalf("saved = %q, %v", output, err)
	}
}

func TestFailedSaveKeepsDocumentDirtyAndOriginalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("new")
	if err := doc.Save(filepath.Join(t.TempDir(), "missing", "new.txt")); err == nil {
		t.Fatal("save to a nonexistent directory succeeded")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" || !doc.Dirty() || doc.Path != canonical {
		t.Fatalf("original=%q, dirty=%v, path=%q", got, doc.Dirty(), doc.Path)
	}
}

func TestSaveRefusesToOverwriteExternallyChangedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.txt")
	if err := os.WriteFile(path, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert(" local")
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(""); err == nil {
		t.Fatal("overwrote changes made outside VivaPad")
	}
	if output, err := os.ReadFile(path); err != nil || string(output) != "external" || !doc.Dirty() {
		t.Fatalf("disk=%q, err=%v, dirty=%v", output, err, doc.Dirty())
	}
}

func TestSearchWrapsOverUnicodeAndReplaceUndoesAsOneEdit(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("alpha 中\nALPHA 中\n")
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	if !doc.Find("中", false) || doc.Editor.Selection().Start != 6 {
		t.Fatalf("forward selection = %+v", doc.Editor.Selection())
	}
	if !doc.Find("alpha", true) || doc.Editor.Selection().Start != 0 {
		t.Fatalf("backward selection = %+v", doc.Editor.Selection())
	}
	if count := doc.ReplaceAll("alpha", "beta"); count != 2 || doc.Editor.String() != "beta 中\nbeta 中\n" {
		t.Fatalf("replace count=%d text=%q", count, doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "alpha 中\nALPHA 中\n" {
		t.Fatalf("undo text = %q", doc.Editor.String())
	}
	if doc.Find("", false) || doc.ReplaceAll("", "x") != 0 || strings.Contains(doc.Editor.String(), "x") {
		t.Fatal("empty query must be inert")
	}
}

func TestReplaceCurrentOnlyChangesSelectedMatch(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("one two one")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	if !doc.ReplaceCurrent("ONE", "three") || doc.Editor.String() != "three two one" {
		t.Fatalf("replace text=%q", doc.Editor.String())
	}
	if doc.ReplaceCurrent("one", "x") {
		t.Fatal("replaced an unrelated selection")
	}
}

func TestPositionHandlesUnicodeWithoutAllocatingPerPaint(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert(strings.Repeat("long line content\n", 500) + "中😀")
	if line, column := doc.Position(); line != 501 || column != 3 {
		t.Fatalf("position = %d:%d", line, column)
	}
	if allocations := testing.AllocsPerRun(100, func() { doc.Position() }); allocations != 0 {
		t.Fatalf("position allocated %g times", allocations)
	}
}

func TestRecoveredUntitledEmptyBufferRemainsDirtyUntilSaved(t *testing.T) {
	doc := New("Recovered example.txt")
	doc.MarkRecovered()
	if !doc.Dirty() {
		t.Fatal("empty recovered buffer could be silently discarded")
	}
	path := filepath.Join(t.TempDir(), "recovered.txt")
	if err := doc.Save(path); err != nil || doc.Dirty() {
		t.Fatalf("saved recovery dirty=%v err=%v", doc.Dirty(), err)
	}
	if doc.DiskDigest() == "" {
		t.Fatal("saved file has no disk digest")
	}
}

func TestDiskDigestIsStableWithoutPerPaintAllocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("text", 10000)), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := doc.DiskDigest()
	if first == "" || testing.AllocsPerRun(100, func() { doc.DiskDigest() }) != 0 {
		t.Fatal("disk digest recomputed or allocated on each paint")
	}
}

func TestSaveRejectsPathReplacedWithSymlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "open.txt")
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("local ")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(""); err == nil {
		t.Fatal("saved through a swapped symlink")
	}
	if value, _ := os.ReadFile(target); string(value) != "external" || !doc.Dirty() {
		t.Fatalf("target=%q dirty=%v", value, doc.Dirty())
	}
}

func TestNewSaveStoresCanonicalPathThroughAliasedParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	doc := New("new 1")
	doc.Editor.Insert("saved")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path != canonical {
		t.Fatalf("saved path = %q, canonical = %q", doc.Path, canonical)
	}
}
