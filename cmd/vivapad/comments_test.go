package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/macro"
)

func TestSingleLineCommentMenuAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	clickID(t, driver, editorID)
	if err := driver.Type("  猫\n  // 犬\nlast"); err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: len([]rune("  猫\n  // 犬\n"))})
	clickID(t, driver, menuScope.At(1))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(19) && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("Edit menu overlaps status bar: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(19))
	for index := range commentCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == commentScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400 {
					t.Fatalf("comment submenu row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("comment submenu row %d missing", index)
		}
	}
	clickID(t, driver, commentScope.At(1))
	if got := doc.Editor.String(); got != "  // 猫\n  // // 犬\nlast" {
		t.Fatalf("comment menu = %q", got)
	}
	state.command("Undo")
	if doc.Editor.String() != "  猫\n  // 犬\nlast" {
		t.Fatal("comment menu failed to undo")
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: len([]rune("  猫\n  // 犬\n"))})
	state.command("Single Line Uncomment")
	if got := doc.Editor.String(); got != "  猫\n  犬\nlast" {
		t.Fatalf("uncomment menu = %q", got)
	}
}

func TestSingleLineCommentsRespectLanguageReadOnlyAndSplitView(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("value")
	doc.Editor.MoveTo(0, false)
	state.command("Single Line Comment")
	if doc.Editor.String() != "value" || !strings.Contains(state.notice, "comment") {
		t.Fatalf("plain text comment = %q notice=%q", doc.Editor.String(), state.notice)
	}
	state.languages[doc] = ".sql"
	state.command("Single Line Comment")
	if doc.Editor.String() != "-- value" {
		t.Fatalf("SQL comment = %q", doc.Editor.String())
	}
	state.command("Read-Only on Current Document")
	state.command("Single Line Uncomment")
	if doc.Editor.String() != "-- value" || state.notice != "Read-only document" {
		t.Fatalf("read-only uncomment = %q notice=%q", doc.Editor.String(), state.notice)
	}
	state.command("Read-Only on Current Document")
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightEditorID)
	state.command("Single Line Uncomment")
	if doc.Editor.String() != "value" || state.activePane != 1 {
		t.Fatalf("split-view uncomment = %q pane=%d", doc.Editor.String(), state.activePane)
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSingleLineCommentKeepsBookmarkOnTheSameLine(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".py"
	doc.Editor.Insert("one\n  猫\nthree")
	doc.Editor.MoveTo(6, false)
	state.toggleBookmark()
	state.command("Single Line Comment")
	if doc.Editor.String() != "one\n  # 猫\nthree" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("comment bookmark = %q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Undo")
	if doc.Editor.String() != "one\n  猫\nthree" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("undo comment bookmark = %q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestBlockCommentMenuFitsCompactWindowAndUncomments(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	clickID(t, driver, editorID)
	if err := driver.Type("  αβ\nnext"); err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 4})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	for index := range commentCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == commentScope.At(index) {
				found = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("comment row %d overlaps chrome: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("comment row %d missing", index)
		}
	}
	clickID(t, driver, commentScope.At(3))
	if got := doc.Editor.String(); got != "  /* αβ */\nnext" {
		t.Fatalf("Block Comment menu = %q", got)
	}
	state.command("Single Line Uncomment")
	if got := doc.Editor.String(); got != "  αβ\nnext" {
		t.Fatalf("Single Line Uncomment stream fallback = %q", got)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "  /* αβ */\nnext" {
		t.Fatal("stream fallback failed one-step undo")
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(4))
	if got := doc.Editor.String(); got != "  αβ\nnext" {
		t.Fatalf("Block Uncomment = %q", got)
	}
}

func TestBlockCommentLanguageFallbackReadOnlyAndSplitView(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("  猫\nnext")
	doc.Editor.MoveTo(2, false)
	state.languages[doc] = ".py"
	state.command("Block Comment")
	if doc.Editor.String() != "  # 猫\nnext" {
		t.Fatalf("line-only language fallback = %q", doc.Editor.String())
	}
	state.command("Block Uncomment")
	if doc.Editor.String() != "  猫\nnext" {
		t.Fatalf("line-only uncomment fallback = %q", doc.Editor.String())
	}
	state.languages[doc] = ".html"
	state.command("Read-Only on Current Document")
	state.command("Block Comment")
	if doc.Editor.String() != "  猫\nnext" || state.notice != "Read-only document" {
		t.Fatalf("read-only stream comment = %q notice=%q", doc.Editor.String(), state.notice)
	}
	state.command("Read-Only on Current Document")
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	clickID(t, driver, rightEditorID)
	doc.Editor.MoveTo(2, false)
	state.command("Block Comment")
	if doc.Editor.String() != "  <!-- 猫 -->\nnext" || state.activePane != 1 {
		t.Fatalf("split HTML comment = %q pane=%d", doc.Editor.String(), state.activePane)
	}
	state.command("Block Uncomment")
	if doc.Editor.String() != "  猫\nnext" {
		t.Fatalf("split HTML uncomment = %q", doc.Editor.String())
	}
}

func TestBlockCommentPreservesBookmarkAcrossSelectedLines(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".go"
	doc.Editor.Insert("one\n猫\nthree")
	doc.Editor.MoveTo(5, false)
	state.toggleBookmark()
	doc.Editor.SetSelection(text.Range{Start: 4, End: 7})
	state.command("Block Comment")
	if doc.Editor.String() != "one\n/* 猫\nt */hree" {
		t.Fatalf("unexpected selected stream content: %q", doc.Editor.String())
	}
	if marks := state.bookmarkLines(doc); len(marks) != 1 || marks[0] != 1 {
		t.Fatalf("block comment moved bookmark: %v", marks)
	}
	state.command("Undo")
	if doc.Editor.String() != "one\n猫\nthree" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("undo stream comment bookmark = %q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
}

func TestSingleLineUncommentFallsBackForStreamOnlyLanguage(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".css"
	doc.Editor.Insert("a /* color */ b")
	doc.Editor.MoveTo(7, false)
	state.command("Single Line Uncomment")
	if doc.Editor.String() != "a color b" {
		t.Fatalf("stream-only line-uncomment fallback = %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestToggleCommentMenuAndShortcutAgreeOnMixedLines(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	clickID(t, driver, editorID)
	if err := driver.Type("// one\ntwo"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, doc.Editor.RuneCount()); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(0))
	if got := doc.Editor.String(); got != "one\n// two" {
		t.Fatalf("mixed menu toggle = %q", got)
	}
	state.command("Undo")
	clickID(t, driver, editorID)
	if err := driver.SelectText(0, doc.Editor.RuneCount()); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(primary(), '/'); err != nil || !handled {
		t.Fatalf("comment shortcut handled=%v err=%v", handled, err)
	}
	if got := doc.Editor.String(); got != "one\n// two" {
		t.Fatalf("mixed shortcut toggle = %q", got)
	}
	state.command("Read-Only on Current Document")
	state.command("Toggle Single Line Comment")
	if doc.Editor.String() != "one\n// two" || state.notice != "Read-only document" {
		t.Fatalf("read-only toggle = %q notice=%q", doc.Editor.String(), state.notice)
	}
	if handled, err := driver.ShortcutChord(primary(), '/'); err != nil || !handled || doc.Editor.String() != "one\n// two" {
		t.Fatalf("read-only toggle shortcut handled=%v text=%q err=%v", handled, doc.Editor.String(), err)
	}
}

func TestStreamOnlyToggleAndMacroReplayUseOriginalLineSemantics(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".css"
	doc.Editor.Insert("/* one */\ntwo")
	doc.Editor.ClearHistory()
	clickID(t, driver, editorID)
	if err := driver.SelectText(0, doc.Editor.RuneCount()); err != nil {
		t.Fatal(err)
	}
	state.command("Record Macro")
	if handled, err := driver.ShortcutChord(primary(), '/'); err != nil || !handled {
		t.Fatalf("CSS comment shortcut handled=%v err=%v", handled, err)
	}
	state.command("Stop Recording")
	if doc.Editor.String() != "one\n/* two */" || len(state.macroLast) != 1 ||
		state.macroLast[0].Kind != macro.CommentStep || state.macroLast[0].BlockStart != "/*" {
		t.Fatalf("recorded stream toggle = %q steps=%+v", doc.Editor.String(), state.macroLast)
	}
	state.command("Undo")
	state.languages[doc] = ".html"
	state.command("Playback Macro")
	if doc.Editor.String() != "one\n/* two */" {
		t.Fatalf("CSS macro playback used different toggle semantics: %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestToggleCommentKeepsBookmarksOnMixedLines(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".py"
	doc.Editor.Insert("# one\ntwo")
	doc.Editor.MoveTo(7, false)
	state.toggleBookmark()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Toggle Single Line Comment")
	if doc.Editor.String() != "one\n# two" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("mixed toggle bookmark = %q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.command("Undo")
	if doc.Editor.String() != "# one\ntwo" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("undo mixed toggle bookmark = %q lines=%v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	state.macroLast = []macro.Step{{Kind: macro.CommentStep, Comment: "line-toggle", LinePrefix: "#"}}
	state.command("Playback Macro")
	if doc.Editor.String() != "one\n# two" || len(state.bookmarkLines(doc)) != 1 || state.bookmarkLines(doc)[0] != 1 {
		t.Fatalf("macro mixed toggle bookmark = %q lines=%v notice=%q", doc.Editor.String(), state.bookmarkLines(doc), state.notice)
	}
}

func TestStreamOnlyLineCommentAddsAndRemovesEachSelectedLine(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".css"
	doc.Editor.Insert("alpha\nbeta")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	state.command("Single Line Comment")
	if got := doc.Editor.String(); got != "/* alpha */\n/* beta */" {
		t.Fatalf("stream-only line comment = %q", got)
	}
	state.command("Single Line Uncomment")
	if got := doc.Editor.String(); got != "alpha\nbeta" {
		t.Fatalf("stream-only line uncomment = %q", got)
	}
}

func TestMixedLineToggleInRightSplitPaneLeavesLeftDocumentUntouched(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("// one\ntwo")
	state.languages[right] = ".go"
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Toggle Single Line Comment")
	if state.activePane != 1 || right.Editor.String() != "one\n// two" || left.Editor.String() != "left" {
		t.Fatalf("split toggle pane=%d left=%q right=%q", state.activePane, left.Editor.String(), right.Editor.String())
	}
}
