package document

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/language"
)

func applySourceLineAction(doc *Document, action string, offsets []int) bool {
	switch action {
	case "join":
		return doc.JoinLines()
	case "split":
		return doc.SplitLinesAt(offsets)
	case "empty":
		return doc.RemoveEmptyLines(false) != 0
	case "blank":
		return doc.RemoveEmptyLines(true) != 0
	case "consecutive":
		return doc.RemoveConsecutiveDuplicateLines() != 0
	case "unique":
		return doc.RemoveDuplicateLines() != 0
	case "reverse":
		return doc.ReverseLines()
	case "sort-asc":
		return doc.SortLines(true)
	case "sort-desc":
		return doc.SortLines(false)
	case "sort-ignore":
		return doc.SortLinesIgnoreCase(true)
	case "sort-length":
		changed, err := doc.SortLinesMode(SortLength, true)
		return changed && err == nil
	case "sort-integer":
		changed, err := doc.SortLinesMode(SortIntegerAware, true)
		return changed && err == nil
	case "locale", "locale-equal":
		return doc.SortLinesLocale(language.English, true)
	case "above", "below":
		before := doc.Editor.String()
		if action == "above" {
			doc.InsertBlankLineAbove()
		} else {
			doc.InsertBlankLineBelow()
		}
		return doc.Editor.String() != before
	}
	return false
}

func TestLineTargetsMatchSourceEditScopesAndAtomicHistory(t *testing.T) {
	for _, scenario := range []struct {
		name, action, before, after string
		selection                   text.Range
		offsets                     []int
		want                        []text.Edit
	}{
		{"split", "split", "a b c\nz", "a \nb \nc\nz", text.Range{}, []int{2, 4, 6}, []text.Edit{{Start: 2, Inserted: 1}, {Start: 5, Inserted: 1}}},
		{"join", "join", "a\nb\nc\n", "a b\nc\n", text.Range{Start: 0, End: 4}, nil, []text.Edit{{Start: 1, Removed: 1, Inserted: 1}}},
		{"empty", "empty", "a\n\nb\n", "a\nb", text.Range{}, nil, []text.Edit{{Start: 2, Removed: 1}, {Start: 3, Removed: 1}}},
		{"blank", "blank", "a\n \t\nb\n", "a\nb", text.Range{}, nil, []text.Edit{{Start: 2, Removed: 3}, {Start: 3, Removed: 1}}},
		{"consecutive eof", "consecutive", "a\na\na", "a", text.Range{}, nil, []text.Edit{{Start: 2, Removed: 2}, {Start: 0, Removed: 2}}},
		{"whole sort", "sort-asc", "z\nb\na\n", "\na\nb\nz", text.Range{}, nil, []text.Edit{{Start: 0, Removed: 6, Inserted: 6}}},
		{"whole reverse", "reverse", "z\nb\na\n", "\na\nb\nz", text.Range{}, nil, []text.Edit{{Start: 0, Removed: 6, Inserted: 6}}},
		{"whole unique", "unique", "b\n\nx\nb\n", "b\n\nx", text.Range{}, nil, []text.Edit{{Start: 0, Removed: 7, Inserted: 4}}},
		{"above", "above", "a\nb", "a\n\nb", text.Range{Start: 2, End: 2}, nil, []text.Edit{{Start: 1, Inserted: 1}}},
		{"below", "below", "a\nb", "a\n\nb", text.Range{}, nil, []text.Edit{{Start: 2, Inserted: 1}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			doc := New("line-targets.txt")
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			doc.Editor.SetSelection(scenario.selection)
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			cursor := doc.Editor.TrackEdits()
			if !applySourceLineAction(doc, scenario.action, scenario.offsets) || doc.Editor.String() != scenario.after {
				t.Fatalf("result = %q, want %q", doc.Editor.String(), scenario.after)
			}
			edits, _, complete := doc.Editor.EditsSince(cursor, nil)
			if !complete || !slices.Equal(edits, scenario.want) {
				t.Fatalf("line primitives = %+v complete=%v, want %+v", edits, complete, scenario.want)
			}
			afterSelection, afterCaret := doc.Editor.Selection(), doc.Editor.Caret()
			if !doc.Editor.Undo() || doc.Editor.String() != scenario.before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() {
				t.Fatal("line action did not Undo atomically to its original selection")
			}
			if !doc.Editor.Redo() || doc.Editor.String() != scenario.after || doc.Editor.Selection() != afterSelection || doc.Editor.Caret() != afterCaret {
				t.Fatal("line action Redo lost the final selection")
			}
		})
	}
}

func TestSourceLineCommandsExpandSelectionWithoutInventingHistory(t *testing.T) {
	doc := New("no-break.txt")
	doc.Editor.Insert("abc\nnext")
	if err := doc.Save(filepath.Join(t.TempDir(), "no-break.txt")); err != nil {
		t.Fatal(err)
	}
	doc.Editor.Insert("!")
	doc.Editor.Undo()
	doc.Editor.MoveTo(1, false)
	if doc.SplitLinesAt(nil) || doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || doc.Editor.Caret() != 3 || !doc.Editor.CanRedo() || doc.Dirty() {
		t.Fatal("no-break Split did not perform the source selection-only action")
	}
	doc = New("sorted.txt")
	doc.Editor.Insert("a\nb\nx")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 1, End: 4})
	if doc.SortLines(true) || doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) || doc.Editor.Caret() != 3 || doc.Editor.CanUndo() {
		t.Fatal("no-op selected sort did not normalize source line content bounds without Undo")
	}
}

func TestJoinSplitFinalSelectionIsStoredInTheUndoUnit(t *testing.T) {
	for _, scenario := range []struct {
		action, before string
		initial        text.Range
		offsets        []int
		want           text.Range
	}{
		{"join", "aa keep\nbb keep\nz", text.Range{Start: 11, End: 2}, nil, text.Range{Start: 0, End: 15}},
		{"split", "a b c\nz", text.Range{Start: 3, End: 1}, []int{2, 4}, text.Range{Start: 0, End: 7}},
	} {
		doc := New("selection.txt")
		doc.Editor.Insert(scenario.before)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(scenario.initial)
		if !applySourceLineAction(doc, scenario.action, scenario.offsets) || doc.Editor.Selection() != scenario.want || doc.Editor.Caret() != scenario.want.End {
			t.Fatalf("%s source selection = %+v caret=%d, want %+v", scenario.action, doc.Editor.Selection(), doc.Editor.Caret(), scenario.want)
		}
		if !doc.Editor.Undo() || !doc.Editor.Redo() || doc.Editor.Selection() != scenario.want || doc.Editor.Caret() != scenario.want.End {
			t.Fatal("line source selection was applied outside its history unit")
		}
	}
}

func TestBlankLineFinalCaretIsRetainedByRedo(t *testing.T) {
	for _, scenario := range []struct {
		action string
		caret  int
		want   int
	}{
		{"above", 2, 0},
		{"below", 1, 4},
	} {
		doc := New("blank.txt")
		doc.Editor.Insert("abc\nnext")
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(scenario.caret, false)
		if !applySourceLineAction(doc, scenario.action, nil) || doc.Editor.Caret() != scenario.want {
			t.Fatal("blank line did not place the source caret")
		}
		if !doc.Editor.Undo() || doc.Editor.Caret() != scenario.caret || !doc.Editor.Redo() || doc.Editor.Caret() != scenario.want {
			t.Fatalf("%s Redo caret=%d, want %d", scenario.action, doc.Editor.Caret(), scenario.want)
		}
	}
}

func TestSourceOrderingUsesUTF16KeysAndActualEOFLine(t *testing.T) {
	for _, scenario := range []struct {
		action, before, after string
		selection             text.Range
	}{
		{"sort-asc", "\ue000\n\U00010000", "\U00010000\n\ue000", text.Range{}},
		{"sort-integer", "x\ue000\nx\U00010000", "x\U00010000\nx\ue000", text.Range{}},
		{"sort-asc", "z\n", "\nz", text.Range{}},
		{"sort-asc", "z\nb\na\n", "a\nb\nz\n", text.Range{Start: 0, End: 6}},
		{"unique", "b\n\nx\nb\n", "b\n\nx\n", text.Range{Start: 0, End: 7}},
	} {
		doc := New("ordering.txt")
		doc.Editor.Insert(scenario.before)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(scenario.selection)
		if !applySourceLineAction(doc, scenario.action, nil) || doc.Editor.String() != scenario.after {
			t.Fatalf("%s = %q, want %q", scenario.action, doc.Editor.String(), scenario.after)
		}
	}
}

func TestRemoveBlankUsesASCIIAndExactSelectionThenTerminalCleanup(t *testing.T) {
	doc := New("blank.txt")
	doc.Editor.Insert("a\n\u00a0\nb")
	doc.Editor.ClearHistory()
	if doc.RemoveEmptyLines(true) != 0 || doc.Editor.String() != "a\n\u00a0\nb" || doc.Editor.CanUndo() {
		t.Fatal("Remove Blank deleted non-ASCII whitespace")
	}
	doc = New("partial.txt")
	doc.Editor.Insert("head\n  \nbody")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 5, End: 7})
	if doc.RemoveEmptyLines(true) != 0 || doc.Editor.String() != "head\n  \nbody" || doc.Editor.CanUndo() {
		t.Fatal("partial blank selection was expanded through an unselected EOL")
	}
	doc = New("terminal.txt")
	doc.Editor.Insert("a\n")
	doc.Editor.ClearHistory()
	if doc.RemoveEmptyLines(false) != 1 || doc.Editor.String() != "a" || !doc.Editor.Undo() || doc.Editor.String() != "a\n" {
		t.Fatal("source terminal empty line was artificially preserved")
	}
}

func TestLineTargetsRefuseCompositionAndRestrictionsAtomically(t *testing.T) {
	for _, setup := range []struct {
		name  string
		apply func(*text.Editor)
	}{
		{"composition", func(editor *text.Editor) { editor.SetMarkedText("ni\nx", text.Range{Start: 0, End: 4}) }},
		{"restricted", func(editor *text.Editor) { editor.Restrict(unicode.IsLetter, 100) }},
	} {
		for _, action := range []string{"join", "split", "empty", "blank", "consecutive", "unique", "sort-asc", "sort-ignore", "sort-length", "sort-integer", "reverse", "locale", "above", "below"} {
			doc := New("refused.txt")
			doc.Editor.Insert("z\nz\n\n \t\na\n")
			doc.Editor.ClearHistory()
			doc.Editor.MoveTo(0, false)
			setup.apply(doc.Editor)
			before, selection, caret := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.Caret()
			composition, active := doc.Editor.Composition()
			canUndo, canRedo := doc.Editor.CanUndo(), doc.Editor.CanRedo()
			cursor := doc.Editor.TrackEdits()
			if applySourceLineAction(doc, action, []int{1, 4, 8}) || doc.Editor.String() != before || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.CanUndo() != canUndo || doc.Editor.CanRedo() != canRedo {
				t.Fatalf("%s %s partially edited a refused source", setup.name, action)
			}
			if got, composing := doc.Editor.Composition(); got != composition || composing != active {
				t.Fatal("refused line command changed composition")
			}
			if edits, _, complete := doc.Editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
				t.Fatal("refused line command added primitives")
			}
		}
	}
}

func TestLocaleSortRetainsSourceEqualReplacementAndCaretOwnership(t *testing.T) {
	doc := New("locale.txt")
	doc.Editor.Insert("a\nb\n")
	if err := doc.Save(filepath.Join(t.TempDir(), "locale.txt")); err != nil {
		t.Fatal(err)
	}
	if !doc.SortLinesLocale(language.English, true) || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatal("eligible equal Locale sort skipped the original accepted replacement")
	}
	if !doc.Editor.Undo() || doc.Dirty() || !doc.Editor.Redo() || !doc.Dirty() {
		t.Fatal("equal Locale replacement lost its saved Undo boundary")
	}
	doc = New("caret.txt")
	doc.Editor.Insert("zeta\nalpha\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(2, false)
	if !doc.SortLinesLocale(language.English, true) || doc.Editor.String() != "alpha\nzeta\n" || doc.Editor.Caret() != 8 {
		t.Fatal("Locale sort did not restore the source caret on its moved content line")
	}
	if !doc.Editor.Undo() || doc.Editor.Caret() != 2 || !doc.Editor.Redo() || doc.Editor.Caret() != 8 {
		t.Fatal("Locale caret restoration was outside its atomic history unit")
	}
}

func TestNumericSortFailureKeepsBytesButNormalizesSourceLineSelection(t *testing.T) {
	doc := New("numeric.txt")
	before := "head\n3.0\n-\n1.0\ntail"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 6, End: 13})
	changed, err := doc.SortLinesMode(SortDecimalDot, true)
	if changed || err == nil || doc.Editor.String() != before || doc.Editor.CanUndo() || doc.Editor.Selection() != (text.Range{Start: 5, End: 14}) || doc.Editor.Caret() != 14 {
		t.Fatalf("numeric refusal = changed=%v err=%v selection=%+v caret=%d", changed, err, doc.Editor.Selection(), doc.Editor.Caret())
	}
}

func BenchmarkSourceLineTargets(b *testing.B) {
	var offsets []int
	for position := 16; position < 16*256; position += 16 {
		offsets = append(offsets, position)
	}
	for _, scenario := range []struct {
		action string
		before string
	}{
		{"join", strings.Repeat("alpha keep beta\n", 256)},
		{"split", strings.Repeat("alpha keep beta ", 256) + "\nfooter"},
		{"empty", strings.Repeat("keep\n\n", 256)},
		{"consecutive", strings.Repeat("keep\nkeep\n", 128) + "keep"},
		{"unique", strings.Repeat("alpha\nbeta\n", 128)},
		{"sort-asc", strings.Repeat("zeta\nalpha\n", 128)},
	} {
		b.Run(scenario.action, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(scenario.before)))
			for b.Loop() {
				doc := New("line-benchmark.txt")
				doc.Editor.Insert(scenario.before)
				doc.Editor.ClearHistory()
				doc.Editor.MoveTo(0, false)
				if scenario.action == "join" {
					doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
				}
				if !applySourceLineAction(doc, scenario.action, offsets) {
					b.Fatal("line benchmark did not edit")
				}
			}
		})
	}
}
