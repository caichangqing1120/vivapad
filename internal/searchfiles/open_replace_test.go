package searchfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func TestReplaceOpenBuffersKeepsDiskAndAllowsPerTabUndo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chinese.txt")
	original := []byte{0xd6, 0xd0}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := document.OpenWithEncoding(path, document.GB18030)
	if err != nil {
		t.Fatal(err)
	}
	untitled := document.New("new 1")
	untitled.Editor.Insert("中")
	plan, err := PreviewOpenReplace([]*document.Document{saved, untitled}, "中", "文", document.SearchOptions{})
	if err != nil || plan.Replacements != 2 || len(plan.Changes) != 2 || saved.Editor.String() != "中" || untitled.Editor.String() != "中" {
		t.Fatalf("open-buffer preview = %+v, %v", plan, err)
	}
	if changed, err := os.ReadFile(path); err != nil || string(changed) != string(original) {
		t.Fatalf("preview touched disk: %x, %v", changed, err)
	}
	if err := plan.Apply([]*document.Document{saved, untitled}); err != nil {
		t.Fatal(err)
	}
	if saved.Editor.String() != "文" || untitled.Editor.String() != "文" || !saved.Dirty() || !untitled.Dirty() {
		t.Fatalf("applied open-buffer replace = saved:%q new:%q", saved.Editor.String(), untitled.Editor.String())
	}
	if changed, err := os.ReadFile(path); err != nil || string(changed) != string(original) {
		t.Fatalf("in-memory apply wrote disk: %x, %v", changed, err)
	}
	if !saved.Editor.Undo() || saved.Editor.String() != "中" || !untitled.Editor.Undo() || untitled.Editor.String() != "中" {
		t.Fatalf("per-tab undo = saved:%q new:%q", saved.Editor.String(), untitled.Editor.String())
	}
}

func TestOpenBufferReplacementSupportsRegexCaptures(t *testing.T) {
	first, second := document.New("new 1"), document.New("new 2")
	first.Editor.Insert("item-12")
	second.Editor.Insert("item-34")
	plan, err := PreviewOpenReplace([]*document.Document{first, second}, `(item)-(\d+)`, `${2}:$1`, document.SearchOptions{Regex: true})
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("regex plan = %+v, %v", plan, err)
	}
	if err := plan.Apply([]*document.Document{first, second}); err != nil || first.Editor.String() != "12:item" || second.Editor.String() != "34:item" {
		t.Fatalf("regex apply = %q, %q, %v", first.Editor.String(), second.Editor.String(), err)
	}
}

func TestOpenBufferPreviewRejectsUnmappableAndInvalidPatterns(t *testing.T) {
	first := document.New("new 1")
	first.Editor.Insert("café")
	path := filepath.Join(t.TempDir(), "western.txt")
	if err := os.WriteFile(path, []byte{'c', 'a', 'f', 0xe9}, 0600); err != nil {
		t.Fatal(err)
	}
	second, err := document.OpenWithEncoding(path, document.Windows1252)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewOpenReplace([]*document.Document{first, second}, "café", "中文", document.SearchOptions{}); err == nil {
		t.Fatal("unmappable replacement was accepted")
	}
	if first.Editor.String() != "café" || second.Editor.String() != "café" {
		t.Fatal("failed preview changed a document")
	}
	if _, err := PreviewOpenReplace([]*document.Document{first, second}, "(", "x", document.SearchOptions{Regex: true}); err == nil {
		t.Fatal("invalid regex was accepted")
	}
	if _, err := PreviewOpenReplace([]*document.Document{first, second}, "", "x", document.SearchOptions{}); err == nil {
		t.Fatal("empty query was accepted")
	}
}

func TestOpenBufferApplyPreflightsEverySource(t *testing.T) {
	first, second := document.New("new 1"), document.New("new 2")
	first.Editor.Insert("cat")
	second.Editor.Insert("cat")
	plan, err := PreviewOpenReplace([]*document.Document{first, second}, "cat", "dog", document.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second.Editor.Insert(" local")
	if err := plan.Apply([]*document.Document{first, second}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale plan applied: %v", err)
	}
	if first.Editor.String() != "cat" || second.Editor.String() != "cat local" {
		t.Fatalf("preflight partially changed tabs: %q, %q", first.Editor.String(), second.Editor.String())
	}
	if err := plan.Apply([]*document.Document{first}); err == nil || first.Editor.String() != "cat" {
		t.Fatalf("closed tab was silently skipped: %v, %q", err, first.Editor.String())
	}
}

func TestOpenBufferReplaceRejectsStaleMixedEOLConversion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("cat\r\ncat\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewOpenReplace([]*document.Document{doc}, "cat", "dog", document.SearchOptions{})
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("mixed preview = %+v, %v", plan, err)
	}
	doc.SetLineEnding(document.CRLF)
	if err := plan.Apply([]*document.Document{doc}); err == nil || doc.Editor.String() != "cat\ncat\n" {
		t.Fatalf("stale mixed format plan applied: %q, %v", doc.Editor.String(), err)
	}
}
