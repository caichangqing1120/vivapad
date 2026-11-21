package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

// H06: navigation and Clear consume the real consumer result.

// openedDocument writes a file and opens it, so the document starts with a real
// saved baseline and no change-history editions (a brand-new saved document would
// mark every line as saved and every line would count as a change).
func openedDocument(t *testing.T, value string) *model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nav.txt")
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	state, _ := startDriver(t)
	if err := state.space.Open(path); err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	doc := state.space.Current()
	if doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatal("precondition: an opened file must be clean without history")
	}
	return state
}

func TestNavigateChangeSkipsBlocksAndWraps(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\nfour\n")
	doc := state.space.Current()
	// Non-contiguous changes: line 1 and line 3, so one block must be skipped.
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(len([]rune("oneX\ntwo\nthree\n")), false)
	doc.Editor.Insert("Y")
	doc.Editor.MoveTo(0, false)
	line := func() int {
		return strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	}
	state.command("Next Change")
	if line() != 1 {
		t.Fatalf("first Next Change went to line %d, want the first changed line 1", line())
	}
	state.command("Next Change")
	if line() != 3 {
		t.Fatalf("second Next Change went to line %d, want the next block at line 3", line())
	}
	state.command("Next Change")
	if line() != 1 {
		t.Fatalf("Next Change did not wrap to the first changed line, went to %d", line())
	}
	state.command("Previous Change")
	if line() != 3 {
		t.Fatalf("Previous Change from line 1 went to %d, want 3 (wrap)", line())
	}
}

func TestNavigateChangeWithoutChangesDoesNotMove(t *testing.T) {
	state := openedDocument(t, "one\ntwo\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(3, false)
	beforeCaret, beforeSelection := doc.Editor.Caret(), doc.Editor.Selection()
	state.command("Next Change")
	if doc.Editor.Caret() != beforeCaret || doc.Editor.Selection() != beforeSelection {
		t.Fatalf("navigation moved caret from %d to %d without a changed line", beforeCaret, doc.Editor.Caret())
	}
	if state.notice != "No changes to navigate" {
		t.Fatalf("notice = %q", state.notice)
	}
}

func TestNavigateChangeWrapsToFinalEmptyLineAfterUnicode(t *testing.T) {
	state := openedDocument(t, "\u4e00\r\nsecond\r\n\u597d")
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.DeleteBackward()
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(0, false)
	const live = "X\u4e00\nsecond\n"
	state.command("Previous Change")
	if doc.Editor.String() != live || doc.Editor.Caret() != len([]rune(live)) || state.notice != "" {
		t.Fatalf("did not wrap to the final deletion point: caret=%d notice=%q", doc.Editor.Caret(), state.notice)
	}
	state.command("Next Change")
	if doc.Editor.String() != live || doc.Editor.Caret() != 0 || state.notice != "" || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatalf("did not wrap back to the first change: caret=%d notice=%q", doc.Editor.Caret(), state.notice)
	}
}

func TestClearChangeHistoryCommandPreservesDirtyAndText(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	text := doc.Editor.String()
	if !doc.Dirty() {
		t.Fatal("precondition: the edited document must be dirty")
	}
	state.command("Clear Change History")
	if state.notice != "Change history cleared" {
		t.Fatalf("notice = %q", state.notice)
	}
	if doc.Editor.String() != text {
		t.Fatalf("clear changed the text: %q", doc.Editor.String())
	}
	if !doc.Dirty() {
		t.Fatal("clear laundered the unsaved document")
	}
	if doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("clear kept history units")
	}
	state.command("Next Change")
	if state.notice != "No changes to navigate" {
		t.Fatalf("after clear the document must report no changes, notice = %q", state.notice)
	}
}

func TestChangeHistoryNavigationRefusesUnavailableSource(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\n")
	// A replaced editor is not a bound source: the document reports the explicit
	// unavailable error instead of answering "no changes".
	doc.Editor = text.NewEditor("replaced")
	before := doc.Editor.Caret()
	state.command("Next Change")
	if doc.Editor.Caret() != before {
		t.Fatal("navigation moved the caret on an unavailable source")
	}
	if state.notice == "" || state.notice == "No changes to navigate" {
		t.Fatalf("unavailable history must report its reason, notice = %q", state.notice)
	}
	state.command("Clear Change History")
	if !strings.Contains(state.notice, "change-history source") {
		t.Fatalf("clear on an unavailable source notice = %q", state.notice)
	}
}

func TestNavigateChangeSkipsTheWholeAdjacentBlock(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\nfour\nfive\n")
	doc := state.space.Current()
	// Lines 1 and 2 are one contiguous changed block; line 4 is a separate block.
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(len([]rune("oneX\ntwo\n")), false)
	doc.Editor.Insert("Y")
	doc.Editor.MoveTo(len([]rune("oneX\ntwoY\nthree\nfour\n")), false)
	doc.Editor.Insert("Z")
	line := func() int {
		return strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	}
	// Caret inside the first block: Next skips both of its lines and lands on 4.
	doc.Editor.MoveTo(len([]rune("oneX\n")), false)
	if start := line(); start != 1 {
		t.Fatalf("precondition caret line = %d", start)
	}
	state.command("Next Change")
	if line() != 4 {
		t.Fatalf("Next from inside the block went to line %d, want the next block at 4", line())
	}
	// Previous from the block's LAST line also skips the whole block: a weaker
	// "exclude the current line" search would land on line 1 instead of wrapping.
	doc.Editor.MoveTo(len([]rune("oneX\ntwoY")), false)
	state.command("Previous Change")
	if line() != 4 {
		t.Fatalf("Previous from inside the block went to line %d, want 4 (wrapped)", line())
	}
}

func TestNavigateChangeWrapsWithinASingleBlock(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.Insert("X")
	doc.Editor.MoveTo(len([]rune("oneX\ntwo\n")), false)
	doc.Editor.Insert("Y")
	doc.Editor.MoveTo(len([]rune("oneX\n")), false)
	// Only one changed block exists, so both directions wrap to its first line.
	state.command("Next Change")
	line := func() int {
		return strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n")
	}
	if line() != 1 {
		t.Fatalf("Next with a single block went to line %d, want 1", line())
	}
	// Backward wrap is asymmetric: it lands on the last changed line.
	state.command("Previous Change")
	if line() != 2 {
		t.Fatalf("Previous with a single block went to line %d, want the last changed line 2", line())
	}
}

func TestNavigateChangeFindsDeletionOnlyTarget(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	// Deleting everything on line 1 leaves a deletion marker without an edition.
	doc.Editor.MoveTo(len([]rune("one\n")), false)
	doc.Editor.MoveTo(len([]rune("one\ntwo")), false)
	doc.Editor.SetSelection(text.Range{Start: len([]rune("one\n")), End: len([]rune("one\ntwo"))})
	doc.Editor.DeleteBackward()
	doc.Editor.MoveTo(0, false)
	state.command("Next Change")
	if line := strings.Count(doc.Editor.String()[:doc.Editor.ByteOffset(doc.Editor.Caret())], "\n"); line != 1 {
		t.Fatalf("deletion-only change not found: went to line %d, want 1", line)
	}
}

func TestNavigateChangeKeepsSelectionAndRecordsNoHistory(t *testing.T) {
	state := openedDocument(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	doc.Editor.MoveTo(len([]rune("one\ntwo\n")), false)
	doc.Editor.Insert("Z")
	doc.Editor.MoveTo(0, false)
	doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
	if doc.Editor.CanUndo() == false {
		t.Fatal("precondition: the edit must be undoable")
	}
	afterEdit := doc.Editor.String()
	state.command("Next Change")
	if !doc.Editor.CanUndo() {
		t.Fatal("navigation consumed the document's history")
	}
	if doc.Editor.String() != afterEdit {
		t.Fatalf("navigation changed the text: %q", doc.Editor.String())
	}
	// One Undo must reverse the known edit. If navigation had added a unit, this
	// Undo would consume the navigation unit and leave the text unchanged.
	doc.Editor.Undo()
	if doc.Editor.String() != "one\ntwo\nthree\n" {
		t.Fatalf("one Undo did not reverse the edit (navigation added a unit?): %q", doc.Editor.String())
	}
	// Navigation replaces the selection with the caret at the target line only via
	// GoToLine; it must not create a history unit.
	if doc.Editor.Selection().Start != doc.Editor.Selection().End {
		state.command("Previous Change")
		if doc.Editor.Selection().Start != doc.Editor.Selection().End {
			t.Fatal("navigation left a selection behind")
		}
	}
}

func TestClearChangeHistoryCommandRefusesLiveCompositionBeforeCancelling(t *testing.T) {
	state := openedDocument(t, "abc\n")
	doc := state.space.Current()
	if !doc.Editor.EnableTentativeComposition() {
		t.Skip("tentative composition is unavailable in this build")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "か"}) {
		t.Fatal("precondition: the composition update must be handled")
	}
	before := doc.Editor.String()
	if err := doc.ClearChangeHistoryPreflight(); err == nil {
		t.Fatal("preflight accepted a live composition")
	}
	state.command("Clear Change History")
	if !strings.Contains(state.notice, "composition") {
		t.Fatalf("command notice = %q, want the composition refusal", state.notice)
	}
	if !doc.Editor.CompositionActive() {
		t.Fatal("the command cancelled the live composition before refusing")
	}
	if doc.Editor.String() != before {
		t.Fatalf("the refused command mutated provisional text: %q -> %q", before, doc.Editor.String())
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) {
		t.Fatal("the composition is no longer cancellable after the refused command")
	}
	if got := doc.Editor.String(); got != "abc\n" {
		t.Fatalf("cancel did not restore the saved bytes: %q", got)
	}
	if doc.Dirty() {
		t.Fatal("a cancelled composition must leave the document clean")
	}
}
