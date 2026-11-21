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

func TestOpenBufferPreviewSkipsWholeDocumentEqualResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	for _, scenario := range []struct {
		name, original, query, replacement string
		options                            document.SearchOptions
	}{
		{name: "literal", original: "cat\r\ncat\n", query: "cat", replacement: "cat"},
		{name: "regex", original: "cat\r\ncat\n", query: `(cat)`, replacement: `$1`, options: document.SearchOptions{Regex: true}},
		{name: "net equal regex", original: "aaa\r\nend\n", query: `(a)a|(a)`, replacement: `$1$2$2`, options: document.SearchOptions{Regex: true}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(scenario.original), 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := document.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			before, selection := doc.Editor.String(), doc.Editor.Selection()
			plan, err := PreviewOpenReplace([]*document.Document{doc, doc}, scenario.query, scenario.replacement, scenario.options)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes) != 0 || plan.Replacements != 0 {
				t.Fatalf("equal-result document entered replacement plan: %+v", plan)
			}
			if err := plan.Apply([]*document.Document{doc, doc}); err != nil {
				t.Fatal(err)
			}
			if doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Dirty() || doc.Editor.CanUndo() {
				t.Fatalf("equal-result preview changed saved document: text=%q dirty=%t undo=%t", doc.Editor.String(), doc.Dirty(), doc.Editor.CanUndo())
			}
			if disk, err := os.ReadFile(path); err != nil || string(disk) != scenario.original {
				t.Fatalf("equal-result preview changed mixed-EOL disk: %q, %v", disk, err)
			}
		})
	}
}

func TestOpenBufferPreviewSkipsRestrictedEqualResultButKeepsChangedTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unchanged.txt")
	if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
		t.Fatal(err)
	}
	unchanged, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	unchanged.Editor.Restrict(func(rune) bool { return true }, 1)
	changed := document.New("changed")
	changed.Editor.Insert("cat dog")
	changed.Editor.SetSavePoint()
	plan, err := PreviewOpenReplace([]*document.Document{unchanged, changed, unchanged},
		`(cat|dog)`, "cat", document.SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Doc != changed || plan.Changes[0].Count != 2 ||
		len(plan.Changes[0].Edits) != 2 || plan.Replacements != 2 {
		t.Fatalf("preview lost changed targets or kept restricted equal result: %+v", plan)
	}
	if err := plan.Apply([]*document.Document{unchanged, changed, unchanged}); err != nil {
		t.Fatal(err)
	}
	if unchanged.Editor.String() != "cat" || unchanged.Dirty() || unchanged.Editor.CanUndo() ||
		changed.Editor.String() != "cat cat" || !changed.Dirty() {
		t.Fatalf("mixed apply changed untouched source or lost target edit: unchanged=%q changed=%q", unchanged.Editor.String(), changed.Editor.String())
	}
	if !changed.Editor.Undo() || changed.Editor.String() != "cat dog" || !changed.Editor.Redo() || changed.Editor.String() != "cat cat" {
		t.Fatalf("changed document did not retain one target batch: %q", changed.Editor.String())
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "cat" {
		t.Fatalf("mixed apply wrote skipped disk: %q, %v", disk, err)
	}
}

func TestOpenBufferPreviewStillRefusesChangedRestrictedSourceAtomically(t *testing.T) {
	unchanged, changed := document.New("unchanged"), document.New("changed")
	unchanged.Editor.Insert("cat")
	changed.Editor.Insert("cat dog")
	unchanged.Editor.SetSavePoint()
	changed.Editor.SetSavePoint()
	unchangedDirty, changedDirty := unchanged.Dirty(), changed.Dirty()
	unchangedUndo, changedUndo := unchanged.Editor.CanUndo(), changed.Editor.CanUndo()
	plan, err := PreviewOpenReplace([]*document.Document{unchanged, changed},
		`(cat|dog)`, "cat", document.SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	changed.Editor.Restrict(func(rune) bool { return true }, 1)
	if err := plan.Apply([]*document.Document{unchanged, changed}); err == nil {
		t.Fatal("changed restricted source was applied")
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Doc != changed ||
		unchanged.Editor.String() != "cat" || changed.Editor.String() != "cat dog" ||
		unchanged.Dirty() != unchangedDirty || changed.Dirty() != changedDirty ||
		unchanged.Editor.CanUndo() != unchangedUndo || changed.Editor.CanUndo() != changedUndo {
		t.Fatalf("refused mixed plan changed a source: plan=%+v unchanged=%q changed=%q", plan, unchanged.Editor.String(), changed.Editor.String())
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

func TestOpenBufferPreviewKeepsMixedLineEndingsForSavePreflight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	const original = "first\rmiddle\nlast"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	mixed, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	plain := document.New("plain")
	plain.Editor.Insert("prefix middle suffix")
	docs := []*document.Document{plain, mixed}
	plan, err := PreviewOpenReplace(docs, "middle", "", document.SearchOptions{})
	if plan != nil || err == nil || !strings.Contains(err.Error(), "CR and LF") {
		t.Fatalf("lossy mixed-EOL preview = %+v, %v", plan, err)
	}
	if plain.Editor.String() != "prefix middle suffix" || mixed.Editor.String() != "first\nmiddle\nlast" || mixed.EOLSequence() != "ML" || mixed.Dirty() || mixed.Editor.CanUndo() {
		t.Fatal("refused preview changed a live document")
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != original {
		t.Fatalf("refused preview touched disk: %q, %v", disk, err)
	}
	plan, err = PreviewOpenReplace(docs, "middle", "kept", document.SearchOptions{})
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("safe preview = %+v, %v", plan, err)
	}
	if err := plan.Apply(docs); err != nil || mixed.EOLSequence() != "ML" {
		t.Fatalf("safe replacement lost mixed separators: %q, %v", mixed.EOLSequence(), err)
	}
	if err := mixed.Save(""); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "first\rkept\nlast" {
		t.Fatalf("saved mixed bytes = %q, %v", disk, err)
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

func TestOpenBufferReplaceRejectsReplacedEditorWithEqualText(t *testing.T) {
	root := t.TempDir()
	open := func(name string) *document.Document {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
		doc, err := document.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	first, second := open("first.txt"), open("second.txt")
	docs := []*document.Document{first, second}
	plan, err := PreviewOpenReplace(docs, "cat", "dog", document.SearchOptions{})
	if err != nil || plan.Replacements != 2 {
		t.Fatalf("preview = %+v, %v", plan, err)
	}
	original := second.Editor
	second.Editor = open("substitute.txt").Editor
	if err := plan.Apply(docs); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("plan did not reject the replaced editor: %v", err)
	}
	if first.Editor.String() != "cat" || second.Editor.String() != "cat" || original.String() != "cat" || first.Dirty() || second.Dirty() {
		t.Fatal("refused plan changed a buffer or its saved state")
	}
	second.Editor = original
	if err := plan.Apply(docs); err != nil || first.Editor.String() != "dog" || second.Editor.String() != "dog" {
		t.Fatalf("plan could not be retried on the original editors: %v", err)
	}
	for _, doc := range docs {
		if disk, err := os.ReadFile(doc.Path); err != nil || string(disk) != "cat" {
			t.Fatalf("%s disk = %q, %v", doc.Name, disk, err)
		}
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
