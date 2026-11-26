package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestFoldCommandsHideNestedRowsWithoutChangingCleanDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fold.go")
	value := "func main() {\n  if true {\n    println(1)\n  }\n}\ntail"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.toggleBookmark()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.command("Fold All")
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 0, LastLine: 4}}) {
		t.Fatalf("fold-all ranges = %+v", got)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, "println(1)") {
			t.Fatal("folded line was still painted")
		}
	}
	if doc.Editor.String() != value || doc.Dirty() || doc.Editor.CanUndo() || len(state.bookmarkLines(doc)) != 1 {
		t.Fatal("folding changed clean text, undo, or bookmarks")
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == leftFoldScope.At(0) {
			clickID(t, driver, hit.ID)
			break
		}
	}
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 1, LastLine: 3}}) {
		t.Fatalf("expanding outer fold lost nested state: %+v", got)
	}
	clickID(t, driver, leftFoldScope.At(1))
	if len(state.foldRanges(doc)) != 0 {
		t.Fatalf("inner gutter toggle did not expand: %+v", state.foldRanges(doc))
	}
	state.command("Fold Level 2")
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 1, LastLine: 3}}) {
		t.Fatalf("level-2 fold ranges = %+v", got)
	}
	state.command("Unfold All")
	if len(state.foldRanges(doc)) != 0 || doc.Dirty() {
		t.Fatal("unfold all changed document")
	}
}

func TestInlineFoldBadgeExpandsOuterFoldWithoutChangingDocument(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "func main() {\n  if true {\n    println(1)\n  }\n}\ntail"
	doc.Editor.Insert(source)
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	state.command("Go")
	state.toggleBookmark()
	state.command("Fold All")
	const anchor, caret = 1, 4
	doc.Editor.SetSelection(text.Range{Start: anchor, End: caret})
	before := doc.Editor.Selection()
	dirty := doc.Dirty()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	var badge gfx.ID
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Expand fold" && hit.ID != leftFoldScope.At(0) {
			badge = hit.ID
			break
		}
	}
	if badge.IsZero() {
		t.Fatal("missing the inline fold control")
	}
	clickID(t, driver, badge)
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 1, LastLine: 3}}) {
		t.Fatalf("outer badge did not preserve the folded child: %+v", got)
	}
	if doc.Editor.String() != source || doc.Editor.Selection() != before || doc.Dirty() != dirty || doc.Editor.CanUndo() || !reflect.DeepEqual(state.bookmarkLines(doc), []int{0}) {
		t.Fatal("expanding the inline badge changed text, selection, history, or bookmarks")
	}
}

func TestFoldAllHidesPythonBodyAfterCommentedHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greet.py")
	value := "def greet(): # inline comment\r\n    hidden_body()\r\noutside()\r\n"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	before := doc.Editor.String()
	state.command("Fold All")
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 0, LastLine: 1}}) {
		t.Fatalf("Python fold ranges = %+v", got)
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, "hidden_body") {
			t.Fatal("folded Python body was still painted")
		}
	}
	clickID(t, driver, leftFoldScope.At(0))
	if len(state.foldRanges(doc)) != 0 || doc.Editor.String() != before || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatal("toggling the Python fold changed document or undo state")
	}
	if actual, err := os.ReadFile(path); err != nil || string(actual) != value {
		t.Fatalf("file changed: %q, %v", actual, err)
	}
}

func TestFoldCurrentAndSearchIntoHiddenLineRevealIt(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("func main() {\n  if true {\n    work()\n  }\n}\ntail")
	doc.Editor.ClearHistory()
	state.command("Go")
	if !doc.GoToLine(2) {
		t.Fatal("inner header unavailable")
	}
	state.command("Fold Current Level")
	if got := state.foldRanges(doc); !reflect.DeepEqual(got, []components.CodeEditorFoldRange{{Header: 1, LastLine: 3}}) {
		t.Fatalf("current fold = %+v", got)
	}
	if !doc.GoToLine(3) {
		t.Fatal("hidden line unavailable")
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if len(state.foldRanges(doc)) != 0 || doc.Editor.CanUndo() {
		t.Fatalf("hidden caret was not revealed: %+v", state.foldRanges(doc))
	}
}

func TestSelectionIntoHiddenFoldRevealsItsBody(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("func main() {\n  work()\n}\n")
	doc.Editor.ClearHistory()
	state.command("Go")
	state.command("Fold All")
	if len(state.foldRanges(doc)) != 1 {
		t.Fatal("fold was not collapsed")
	}
	hidden := doc.Editor.RuneIndex(strings.Index(doc.Editor.String(), "work"))
	doc.Editor.SetSelection(text.Range{Start: hidden, End: 0})
	if doc.Editor.Caret() != 0 {
		t.Fatalf("test requires caret outside hidden body, got %d", doc.Editor.Caret())
	}
	selection := doc.Editor.Selection()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if len(state.foldRanges(doc)) != 0 || doc.Editor.Selection() != selection || doc.Editor.CanUndo() {
		t.Fatalf("hidden selection not revealed or changed: folds=%+v selection=%+v", state.foldRanges(doc), doc.Editor.Selection())
	}
}

func TestViewFoldMenusReachAllLevelsAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, 16)
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(16) && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("View fold submenu clipped: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(15))
	seen := make([]bool, 8)
	for _, hit := range driver.Snapshot().Hits {
		for index := range seen {
			if hit.ID == foldLevelScope.At(index) {
				seen[index] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("fold level %d clipped: %+v", index+1, hit.Bounds)
				}
			}
		}
	}
	for index, found := range seen {
		if !found {
			t.Fatalf("fold level %d not reachable", index+1)
		}
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.foldMenuOpen || state.space.Current().Dirty() {
		t.Fatal("Escape from fold submenu edited a document")
	}
}

func TestFoldStateFollowsDocumentAcrossSplitViews(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("{\ninside\n}\nend")
	doc.Editor.ClearHistory()
	state.command("JSON")
	state.toggleSplit()
	state.paneDocs[0], state.paneDocs[1] = doc, doc
	state.activePane = 1
	state.command("Fold All")
	if len(state.foldRanges(doc)) != 1 {
		t.Fatal("clone did not share fold state")
	}
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	left, right := false, false
	for _, hit := range driver.Snapshot().Hits {
		left = left || hit.ID == leftFoldScope.At(0)
		right = right || hit.ID == rightFoldScope.At(0)
	}
	if !left || !right {
		t.Fatalf("split clone gutter buttons = left:%v right:%v", left, right)
	}
}

func TestEditingInvalidatesFoldStateAndReadOnlyCanStillFold(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("{\ninside\n}\nend")
	doc.Editor.ClearHistory()
	state.command("JSON")
	state.readOnly[doc] = true
	state.command("Fold All")
	if len(state.foldRanges(doc)) != 1 || doc.Editor.CanUndo() || state.notice == "Read-only document" {
		t.Fatal("read-only blocked presentation-only folding")
	}
	state.readOnly[doc] = false
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("\nchanged")
	if len(state.foldRanges(doc)) != 0 || len(state.folded[doc]) != 0 {
		t.Fatalf("edit left stale collapsed headers: %+v", state.folded[doc])
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
}

func TestFoldAndUnfoldEveryOriginalLevel(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	var source strings.Builder
	for range 8 {
		source.WriteString("{\n")
	}
	for range 8 {
		source.WriteString("}\n")
	}
	value := source.String()
	doc.Editor.Insert(value)
	doc.Editor.ClearHistory()
	state.command("JSON")
	for level := 1; level <= 8; level++ {
		state.command("Fold Level " + strconv.Itoa(level))
		if len(state.folded[doc]) != level {
			t.Fatalf("fold level %d set %d headers", level, len(state.folded[doc]))
		}
	}
	for level := 1; level <= 8; level++ {
		state.command("Unfold Level " + strconv.Itoa(level))
		if len(state.folded[doc]) != 8-level {
			t.Fatalf("unfold level %d kept %d headers", level, len(state.folded[doc]))
		}
	}
	if doc.Editor.String() != value || doc.Editor.CanUndo() {
		t.Fatal("level commands edited text or undo history")
	}
}
