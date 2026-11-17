package document

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

// H04: Clear Change History preserves the real unsaved boundary. Every test
// first performs a real Save so the document's saved bytes, digests and save
// point agree; otherwise Dirty() would stay true for unrelated reasons and the
// assertions could not detect a reset that laundered the boundary.

func savedDocument(t *testing.T, name, value string) (*Document, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	doc := New(name)
	doc.Editor.Insert(value)
	if err := doc.Save(path); err != nil {
		t.Fatalf("save baseline: %v", err)
	}
	if doc.Dirty() {
		t.Fatal("precondition: a freshly saved document must be clean")
	}
	return doc, path
}

func TestClearChangesPreservesEqualTextDirty(t *testing.T) {
	doc, path := savedDocument(t, "equal.txt", "abc")
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("x")
	doc.Editor.DeleteBackward()
	if doc.Editor.String() != "abc" || !doc.Dirty() {
		t.Fatalf("precondition text=%q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	before, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("precondition consumer: %v", err)
	}
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("ClearChangeHistory: %v", err)
	}
	if doc.Editor.String() != "abc" {
		t.Fatalf("clear changed the text: %q", doc.Editor.String())
	}
	if !doc.Dirty() {
		t.Fatal("equal-text dirty document became clean")
	}
	if doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("clear kept history units")
	}
	// The old generation must be superseded exactly once.
	if err := before.Sync(); err == nil {
		t.Fatal("the pre-reset consumer still accepts frames from the old generation")
	}
	rebuilt, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("clear must leave an available consumer: %v", err)
	}
	if rebuilt == before {
		t.Fatal("clear did not establish a new source generation")
	}
	if mask, err := rebuilt.LineMask(0, 0, 1); err != nil || mask != 0 {
		t.Fatalf("new generation mask=%#x err=%v, want no changes", mask, err)
	}
	// A later Save clears the dirty state again and a later edit is tracked.
	if err := doc.Save(path); err != nil {
		t.Fatalf("save after clear: %v", err)
	}
	if doc.Dirty() {
		t.Fatalf("save baseline after reset did not clear dirty (%q)", path)
	}
	doc.Editor.Insert("z")
	if _, err := doc.ChangeHistory(); err != nil {
		t.Fatalf("history after reset+edit: %v", err)
	}
	const modifiedEditionBit = 1 << 2
	if mask, err := rebuilt.LineMask(0, 0, 2); err != nil || mask&modifiedEditionBit == 0 {
		t.Fatalf("post-reset edit mask=%#x err=%v, want the modified edition bit", mask, err)
	}
}

func TestClearChangesKeepsSavePointInRedoDirty(t *testing.T) {
	doc, path := savedDocument(t, "redo.txt", "abc")
	doc.Editor.Insert("x")
	if err := doc.Save(path); err != nil {
		t.Fatalf("save future boundary: %v", err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "abc" || !doc.Editor.CanRedo() || !doc.Dirty() {
		t.Fatal("precondition: saved bytes must be reachable only through Redo")
	}
	selection := doc.Editor.Selection()
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("clear with future save point: %v", err)
	}
	if doc.Editor.String() != "abc" || doc.Editor.Selection() != selection || !doc.Dirty() ||
		doc.Editor.AtSavePoint() || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("clear changed the document or made the future save point reachable")
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "abcx" {
		t.Fatalf("clear changed saved bytes: %q, %v", disk, err)
	}
	if err := doc.Save(path); err != nil || doc.Dirty() {
		t.Fatalf("new save did not establish a clean boundary: %v", err)
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "abc" {
		t.Fatalf("new save wrote %q, %v", disk, err)
	}
	doc.Editor.Insert("z")
	if !doc.Editor.Undo() || doc.Dirty() || !doc.Editor.Redo() || !doc.Dirty() {
		t.Fatal("post-clear save, Undo and Redo lost the new saved boundary")
	}
}

func TestClearChangesKeepsDiscardedSavePointDirty(t *testing.T) {
	doc, path := savedDocument(t, "branch.txt", "abc")
	doc.Editor.Insert("x")
	if err := doc.Save(path); err != nil {
		t.Fatalf("save future boundary: %v", err)
	}
	if !doc.Editor.Undo() {
		t.Fatal("undo saved edit")
	}
	doc.Editor.Insert("z")
	if doc.Editor.String() != "abcz" || doc.Editor.CanRedo() || doc.Editor.AtSavePoint() || !doc.Dirty() {
		t.Fatal("precondition: the saved branch must be discarded")
	}
	selection := doc.Editor.Selection()
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("clear discarded branch: %v", err)
	}
	if doc.Editor.String() != "abcz" || doc.Editor.Selection() != selection || !doc.Dirty() ||
		doc.Editor.AtSavePoint() || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatal("clear restored an unreachable saved boundary")
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "abcx" {
		t.Fatalf("clear changed saved bytes: %q, %v", disk, err)
	}
	if err := doc.Save(path); err != nil || doc.Dirty() {
		t.Fatalf("save after discarded branch did not clear dirty: %v", err)
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != "abcz" {
		t.Fatalf("new branch save wrote %q, %v", disk, err)
	}
}

func TestClearChangesKeepsFormatAndRecoveryDirty(t *testing.T) {
	doc, _ := savedDocument(t, "format.txt", "one\r\ntwo\r\n")
	doc.uniformEOL = true
	if !doc.Dirty() {
		t.Fatal("precondition: EOL normalisation is dirty")
	}
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("ClearChangeHistory: %v", err)
	}
	if !doc.Dirty() {
		t.Fatal("clear laundered the EOL dirty state")
	}
	if got := doc.Editor.String(); got != "one\r\ntwo\r\n" {
		t.Fatalf("clear changed the bytes: %q", got)
	}
	doc.uniformEOL = false
	doc.MarkRecovered()
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("ClearChangeHistory on a recovered document: %v", err)
	}
	if !doc.Dirty() {
		t.Fatal("clear laundered the recovery dirty state")
	}
	if got := doc.Editor.String(); got != "one\r\ntwo\r\n" {
		t.Fatalf("clear changed the recovered bytes: %q", got)
	}
}

func TestClearChangesResetsSharedCloneOnlyOnce(t *testing.T) {
	doc, _ := savedDocument(t, "shared.txt", strings.Repeat("line\n", 4))
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("dirty")
	if !doc.Dirty() {
		t.Fatal("precondition: edited document must be dirty")
	}
	// Both panes of a split view share one Document, so one clear must reset the
	// source exactly once and both panes must observe the same new generation.
	left, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("left pane consumer: %v", err)
	}
	right, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("right pane consumer: %v", err)
	}
	if left != right {
		t.Fatal("split panes do not share one consumer")
	}
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatalf("ClearChangeHistory: %v", err)
	}
	if err := left.Sync(); err == nil {
		t.Fatal("the shared pre-reset consumer still accepts the old generation")
	}
	first, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("first pane after reset: %v", err)
	}
	second, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("second pane after reset: %v", err)
	}
	if first != second || first == left {
		t.Fatal("the reset did not rebind exactly one shared source")
	}
	if !doc.Dirty() {
		t.Fatal("clear laundered the shared document's dirty state")
	}
	if doc.Editor.CanUndo() {
		t.Fatal("clear left undo units on the shared document")
	}
}

func TestClearChangesRefusesActiveCompositionWithoutSideEffects(t *testing.T) {
	doc, _ := savedDocument(t, "compose.txt", "abc")
	if !doc.Editor.EnableTentativeComposition() {
		t.Skip("tentative composition is unavailable in this build")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "か"}) {
		t.Fatal("precondition: the composition update must be handled")
	}
	consumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("precondition consumer: %v", err)
	}
	if err := doc.ClearChangeHistory(); err == nil {
		t.Fatal("clear accepted a live composition")
	}
	if !doc.Editor.CompositionActive() {
		t.Fatal("the refused clear discarded the live composition")
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) {
		t.Fatal("the composition is no longer cancellable after the refusal")
	}
	if got := doc.Editor.String(); got != "abc" {
		t.Fatalf("refused clear changed the text: %q", got)
	}
	if !doc.Dirty() == false {
		t.Fatal("a cancelled composition must return to the saved bytes")
	}
	if err := consumer.Sync(); err != nil {
		t.Fatalf("the refused clear invalidated the consumer: %v", err)
	}
}

func TestClearChangesRefusesDraftAndReplacedEditor(t *testing.T) {
	var absent *Document
	if err := absent.ClearChangeHistoryPreflight(); !errors.Is(err, errNoChangeHistorySource) {
		t.Fatalf("nil document preflight error = %v, want the no-source error", err)
	}
	if err := absent.ClearChangeHistory(); !errors.Is(err, errNoChangeHistorySource) {
		t.Fatalf("nil document clear error = %v, want the no-source error", err)
	}
	draft := NewDraft("draft", text.NewEditor("x"))
	if err := draft.ClearChangeHistory(); !errors.Is(err, errNoChangeHistorySource) {
		t.Fatalf("draft clear error = %v, want the no-source error", err)
	}
	doc, _ := savedDocument(t, "replaced.txt", "abc")
	doc.Editor = text.NewEditor("replaced")
	if err := doc.ClearChangeHistory(); !errors.Is(err, errNoChangeHistorySource) {
		t.Fatalf("replaced-editor clear error = %v, want the no-source error", err)
	}
	if doc.Editor.String() != "replaced" {
		t.Fatal("the refused clear touched the replaced editor")
	}
}
