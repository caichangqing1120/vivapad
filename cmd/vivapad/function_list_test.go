package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func TestFunctionListFilterAndNavigationKeepCleanSourceUntouched(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "symbols.go")
	value := "package sample\n//line source.template:700\ntype Server struct{}\nfunc (s *Server) ServeHTTP() {}\nfunc main() {}\n"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(18))
	if !state.functionListVisible {
		t.Fatal("View command did not open Function List")
	}
	wantHints := map[string]string{
		"Server":              "type on line 3",
		"(*Server).ServeHTTP": "function on line 4",
		"main":                "function on line 5",
	}
	for _, hit := range driver.Snapshot().Hits {
		if want, ok := wantHints[hit.Label]; hit.ID.Scope() == functionSymbolScope && ok {
			if hit.Hint != want {
				t.Errorf("%s hint = %q, want %q", hit.Label, hit.Hint, want)
			}
			delete(wantHints, hit.Label)
		}
	}
	if len(wantHints) != 0 {
		t.Fatalf("missing Function List results: %v", wantHints)
	}
	clickLabeledControl(t, driver, "(*Server).ServeHTTP")
	doc := state.space.Current()
	if doc.Editor.Caret() != doc.Editor.RuneIndex(strings.Index(value, "ServeHTTP")) || doc.Editor.String() != value || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatalf("function navigation changed source: caret=%d text=%q", doc.Editor.Caret(), doc.Editor.String())
	}
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if got := doc.Editor.String(); got != strings.Replace(value, "ServeHTTP", "XServeHTTP", 1) {
		t.Fatalf("first key after symbol click did not reach the editor: %q", got)
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != value || doc.Dirty() {
		t.Fatalf("undo after function activation: text=%q dirty=%v handled=%v err=%v", doc.Editor.String(), doc.Dirty(), handled, err)
	}
	clickID(t, driver, functionFilterID)
	if err := driver.Type("main"); err != nil {
		t.Fatal(err)
	}
	seenMain, seenMethod := false, false
	for _, hit := range driver.Snapshot().Hits {
		seenMain = seenMain || hit.ID.Scope() == functionSymbolScope && hit.Label == "main"
		seenMethod = seenMethod || hit.ID.Scope() == functionSymbolScope && strings.Contains(hit.Label, "ServeHTTP")
	}
	if !seenMain || seenMethod {
		t.Fatalf("filtered functions: main=%v method=%v", seenMain, seenMethod)
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != doc.Editor.RuneIndex(strings.Index(value, "main()")) || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatal("keyboard function navigation did not select main safely")
	}
}

func TestFunctionListSharesDockWithWorkspaceAndMapAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("package main\nfunc left() {}")
	state.command("Go")
	right := state.space.NewDocument()
	right.Editor.Insert("package main\nfunc right() {}")
	state.command("Go")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	state.folderVisible, state.documentMapVisible = true, true
	state.command("Function List")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if !state.functionListVisible || state.activeLeftDock != "functions" {
		t.Fatal("Function List did not take the left dock tab")
	}
	var leftEditor, rightEditor, mapBounds gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		switch hit.ID {
		case editorID:
			leftEditor = hit.Bounds
		case rightEditorID:
			rightEditor = hit.Bounds
		case documentMapID:
			mapBounds = hit.Bounds
		}
	}
	if leftEditor.X < state.folderPanelWidth(640) || rightEditor.X+rightEditor.Width > mapBounds.X || mapBounds.X+mapBounds.Width > 640 || mapBounds.Y+mapBounds.Height > 400-statusHeight {
		t.Fatalf("compact dock/editor/map overlap: left=%+v right=%+v map=%+v", leftEditor, rightEditor, mapBounds)
	}
	clickID(t, driver, leftDockWorkspaceID)
	if state.activeLeftDock != "workspace" || !state.folderVisible {
		t.Fatal("Workspace tab did not restore its panel")
	}
	clickID(t, driver, leftDockFunctionsID)
	clickLabeledControl(t, driver, "right")
	if right.Editor.Caret() != right.Editor.RuneIndex(strings.Index(right.Editor.String(), "right")) || left.Editor.Caret() != left.Editor.RuneCount() {
		t.Fatalf("function navigation reached wrong split pane: left=%d right=%d", left.Editor.Caret(), right.Editor.Caret())
	}
	leftBefore, rightBefore := left.Editor.String(), right.Editor.String()
	if err := driver.Type("X"); err != nil {
		t.Fatal(err)
	}
	if left.Editor.String() != leftBefore || right.Editor.String() != strings.Replace(rightBefore, "right", "Xright", 1) {
		t.Fatalf("first key in split view: left=%q right=%q", left.Editor.String(), right.Editor.String())
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || right.Editor.String() != rightBefore {
		t.Fatalf("undo after split activation: text=%q handled=%v err=%v", right.Editor.String(), handled, err)
	}
	clickID(t, driver, functionCloseID)
	if state.functionListVisible || !state.folderVisible || state.activeLeftDock != "workspace" || right.Editor.String() != "package main\nfunc right() {}" {
		t.Fatal("closing Function List lost the workspace or edited the active pane")
	}
}

func TestFunctionListRejectsStaleKeyboardResultAfterEdit(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("package main\nfunc old() {}\n")
	state.command("Go")
	state.command("Function List")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, functionFilterID)
	doc.Editor.ReplaceRange(text.Range{Start: 0, End: doc.Editor.RuneCount()}, "package main\n")
	doc.Editor.MoveTo(0, false)
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 0 || doc.Editor.String() != "package main\n" || state.functionListVisible != true {
		t.Fatalf("stale function selection changed source: caret=%d text=%q", doc.Editor.Caret(), doc.Editor.String())
	}
}

func TestFunctionListRejectsEnterBeforeFilterRepaint(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "package main\nfunc first() {}\nfunc second() {}\n"
	doc.Editor.Insert(source)
	doc.Editor.MoveTo(0, false)
	state.command("Go")
	state.command("Function List")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, functionFilterID)
	// Deliver Enter against the last frame before repainting the edited filter.
	state.functionFilter.Insert("second")
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 0 || doc.Editor.String() != source {
		t.Fatalf("Enter activated the old filter result: caret=%d", doc.Editor.Caret())
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if want := strings.Index(source, "second"); doc.Editor.Caret() != want || doc.Editor.String() != source {
		t.Fatalf("fresh Enter reached caret %d, want %d", doc.Editor.Caret(), want)
	}
}

func TestFunctionListRejectsReleaseAfterFilterReplacesPressedRow(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	const source = "package main\nfunc first() {}\nfunc second() {}\n"
	doc.Editor.Insert(source)
	doc.Editor.MoveTo(0, false)
	state.command("Go")
	state.command("Function List")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	var first gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == functionSymbolScope && hit.Label == "first" {
			first = hit.Bounds
		}
	}
	if first.Width == 0 {
		t.Fatal("first symbol is not visible")
	}
	x, y := first.X+first.Width/2, first.Y+first.Height/2
	if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	state.functionFilter.Insert("second")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.Caret() != 0 || doc.Editor.String() != source {
		t.Fatalf("release activated the replacement row: caret=%d", doc.Editor.Caret())
	}
	clickLabeledControl(t, driver, "second")
	if want := strings.Index(source, "second"); doc.Editor.Caret() != want || doc.Editor.String() != source {
		t.Fatalf("fresh click reached caret %d, want %d", doc.Editor.Caret(), want)
	}
}

func TestFunctionListVisibilityAndActiveDockSurviveRestart(t *testing.T) {
	root := t.TempDir()
	path, sessionPath := filepath.Join(root, "source.go"), filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.folderVisible = true
	state.command("Function List")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || !stored.FunctionListVisible || stored.ActiveLeftDock != "functions" {
		t.Fatalf("saved function dock = %+v, %v", stored, err)
	}
	for _, paths := range [][]string{nil, {path}} {
		restarted := newStartupModel(paths, sessionPath)
		if !restarted.functionListVisible || !restarted.folderVisible || restarted.activeLeftDock != "functions" {
			t.Fatalf("restored function dock with args=%v: visible=%v dock=%q", paths, restarted.functionListVisible, restarted.activeLeftDock)
		}
	}
}

func TestFunctionListWheelReachesLongListAndUnsupportedModeClearsResults(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	var source strings.Builder
	source.WriteString("package main\n")
	for index := range 80 {
		fmt.Fprintf(&source, "func item%d() {}\n", index)
	}
	doc.Editor.Insert(source.String())
	state.command("Go")
	state.command("Function List")
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == functionPanelID {
			if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 40, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if state.functionOffset == 0 {
		t.Fatal("long Function List ignored wheel input")
	}
	state.command("HTML")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == functionSymbolScope {
			t.Fatalf("unsupported HTML mode retained symbol: %+v", hit)
		}
	}
	if doc.Editor.String() != source.String() {
		t.Fatal("switching Function List language edited source")
	}
}

func TestFunctionListRefreshesUnsavedTextAndNavigatesReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("package main\nfunc first() {}\n")
	doc.Editor.ClearHistory()
	state.command("Go")
	state.command("Function List")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("func second() {}\n")
	doc.Editor.ClearHistory()
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	state.readOnly[doc] = true
	clickLabeledControl(t, driver, "second")
	if doc.Editor.Caret() != doc.Editor.RuneIndex(strings.Index(doc.Editor.String(), "second")) || !doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatalf("read-only live symbol navigation = caret:%d dirty:%v", doc.Editor.Caret(), doc.Dirty())
	}
}

func TestFunctionListViewMenuFitsCompactViewport(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	seen := false
	revealViewMenuItem(t, driver, 18)
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(18) {
			seen = true
			if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
				t.Fatalf("Function List menu row overlaps status: %+v", hit.Bounds)
			}
		}
	}
	if !seen {
		t.Fatal("Function List is not reachable from compact View menu")
	}
	clickID(t, driver, menuItemScope.At(18))
	if !state.functionListVisible {
		t.Fatal("Function List command did not open dock")
	}
}

func TestFunctionListViewMenuReflectsCheckedStateAndClosesDock(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(18))
	clickID(t, driver, menuScope.At(3))
	checked := false
	for _, hit := range driver.Snapshot().Hits {
		checked = checked || hit.ID == menuItemScope.At(18) && hit.Checked
	}
	if !checked {
		t.Fatal("open Function List was not checked in View menu")
	}
	clickID(t, driver, menuItemScope.At(18))
	if state.functionListVisible || state.folderPanelWidth(640) != 0 {
		t.Fatal("second View command did not close the dock")
	}
}
