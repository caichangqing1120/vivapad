package document

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func savedHistoryDocument(t *testing.T) *Document {
	t.Helper()
	doc := New("history.txt")
	doc.Editor.Insert("abc")
	if err := doc.Save(filepath.Join(t.TempDir(), "history.txt")); err != nil {
		t.Fatal(err)
	}
	if doc.Dirty() {
		t.Fatal("saved document is dirty")
	}
	return doc
}

func TestClearChangesPreservesEqualTextDirtyAndRebindsSharedHistory(t *testing.T) {
	doc := savedHistoryDocument(t)
	doc.Editor.MoveTo(1, false)
	doc.Editor.Insert("x")
	doc.Editor.DeleteBackward()
	if doc.Editor.String() != "abc" || !doc.Dirty() {
		t.Fatalf("expected equal-text dirty document, got %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	left, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	right, err := doc.ChangeHistory()
	if err != nil || left != right {
		t.Fatalf("clones did not share the source: %v", err)
	}
	if err := doc.ClearChangeHistory(); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "abc" || !doc.Dirty() || doc.Editor.CanUndo() || doc.Editor.CanRedo() {
		t.Fatalf("clear changed text, dirty state, or undo history: %q dirty=%v", doc.Editor.String(), doc.Dirty())
	}
	if err := left.Sync(); err == nil {
		t.Fatal("old consumer accepted the reset generation")
	}
	fresh, err := doc.ChangeHistory()
	if err != nil || fresh == left {
		t.Fatalf("clear did not rebind one fresh consumer: %v", err)
	}
	if shared, err := doc.ChangeHistory(); err != nil || shared != fresh {
		t.Fatalf("clones no longer share the new consumer: %v", err)
	}
	if err := doc.Save(""); err != nil || doc.Dirty() {
		t.Fatalf("save after clear did not restore clean boundary: %v", err)
	}
	doc.Editor.Insert("z")
	if current, err := doc.ChangeHistory(); err != nil || current != fresh {
		t.Fatalf("later edit lost history: %v", err)
	}
}

func TestClearChangesKeepsFormatAndRecoveryDirty(t *testing.T) {
	doc := savedHistoryDocument(t)
	doc.SetUTF8BOM(true)
	if err := doc.ClearChangeHistory(); err != nil || !doc.Dirty() || !doc.UTF8BOM() {
		t.Fatalf("clear lost format dirty state: %v dirty=%v", err, doc.Dirty())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.MarkRecovered()
	if err := doc.ClearChangeHistory(); err != nil || !doc.Dirty() {
		t.Fatalf("clear lost recovery dirty state: %v dirty=%v", err, doc.Dirty())
	}
}

func TestClearChangesRefusesCompositionAndUnboundEditors(t *testing.T) {
	doc := savedHistoryDocument(t)
	before, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("cannot enable tagged composition")
	}
	doc.Editor.MoveTo(1, false)
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "ni"}) {
		t.Fatal("cannot start tagged composition")
	}
	if !doc.Editor.CompositionActive() {
		t.Fatal("expected active composition")
	}
	if err := doc.ClearChangeHistoryPreflight(); err == nil {
		t.Fatal("preflight accepted a live composition")
	}
	if err := doc.ClearChangeHistory(); err == nil || !doc.Editor.CompositionActive() {
		t.Fatalf("clear accepted or discarded active composition: %v", err)
	}
	if current, err := doc.ChangeHistory(); err != nil || current != before {
		t.Fatalf("refused clear changed the consumer: %v", err)
	}

	draft := NewDraft("preview", text.NewEditor("x"))
	if err := draft.ClearChangeHistory(); !errors.Is(err, errNoChangeHistorySource) {
		t.Fatalf("draft clear error = %v", err)
	}
	doc = savedHistoryDocument(t)
	doc.Editor = text.NewEditor("replacement")
	if err := doc.ClearChangeHistory(); !errors.Is(err, errNoChangeHistorySource) || doc.Editor.String() != "replacement" {
		t.Fatalf("replacement clear error = %v, text=%q", err, doc.Editor.String())
	}
}
