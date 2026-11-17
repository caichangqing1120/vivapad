package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/printing"
)

func requireEditorCommandFocus(t *testing.T, driver *app.Driver, want gfx.ID) {
	t.Helper()
	for range 2 {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if got := driver.Snapshot().Focused; got != want {
		t.Fatalf("editor command focus = %+v, want %+v", got, want)
	}
}

func TestCommentMenuReturnsFocusForNativeUndoAndTyping(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	clickID(t, driver, editorID)
	if err := driver.Type("alpha"); err != nil {
		t.Fatal(err)
	}
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(1))
	requireEditorCommandFocus(t, driver, editorID)
	if doc.Editor.String() != "// alpha" {
		t.Fatal("comment menu did not edit its source")
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || doc.Editor.String() != "alpha" || doc.Editor.CanUndo() {
		t.Fatalf("Undo immediately after comment = handled %v, text %q, %v", handled, doc.Editor.String(), err)
	}
	if err := driver.Type("!"); err != nil || doc.Editor.String() != "!alpha" {
		t.Fatalf("typing immediately after menu Undo = %q, %v", doc.Editor.String(), err)
	}
}

func TestPrintNowMenuReturnsFocusWithoutChangingSelection(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	clickID(t, driver, editorID)
	doc.Editor.Insert("alpha\nPRINTJOB")
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 6})
	before, caret, value, dirty := doc.Editor.Selection(), doc.Editor.Caret(), doc.Editor.String(), doc.Dirty()
	if before.Start == before.End {
		t.Fatal("test did not establish an editor selection")
	}
	called := false
	state.printJob = func(job printing.Job) (int, error) {
		called = true
		if job.Scope != printing.ScopeAll || job.Text != value || job.Interactive || job.Selection.Start != job.Selection.End {
			t.Fatalf("Print Now selection contract = %+v", job)
		}
		return 1, nil
	}
	clickID(t, driver, menuScope.At(0))
	revealFileMenuItem(t, driver, 22)
	clickID(t, driver, menuItemScope.At(22))
	if !called {
		t.Fatal("Print Now was not dispatched from the actual File menu")
	}
	requireEditorCommandFocus(t, driver, editorID)
	if doc.Editor.Selection() != before || doc.Editor.Caret() != caret ||
		doc.Editor.String() != value || doc.Dirty() != dirty {
		t.Fatal("Print Now changed text, directional selection, caret or dirty state")
	}
}

func TestPrintDialogCancelReturnsFocusWithoutChangingSelection(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	clickID(t, driver, editorID)
	doc.Editor.Insert("alpha\nPRINTJOB")
	doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 6})
	before, caret, value, dirty := doc.Editor.Selection(), doc.Editor.Caret(), doc.Editor.String(), doc.Dirty()
	called := false
	state.printJob = func(job printing.Job) (int, error) {
		called = true
		if !job.Interactive || job.Text != value || job.Selection.End <= job.Selection.Start {
			t.Fatalf("Print... selection contract = %+v", job)
		}
		return 0, printing.ErrPrintCanceled
	}
	clickID(t, driver, menuScope.At(0))
	revealFileMenuItem(t, driver, 21)
	clickID(t, driver, menuItemScope.At(21))
	if !called {
		t.Fatal("Print... was not dispatched from the actual File menu")
	}
	requireEditorCommandFocus(t, driver, editorID)
	if doc.Editor.Selection() != before || doc.Editor.Caret() != caret ||
		doc.Editor.String() != value || doc.Dirty() != dirty {
		t.Fatal("Print... Cancel changed text, directional selection, caret or dirty state")
	}
}

func TestEditorCommandFamiliesReturnFocusWithoutChangingTheirSelectionAgain(t *testing.T) {
	for _, command := range []string{
		"Single Line Comment", "UPPERCASE", "Indent Lines", "Duplicate Line",
		"Sort Lines Ascending", "Windows (CR LF)", "Select All", "Go", "Stop Recording",
		"Word Wrap", "Show Line Numbers", "Zoom In", "Show Space and Tab", "First Tab", "Fold All",
	} {
		t.Run(command, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("beta\nalpha")
			doc.Editor.ClearHistory()
			state.languages[doc] = ".go"
			state.configureCode(doc)
			doc.Editor.SetSelection(text.Range{Start: doc.Editor.RuneCount(), End: 0})
			clickID(t, driver, menuScope.At(1))
			if err := driver.Move(1, 1); err != nil {
				t.Fatal(err)
			}
			state.command(command)
			selection, caret, value := doc.Editor.Selection(), doc.Editor.Caret(), doc.Editor.String()
			state.invalidator.Invalidate()
			requireEditorCommandFocus(t, driver, editorID)
			if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.String() != value {
				t.Fatal("focus restoration changed command text or directional selection")
			}
		})
	}
}

func TestMacroMenuKeepsEditorInputAcrossRecordStopAndPlayback(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha")
	doc.Editor.ClearHistory()
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(0))
	requireEditorCommandFocus(t, driver, editorID)
	if !state.macroRecording {
		t.Fatal("Record menu did not start recording")
	}
	if err := driver.Type("!"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(1))
	requireEditorCommandFocus(t, driver, editorID)
	if state.macroRecording || len(state.macroLast) != 1 {
		t.Fatal("focus changes canceled or changed the recording")
	}
	clickID(t, driver, menuScope.At(8))
	clickID(t, driver, menuItemScope.At(2))
	requireEditorCommandFocus(t, driver, editorID)
	if doc.Editor.String() != "alpha!!" {
		t.Fatalf("macro menu result = %q", doc.Editor.String())
	}
	if err := driver.Type("?"); err != nil || doc.Editor.String() != "alpha!!?" {
		t.Fatalf("typing after macro menu = %q, %v", doc.Editor.String(), err)
	}
}

func TestEditorCommandFocusDoesNotStealDialogOrQueryUndo(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha")
	doc.Editor.ClearHistory()
	clickID(t, driver, searchID)
	clickID(t, driver, queryID)
	if err := driver.Type("needle"); err != nil {
		t.Fatal(err)
	}
	state.command("Record Macro")
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, queryID)
	if state.macroRecording {
		t.Fatal("Record accepted while a query owns input")
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || state.query.String() != "" || doc.Editor.String() != "alpha" || doc.Editor.CanUndo() {
		t.Fatal("query Undo was redirected to the document")
	}
	state.searchOpen = false
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, columnTextID)
	if err := driver.Type("X"); err != nil || state.columnText.String() != "X" || doc.Editor.String() != "alpha" {
		t.Fatal("editor focus request stole the Column Editor input")
	}
}

func TestEditorCommandFocusUsesTheActiveSubViewAndPreservesReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	left.Editor.ClearHistory()
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	right.Editor.ClearHistory()
	state.languages[right] = ".go"
	state.configureCode(right)
	state.command("Split View")
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(1))
	requireEditorCommandFocus(t, driver, rightEditorID)
	if left.Editor.String() != "left" || left.Editor.CanUndo() || right.Editor.String() != "// right" || state.activePane != 1 {
		t.Fatal("comment focus or edit crossed document views")
	}
	state.command("Read-Only on Current Document")
	state.command("Single Line Uncomment")
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, rightEditorID)
	if right.Editor.String() != "// right" || state.notice != "Read-only document" {
		t.Fatal("focus restoration bypassed read-only protection")
	}
}

func TestPendingEditorFocusYieldsToNewDialogOrModelessQuery(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		command string
		focus   gfx.ID
	}{
		{"column dialog", "Column Editor...", columnTextID},
		{"modeless mark", "Mark...", queryID},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("alpha")
			state.languages[doc] = ".go"
			state.configureCode(doc)
			clickID(t, driver, editorID)
			state.command("Single Line Comment")
			state.command(scenario.command)
			state.invalidator.Invalidate()
			for range 3 {
				requireEditorCommandFocus(t, driver, scenario.focus)
			}
		})
	}
	state, driver := startDriver(t)
	state.languages[state.space.Current()] = ".go"
	state.space.Current().Editor.Insert("alpha")
	clickID(t, driver, editorID)
	state.command("Single Line Comment")
	if handled, err := driver.ShortcutChord(primary(), 'f'); err != nil || !handled {
		t.Fatalf("Find chord = %v, %v", handled, err)
	}
	for range 3 {
		requireEditorCommandFocus(t, driver, queryID)
	}
	clickID(t, driver, editorID)
	state.command("Single Line Comment")
	clickID(t, driver, queryID)
	for range 3 {
		requireEditorCommandFocus(t, driver, queryID)
	}
}

func TestExplicitEditorCommandReturnsFromModelessMarkWithoutClosingIt(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("alpha")
	doc.Editor.ClearHistory()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	state.command("Mark...")
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, queryID)
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(1))
	requireEditorCommandFocus(t, driver, editorID)
	if !state.markDialogOpen || doc.Editor.String() != "// alpha" {
		t.Fatal("comment closed the modeless tool or missed the source")
	}
	if err := driver.Type("!"); err != nil || doc.Editor.String() != "// alpha!" || state.query.String() != "" {
		t.Fatal("typing after the editor command went to the modeless query")
	}
}

func TestCaptureEditorCommandFocusWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_COMMAND_FOCUS_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_COMMAND_FOCUS_SHOTS to capture editor command focus")
	}
	state := newModel()
	doc := state.space.Current()
	doc.Name = "focus-return.go"
	doc.Editor.Insert("alpha keep beta\nleft keep right")
	doc.Editor.ClearHistory()
	state.languages[doc] = ".go"
	state.configureCode(doc)
	doc.Editor.SetSelection(text.Range{Start: 6, End: 10})
	state.styleToken(0, true)
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	doc.Editor.MoveTo(0, false)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	driver := session.Driver()
	clickID(t, driver, rightEditorID)
	doc.Editor.MoveTo(0, false)
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, rightEditorID)
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(19))
	clickID(t, driver, commentScope.At(1))
	requireEditorCommandFocus(t, driver, rightEditorID)
	capture := func(name string, width, height int) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s editor focus layout: %+v", name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "editor-focus-"+name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	capture("comment-desktop", 1280, 800)
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled {
		t.Fatal("editor command focus screenshot could not Undo")
	}
	if err := driver.Type("!"); err != nil || doc.Editor.String() != "!alpha keep beta\nleft keep right" || len(state.styleSpans(doc, 0)) != 2 {
		t.Fatalf("editor focus screenshot typed into wrong target: %q, %v", doc.Editor.String(), err)
	}
	capture("typed-compact", 640, 400)
	state.command("Column Editor...")
	state.invalidator.Invalidate()
	requireEditorCommandFocus(t, driver, columnTextID)
	capture("dialog-compact", 640, 400)
}

func TestPendingEditorFocusYieldsToMenuOverflow(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("alpha")
	state.languages[doc] = ".go"
	state.configureCode(doc)
	clickID(t, driver, editorID)
	state.command("Single Line Comment")
	clickID(t, driver, menuOverflowID)
	if !state.menuOverflowOpen {
		t.Fatal("overflow fixture did not open")
	}
	for range 3 {
		requireEditorCommandFocus(t, driver, menuOverflowID)
	}
}
