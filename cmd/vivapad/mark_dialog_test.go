package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func setMarkQuery(state *model, value string) {
	state.query.SetSelection(text.Range{Start: 0, End: state.query.RuneCount()})
	state.query.Insert(value)
}

func TestMarkDialogPurgeBookmarksAndInvalidQueryAreAtomic(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat cat\ndog cat")
	doc.Editor.ClearHistory()
	before, dirty := doc.Editor.String(), doc.Dirty()
	state.command("Mark...")
	if !state.markDialogOpen {
		t.Fatal("Search > Mark did not open the panel")
	}
	state.markBookmarkLine = true
	setMarkQuery(state, "cat")
	state.markAll()
	if len(state.markedSpans) != 3 || !slices.Equal(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("initial marks=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	setMarkQuery(state, "dog")
	state.markAll()
	if len(state.markedSpans) != 4 || !slices.Equal(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("cumulative marks=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	state.markPurge = true
	state.markMode = "regex"
	setMarkQuery(state, "(")
	state.markAll()
	if len(state.markedSpans) != 4 || !slices.Equal(state.bookmarkLines(doc), []int{0, 1}) || !strings.Contains(state.notice, "missing") {
		t.Fatalf("invalid regex partially purged marks=%+v bookmarks=%v notice=%q", state.markedSpans, state.bookmarkLines(doc), state.notice)
	}
	state.markMode = "normal"
	setMarkQuery(state, "dog")
	state.markAll()
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 8, End: 11}}) || !slices.Equal(state.bookmarkLines(doc), []int{1}) ||
		doc.Editor.String() != before || doc.Dirty() != dirty || doc.Editor.CanUndo() {
		t.Fatalf("purged marks=%+v bookmarks=%v, text=%q", state.markedSpans, state.bookmarkLines(doc), doc.Editor.String())
	}
}

func TestMarkInSelectionAndWrapDoNotEscapeByteRange(t *testing.T) {
	state := newModel()
	state.command("Mark...")
	doc := state.space.Current()
	doc.Editor.Insert("猫 cat cat\ncat")
	state.markInSelection = true
	state.markBookmarkLine = true
	setMarkQuery(state, "cat")
	start := strings.Index(doc.Editor.String(), "cat")
	doc.Editor.SetSelection(text.Range{Start: 2, End: 5})
	state.markAll()
	if !slices.Equal(state.markedSpans, []text.Span{{Start: start, End: start + 3}}) || !slices.Equal(state.bookmarkLines(doc), []int{0}) {
		t.Fatalf("selected mark=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	doc.Editor.MoveTo(0, false)
	state.markAll()
	if len(state.markedSpans) != 1 || !strings.Contains(state.notice, "selection") {
		t.Fatalf("empty selection changed marks=%+v notice=%q", state.markedSpans, state.notice)
	}
	state.markInSelection = false
	doc.Editor.MoveTo(6, false)
	state.markWrap = false
	state.markPurge = true
	state.markAll()
	if len(state.markedSpans) != 2 || state.markedSpans[0].Start <= start {
		t.Fatalf("unwrapped mark searched before caret: %+v", state.markedSpans)
	}
	state.markWrap = true
	state.markAll()
	if len(state.markedSpans) != 3 || state.markedSpans[0].Start != start {
		t.Fatalf("wrapped mark did not include earlier match: %+v", state.markedSpans)
	}
}

func TestMarkExtendedEscapesAndRegexDotAll(t *testing.T) {
	if got, err := parseMarkExtended(`a\n\r\t\0\x41\\`); err != nil || got != "a\n\r\t\x00A\\" {
		t.Fatalf("extended escapes = %q, %v", got, err)
	}
	for _, invalid := range []string{`\`, `\x`, `\x8`, `\xGG`, `\x80`, `\z`} {
		if _, err := parseMarkExtended(invalid); err == nil {
			t.Errorf("invalid escape %q accepted", invalid)
		}
	}
	state := newModel()
	state.command("Mark...")
	state.space.Current().Editor.Insert("a\tb\na\nb")
	state.markMode = "extended"
	setMarkQuery(state, `a\tb`)
	state.markAll()
	if len(state.markedSpans) != 1 || state.markedSpans[0] != (text.Span{Start: 0, End: 3}) {
		t.Fatalf("extended mark = %+v, notice=%q", state.markedSpans, state.notice)
	}
	state.markMode, state.markPurge, state.markDotAll = "regex", true, true
	setMarkQuery(state, "a.b")
	state.markAll()
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 3}, {Start: 4, End: 7}}) {
		t.Fatalf("dot-all mark = %+v, notice=%q", state.markedSpans, state.notice)
	}
}

func TestMarkClearInSelectionAndCopyMarkedText(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("cat\ncat\ncat")
	state.command("Mark...")
	state.markBookmarkLine = true
	setMarkQuery(state, "cat")
	state.markAll()
	if len(state.markedSpans) != 3 || !slices.Equal(state.bookmarkLines(doc), []int{0, 1, 2}) {
		t.Fatalf("setup marks=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	driver.SetClipboard("previous")
	state.copyMarkedText()
	if driver.Clipboard() != "cat\r\ncat\r\ncat\r\n" {
		t.Fatalf("copied marks = %q", driver.Clipboard())
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	state.markInSelection = true
	state.clearMarkDialog()
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 3}, {Start: 8, End: 11}}) || !slices.Equal(state.bookmarkLines(doc), []int{0, 2}) {
		t.Fatalf("clear selected marks=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	state.markInSelection = false
	state.clearMarkDialog()
	if len(state.markedSpans) != 0 || len(state.bookmarkLines(doc)) != 0 {
		t.Fatalf("Clear All left marks/bookmarks=%+v/%v", state.markedSpans, state.bookmarkLines(doc))
	}
	driver.SetClipboard("untouched")
	state.copyMarkedText()
	if driver.Clipboard() != "untouched" {
		t.Fatal("empty marks replaced the clipboard")
	}
	state.markMode = "regex"
	setMarkQuery(state, "cat\\ncat")
	state.markAll()
	if len(state.markedSpans) != 1 {
		t.Fatalf("multiline setup marks = %+v", state.markedSpans)
	}
	state.markMode = "normal"
	setMarkQuery(state, "cat")
	state.markAll()
	state.copyMarkedText()
	if got := driver.Clipboard(); !strings.Contains(got, "\r\n----\r\n") || !strings.HasSuffix(got, "\r\n") {
		t.Fatalf("multiline marked copy = %q", got)
	}
}

func TestMarkClearInvalidSelectionLeavesSharedCloneAndBookmarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("cat\ncat")
	doc.Editor.ClearHistory()
	state.command("Clone to Other View")
	state.command("Mark...")
	state.markBookmarkLine = true
	setMarkQuery(state, "cat")
	state.markAll()
	if !state.splitEnabled || state.paneDocs[0] != doc || state.paneDocs[1] != doc || len(state.markedSpans) != 2 {
		t.Fatalf("shared clone setup = %v, %+v", state.paneDocs, state.markedSpans)
	}
	state.markInSelection = true
	doc.Editor.MoveTo(0, false)
	state.clearMarkDialog()
	if len(state.markedSpans) != 2 || !slices.Equal(state.bookmarkLines(doc), []int{0, 1}) {
		t.Fatalf("empty selection partially cleared clone marks=%+v bookmarks=%v", state.markedSpans, state.bookmarkLines(doc))
	}
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	state.clearMarkDialog()
	if !slices.Equal(state.markedSpans, []text.Span{{Start: 0, End: 3}}) || !slices.Equal(state.bookmarkLines(doc), []int{0}) || doc.Editor.CanUndo() {
		t.Fatalf("selected clone clear = %+v / %v", state.markedSpans, state.bookmarkLines(doc))
	}
}
