package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func BenchmarkMultiSelectionReplaceTwoRanges(b *testing.B) {
	suffix := strings.Repeat("z", 512<<10)
	initial := "a a " + suffix
	want := "X X " + suffix
	ranges := []text.Range{{Start: 0, End: 1}, {Start: 2, End: 3}}
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		state := newModel()
		doc := state.space.Current()
		doc.Editor.Insert(initial)
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(ranges[1])
		state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
		b.StartTimer()
		changed, err := state.editMultiSelection(0, "X", multiInsert)
		b.StopTimer()
		if err != nil || !changed || doc.Editor.String() != want {
			b.Fatalf("multi-selection edit: changed=%v, error=%v", changed, err)
		}
		b.StartTimer()
	}
}

func BenchmarkMultiSelectionTextInputRefresh(b *testing.B) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("猫 " + strings.Repeat("z", 512<<10) + " 犬")
	end := doc.Editor.RuneCount()
	ranges := []text.Range{{Start: 0, End: 1}, {Start: end - 1, End: end}}
	doc.Editor.SetSelection(ranges[1])
	state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
	var input *app.TextInput
	b.ReportAllocs()
	for b.Loop() {
		input = state.multiSelectionTextInput(0)
	}
	if got := input.CopyText(); got != "猫\n犬\n" {
		b.Fatalf("copy text = %q", got)
	}
}

func TestMultiSelectionClipboardRejectsStaleInput(t *testing.T) {
	for _, change := range []struct {
		name string
		edit func(*model, *document.Document)
	}{
		{"document edit", func(_ *model, doc *document.Document) { doc.Editor.Insert("new") }},
		{"tab switch", func(state *model, _ *document.Document) { state.space.NewDocument() }},
	} {
		t.Run(change.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("猫 猫")
			doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
			state.multiSelectAll(document.SearchOptions{})
			input := state.multiSelectionTextInput(0)
			if got := input.CopyText(); got != "猫\n猫\n" {
				t.Fatalf("initial clipboard = %q", got)
			}
			change.edit(state, doc)
			if got := input.CopyText(); got != "" {
				t.Fatalf("stale input copied %q", got)
			}
		})
	}
}

func TestMultiSelectionNativeTypingAndUndoAreAtomic(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "X X X" || doc.Editor.CanUndo() == false || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-caret typing = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != "cat cat cat" || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges, []text.Range{
		{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11},
	}) {
		t.Fatalf("multi-caret undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Redo")
	if doc.Editor.String() != "X X X" || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-caret redo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "  " {
		t.Fatalf("multi-caret backspace = %q", doc.Editor.String())
	}
}

func TestMultiSelectionGraphemeBackspaceAndMixedLineBookmarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("e\u0301 z e\u0301")
	doc.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: doc, value: doc.Editor.String(), active: true,
		ranges: []text.Range{{Start: 2, End: 2}, {Start: 7, End: 7}},
	}
	if changed, err := state.editMultiSelection(0, "", multiDeleteBackward); err != nil || !changed || doc.Editor.String() != " z " {
		t.Fatalf("grapheme multi-backspace = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "e\u0301 z e\u0301" || doc.Editor.CanUndo() {
		t.Fatalf("grapheme deletion undo = %q", doc.Editor.String())
	}
	other := state.space.NewDocument()
	other.Editor.Insert("a\nb\nc\n")
	other.GoToLine(2)
	state.command("Toggle Bookmark")
	other.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: other, value: other.Editor.String(), active: true,
		ranges: []text.Range{{Start: 0, End: 1}, {Start: 4, End: 5}},
	}
	other.Editor.SetSelection(text.Range{Start: 4, End: 5})
	if changed, err := state.editMultiSelection(0, "X\nY", multiInsert); err != nil || !changed || other.Editor.String() != "X\nY\nb\nX\nY\n" ||
		!reflect.DeepEqual(state.bookmarkLines(other), []int{2}) {
		t.Fatalf("multiline multi-edit = %q, marks=%v, err=%v", other.Editor.String(), state.bookmarkLines(other), err)
	}
	state.command("Undo")
	if other.Editor.String() != "a\nb\nc\n" || !reflect.DeepEqual(state.bookmarkLines(other), []int{1}) {
		t.Fatalf("multiline multi-undo = %q, marks=%v", other.Editor.String(), state.bookmarkLines(other))
	}
}

func TestMultiSelectionIMEFallsBackToSingleCaretWithoutDuplicatingPreedit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	clickID(t, driver, editorID)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("猫"); err != nil {
		t.Fatal(err)
	}
	if state.multiSelections[0].active || doc.Editor.String() == "猫 猫" {
		t.Fatalf("IME preedit duplicated across ranges: %q, state=%+v", doc.Editor.String(), state.multiSelections[0])
	}
	if err := driver.Compose(""); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSelectionCopyCutAndRedactUseEveryRange(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("red red red")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	state.command("Copy")
	if got := driver.Clipboard(); got != "red\nred\nred\n" || !app.ClipboardRectangular() {
		t.Fatalf("multi-selection clipboard = %q", got)
	}
	state.command("Redact Selection")
	if doc.Editor.String() != "███ ███ ███" {
		t.Fatalf("multi-selection redaction = %q", doc.Editor.String())
	}
	state.command("Undo")
	if doc.Editor.String() != "red red red" || len(state.multiSelections[0].ranges) != 3 {
		t.Fatalf("multi-selection redaction undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Cut")
	if doc.Editor.String() != "  " || driver.Clipboard() != "red\nred\nred\n" || !app.ClipboardRectangular() {
		t.Fatalf("multi-selection cut = %q, clipboard=%q", doc.Editor.String(), driver.Clipboard())
	}
	state.command("Undo")
	if doc.Editor.String() != "red red red" {
		t.Fatalf("multi-selection cut undo = %q", doc.Editor.String())
	}
}

func TestMultiSelectionReadOnlyAndSplitCloneDoNotDuplicateEdits(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("one one")
	state.command("Clone to Other View")
	state.activatePane(0)
	clickID(t, driver, editorID)
	left.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	state.readOnly[left] = true
	state.command("Cut")
	if left.Editor.String() != "one one" || len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("read-only cut changed clone or selections")
	}
	delete(state.readOnly, left)
	driver.SetClipboard("Z")
	state.activatePane(1)
	left.Editor.MoveTo(0, false)
	state.command("Paste")
	if left.Editor.String() == "one one" {
		t.Fatal("active clone did not edit")
	}
	state.syncMultiSelection(0, left)
	if state.multiSelections[0].active || len(state.multiSelections[0].ranges) != 0 {
		t.Fatalf("other pane retained stale ranges: %+v", state.multiSelections[0])
	}
}

func TestMultiSelectionNativeCopyCutUsesDisjointClipboard(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const initial = "红 gap 🙂 gap e\u0301"
	const clipboard = "e\u0301\n\n红\n🙂\n"
	doc.Editor.Insert(initial)
	clickID(t, driver, editorID)
	doc.Editor.ClearHistory()
	ranges := []text.Range{{Start: 12, End: 14}, {Start: 5, End: 5}, {Start: 0, End: 1}, {Start: 6, End: 7}}
	doc.Editor.SetSelection(ranges[len(ranges)-1])
	state.multiSelections[0] = multiSelectionState{doc: doc, value: initial, active: true, ranges: ranges}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'c'); err != nil || !handled || driver.Clipboard() != clipboard || !app.ClipboardRectangular() {
		t.Fatalf("native multi-copy handled=%v err=%v clipboard=%q", handled, err, driver.Clipboard())
	}
	if handled, err := driver.ShortcutChord(primary(), 'x'); err != nil || !handled || doc.Editor.String() != " gap  gap " || driver.Clipboard() != clipboard || !app.ClipboardRectangular() {
		t.Fatalf("native multi-cut handled=%v err=%v text=%q clipboard=%q", handled, err, doc.Editor.String(), driver.Clipboard())
	}
	state.command("Undo")
	if doc.Editor.String() != initial || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges, ranges) {
		t.Fatalf("native multi-cut undo: text=%q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	receiver := state.space.NewDocument()
	receiver.Editor.Insert("123\n456\n789\n000")
	clickID(t, driver, editorID)
	receiver.Editor.MoveTo(1, false)
	state.invalidator.Invalidate()
	if handled, err := driver.ShortcutChord(primary(), 'v'); err != nil || !handled || receiver.Editor.String() != "1e\u030123\n456\n7红89\n0🙂00" {
		t.Fatalf("column paste handled=%v err=%v text=%q", handled, err, receiver.Editor.String())
	}
}

func TestMultiSelectionClipboardPreservesPhysicalLineEndings(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"CRLF", "a\r\nb\r\nc\r\nd", "c\r\nd\r\n\r\na\r\n\r\n"},
		{"LF", "a\nb\nc\nd", "c\nd\n\na\n\n"},
		{"CR", "a\rb\rc\rd", "c\rd\r\ra\r\r"},
		{"mixed", "a\r\nb\nc\rd", "c\rd\r\n\r\na\r\n\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, driver := startDriver(t)
			path := filepath.Join(t.TempDir(), "clipboard.txt")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := state.space.Open(path); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			ranges := []text.Range{{Start: 4, End: 7}, {Start: 2, End: 2}, {Start: 0, End: 2}}
			doc.Editor.SetSelection(ranges[2])
			state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
			state.command("Copy")
			if driver.Clipboard() != tc.want || !app.ClipboardRectangular() || doc.Dirty() || doc.Editor.CanUndo() {
				t.Fatalf("copy = %q, rectangular=%v, dirty=%v", driver.Clipboard(), app.ClipboardRectangular(), doc.Dirty())
			}
			state.command("Cut")
			if driver.Clipboard() != tc.want || !app.ClipboardRectangular() || doc.Editor.String() != "b\n" {
				t.Fatalf("cut = %q, remaining text = %q", driver.Clipboard(), doc.Editor.String())
			}
			state.command("Undo")
			if doc.Dirty() || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges, ranges) {
				t.Fatalf("undo: dirty=%v, text=%q, ranges=%+v", doc.Dirty(), doc.Editor.String(), state.multiSelections[0].ranges)
			}
		})
	}
}

func TestMultiSelectionClipboardSingleAndEmptyRanges(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat dog")
	for _, tc := range []struct {
		ranges []text.Range
		want   string
	}{
		{[]text.Range{{Start: 0, End: 3}}, "cat"},
		{[]text.Range{{Start: 0, End: 0}, {Start: 4, End: 4}}, "sentinel"},
	} {
		driver.SetClipboard("sentinel")
		doc.Editor.SetSelection(tc.ranges[len(tc.ranges)-1])
		state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: tc.ranges}
		state.command("Copy")
		if driver.Clipboard() != tc.want || app.ClipboardRectangular() {
			t.Fatalf("copy %v = %q, rectangular=%v", tc.ranges, driver.Clipboard(), app.ClipboardRectangular())
		}
	}
}

func TestMultiSelectionPointerAndSelectAllDismissRanges(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat cat")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	clickID(t, driver, editorID)
	if state.multiSelections[0].active {
		t.Fatal("pointer click retained disjoint selections")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectAll(document.SearchOptions{})
	state.command("Select All")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if state.multiSelections[0].active || doc.Editor.Selection() != (text.Range{Start: 0, End: 7}) {
		t.Fatalf("ordinary Select All was overridden: %+v, selection=%+v", state.multiSelections[0], doc.Editor.Selection())
	}
}

func TestMultiSelectionKeepsPaneLocalMainSelectionAcrossCloneFocus(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat cat")
	state.command("Clone to Other View")
	state.activatePane(0)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.multiSelectNext(document.SearchOptions{})
	if len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("left view did not add the second selection")
	}
	state.activatePane(1)
	doc.Editor.SetSelection(text.Range{Start: 8, End: 11})
	state.multiSelectNext(document.SearchOptions{})
	state.activatePane(0)
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) || len(state.multiSelections[0].ranges) != 2 || len(state.multiSelections[1].ranges) != 2 {
		t.Fatalf("clone focus lost selection: left=%+v right=%+v main=%+v", state.multiSelections[0], state.multiSelections[1], doc.Editor.Selection())
	}
}

func TestMultiSelectionCollapsedRangesPaintAdditionalCarets(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a a")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.multiSelectAll(document.SearchOptions{})
	if changed, err := state.editMultiSelection(0, "X", multiInsert); err != nil || !changed {
		t.Fatalf("multi-caret insertion = %v, %v", changed, err)
	}
	snapshot := components.CodeEditorSnapshot{}
	state.paintMultiSelection(&snapshot, doc, 0)
	if !snapshot.ShowCaret || len(snapshot.SelectionRanges) != 0 || len(snapshot.ExtraCarets) != 1 ||
		snapshot.ExtraCarets[0].Offset != doc.Editor.ByteOffset(1) || snapshot.Caret != doc.Editor.ByteOffset(3) {
		t.Fatalf("collapsed multi-caret paint = %+v", snapshot)
	}
}

func TestMultiSelectionMenuPasteAppliesSameMultilineTextAtEachRange(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("α α")
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	doc.Editor.ClearHistory()
	state.multiSelectAll(document.SearchOptions{})
	driver.SetClipboard("β\nγ\nδ")
	state.command("Paste")
	if doc.Editor.String() != "β\nγ\nδ β\nγ\nδ" || len(state.multiSelections[0].ranges) != 2 {
		t.Fatalf("multi-paste = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != "α α" || doc.Editor.CanUndo() {
		t.Fatalf("multi-paste undo = %q", doc.Editor.String())
	}
}

func TestMultiSelectionPasteAndUndoPreservePhysicalEOL(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		for _, native := range []bool{false, true} {
			name := "LF"
			if ending == "\r\n" {
				name = "CRLF"
			}
			if native {
				name += "/keyboard"
			} else {
				name += "/menu"
			}
			t.Run(name, func(t *testing.T) {
				state, driver := startDriver(t)
				path := filepath.Join(t.TempDir(), "paste.txt")
				original := "cat" + ending + "cat" + ending + "cat"
				if err := os.WriteFile(path, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
				if err := state.space.Open(path); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, editorID)
				doc := state.space.Current()
				doc.Editor.MoveTo(5, false)
				state.multiSelectAll(document.SearchOptions{})
				state.invalidator.Invalidate()
				if err := driver.Pump(); err != nil {
					t.Fatal(err)
				}
				paste := func(value string) {
					t.Helper()
					driver.SetClipboard(value)
					if native {
						if _, err := driver.ShortcutChord(primary(), 'v'); err != nil {
							t.Fatal(err)
						}
					} else {
						state.command("Paste")
					}
				}
				check := func(want string) {
					t.Helper()
					copyPath := filepath.Join(t.TempDir(), "copy.txt")
					if err := doc.SaveCopy(copyPath); err != nil {
						t.Fatal(err)
					}
					got, err := os.ReadFile(copyPath)
					if err != nil || string(got) != want {
						t.Fatalf("physical text = %q, want %q (err=%v)", got, want, err)
					}
				}
				for _, tc := range []struct{ clipboard, want string }{
					{"α\r\n\r\nγ", "α" + ending + ending + "γ"},
					{"α\r\nγ", strings.Join([]string{"α", "γ", "α", "γ", "α", "γ"}, ending)},
				} {
					paste(tc.clipboard)
					check(tc.want)
					state.command("Undo")
					check(original)
					if doc.Editor.CanUndo() || len(state.multiSelections[0].ranges) != 3 {
						t.Fatal("one undo did not restore all three selections")
					}
					state.command("Redo")
					check(tc.want)
					if doc.Editor.CanRedo() {
						t.Fatal("one redo did not restore the entire paste")
					}
					state.command("Undo")
					state.invalidator.Invalidate()
					if err := driver.Pump(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestMultiSelectionNavigationAndMergedDelete(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	doc := state.space.Current()
	doc.Editor.Insert("ab ab\nab")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.multiSelectAll(document.SearchOptions{})
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key  gfx.Key
		want []text.Range
	}{
		{gfx.KeyLeft, []text.Range{{Start: 0, End: 0}, {Start: 3, End: 3}, {Start: 6, End: 6}}},
		{gfx.KeyRight, []text.Range{{Start: 1, End: 1}, {Start: 4, End: 4}, {Start: 7, End: 7}}},
		{gfx.KeyHome, []text.Range{{Start: 0, End: 0}, {Start: 6, End: 6}}},
		{gfx.KeyEnd, []text.Range{{Start: 5, End: 5}, {Start: 8, End: 8}}},
	} {
		if err := driver.Key(tc.key, 0); err != nil {
			t.Fatal(err)
		}
		if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, tc.want) || doc.Editor.CanUndo() {
			t.Fatalf("%v moved to %v, want %v without editing", tc.key, got, tc.want)
		}
	}
	doc.Editor.MoveTo(2, false)
	state.multiSelections[0].ranges = []text.Range{{Start: 1, End: 1}, {Start: 2, End: 2}}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != " ab\nab" || doc.Editor.Caret() != 0 || len(state.multiSelections[0].ranges) > 1 {
		t.Fatalf("adjacent deletions did not merge carets: %q, %v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != "ab ab\nab" || doc.Editor.CanUndo() {
		t.Fatal("adjacent deletions were not one undo unit")
	}
}

func TestMultiSelectionAltClickTogglesSecondaryCaret(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	doc := state.space.Current()
	doc.Editor.Insert("ab\ncd\nef")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	caret, ok := state.codeEditor().CaretBounds()
	if !ok {
		t.Fatal("caret bounds missing")
	}
	if err := driver.KeyDown(gfx.KeyAltLeft, ""); err != nil {
		t.Fatal(err)
	}
	clickRow := func(row float32) {
		t.Helper()
		y := caret.Y + caret.Height*(row+0.5)
		if err := driver.Press(caret.X, y, gfx.PointerButtonPrimary); err != nil {
			t.Fatal(err)
		}
		if err := driver.Release(caret.X, y, gfx.PointerButtonPrimary); err != nil {
			t.Fatal(err)
		}
	}
	clickRow(1)
	clickRow(2)
	if got := state.multiSelections[0].ranges; len(got) != 3 || state.rectangles[0].active {
		t.Fatalf("Alt+Click did not add two independent carets: %v", got)
	}
	clickRow(1)
	if got := state.multiSelections[0].ranges; !reflect.DeepEqual(got, []text.Range{{Start: 7, End: 7}, {Start: 1, End: 1}}) || doc.Editor.CanUndo() {
		t.Fatalf("Alt+Click did not remove only the secondary caret: %v", got)
	}
	if err := driver.KeyUp(gfx.KeyAltLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "aXb\ncd\neXf" {
		t.Fatalf("typing after Alt+Click = %q", doc.Editor.String())
	}
}

func TestMultiSelectionOverlappingGraphemeDeleteMergesTargets(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	doc := state.space.Current()
	const original = "a\u0301 b"
	doc.Editor.Insert(original)
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	state.multiSelections[0] = multiSelectionState{doc: doc, value: original, active: true,
		ranges: []text.Range{{Start: 0, End: 0}, {Start: 1, End: 1}}}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyDelete, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != " b" || !reflect.DeepEqual(state.multiSelections[0].ranges, []text.Range{{Start: 0, End: 0}}) {
		t.Fatalf("overlapping grapheme delete = %q, ranges = %v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
	state.command("Undo")
	if doc.Editor.String() != original || doc.Editor.CanUndo() || len(state.multiSelections[0].ranges) != 2 {
		t.Fatal("overlapping delete was not one undo unit")
	}
	state.command("Redo")
	if doc.Editor.String() != " b" || doc.Editor.CanRedo() || len(state.multiSelections[0].ranges) != 1 {
		t.Fatal("overlapping delete was not one redo unit")
	}
}

func TestMultiSelectionForwardDeleteIsOneUndoUnit(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("aX bX")
	doc.Editor.MoveTo(4, false)
	doc.Editor.ClearHistory()
	state.multiSelections[0] = multiSelectionState{
		doc: doc, value: doc.Editor.String(), active: true,
		ranges: []text.Range{{Start: 1, End: 1}, {Start: 4, End: 4}},
	}
	if changed, err := state.editMultiSelection(0, "", multiDeleteForward); err != nil || !changed || doc.Editor.String() != "a b" {
		t.Fatalf("forward delete at two carets = %q, %v", doc.Editor.String(), err)
	}
	state.command("Undo")
	if doc.Editor.String() != "aX bX" || doc.Editor.CanUndo() || !reflect.DeepEqual(state.multiSelections[0].ranges,
		[]text.Range{{Start: 1, End: 1}, {Start: 4, End: 4}}) {
		t.Fatalf("forward delete undo = %q, ranges=%+v", doc.Editor.String(), state.multiSelections[0].ranges)
	}
}
