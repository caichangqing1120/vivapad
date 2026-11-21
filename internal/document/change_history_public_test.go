package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestChangeHistorySyncDuringTentativePreeditKeepsSourceAvailable(t *testing.T) {
	doc := New("compose")
	consumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("cannot enable tagged composition")
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "に"}) {
		t.Fatal("preedit update rejected")
	}
	if current, err := doc.ChangeHistory(); err != nil || current != consumer {
		t.Fatalf("provisional paint invalidated the source: consumer=%p err=%v", current, err)
	}
	if mask, err := consumer.LineMask(0, len(doc.Editor.String()), len(doc.Editor.String())); err != nil || mask&4 == 0 {
		t.Fatalf("tentative producer frame lost its modified edition: mask=%d err=%v", mask, err)
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) {
		t.Fatal("preedit cancellation rejected")
	}
	if current, err := doc.ChangeHistory(); err != nil || current != consumer {
		t.Fatalf("cancel did not preserve the source: consumer=%p err=%v", current, err)
	}
	if mask, err := consumer.LineMask(0, len(doc.Editor.String()), len(doc.Editor.String())); err != nil || mask != 0 {
		t.Fatalf("cancellation left a change marker: mask=%d err=%v", mask, err)
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionResult, Text: "中"}) {
		t.Fatal("result rejected")
	}
	if current, err := doc.ChangeHistory(); err != nil || current != consumer {
		t.Fatalf("result did not reach the source: consumer=%p err=%v", current, err)
	}
	if mask, err := consumer.LineMask(0, len(doc.Editor.String()), len(doc.Editor.String())); err != nil || mask == 0 {
		t.Fatalf("committed result has no change metadata: mask=%d err=%v", mask, err)
	}
}

func TestDocumentBindsChangeHistoryBeforeFirstEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.txt")
	if err := os.WriteFile(path, []byte("a\r\nb"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, open := range []func() (*Document, error){
		func() (*Document, error) { return New("untitled"), nil },
		func() (*Document, error) { return Open(path) },
	} {
		doc, err := open()
		if err != nil {
			t.Fatal(err)
		}
		consumer, err := doc.ChangeHistory()
		if err != nil || consumer == nil {
			t.Fatalf("final editor did not bind a producer: %v", err)
		}
		doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
		doc.Editor.Insert("中")
		if current, err := doc.ChangeHistory(); err != nil || current != consumer {
			t.Fatalf("first edit lost the bound consumer: %v", err)
		}
		if mask, err := consumer.LineMask(0, len(doc.Editor.String()), len(doc.Editor.String())); err != nil || mask == 0 {
			t.Fatalf("first edit did not produce change metadata: %d, %v", mask, err)
		}
	}
}

func TestDocumentRejectsReplacedHistorySource(t *testing.T) {
	doc := New("source")
	if _, err := doc.ChangeHistory(); err != nil {
		t.Fatal(err)
	}
	doc.Editor = text.NewEditor(doc.Editor.String())
	if consumer, err := doc.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("replaced editor reused old history source: %v, %v", consumer, err)
	}
}

func TestDraftHasNoHistorySourceButCanValidateLineEndings(t *testing.T) {
	draft := NewDraft("preview", text.NewEditor("a\nb\nc"))
	if consumer, err := draft.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("transient draft exposed a user history source: %v, %v", consumer, err)
	}
	if err := draft.RestoreLineEndings(CRLF, "WL"); err != nil || draft.EOLSequence() != "WL" {
		t.Fatalf("transient draft lost its encoding preflight labels: sequence=%q error=%v", draft.EOLSequence(), err)
	}
}

func TestDocumentChangeHistorySaveBoundaryFollowsOnlySuccessfulSave(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("abc")
	consumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	mask, err := consumer.LineMask(0, 3, 3)
	if err != nil || mask == 0 {
		t.Fatalf("unsaved edit has no change marker: %d, %v", mask, err)
	}
	root := t.TempDir()
	if err := doc.SaveCopy(filepath.Join(root, "copy.txt")); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(root); err == nil {
		t.Fatal("directory destination accepted")
	}
	if current, err := doc.ChangeHistory(); err != nil || current != consumer {
		t.Fatalf("failed/copy save changed the source: %v", err)
	}
	if got, err := consumer.LineMask(0, 3, 3); err != nil || got != mask {
		t.Fatalf("failed/copy save changed editions: %d -> %d, %v", mask, got, err)
	}
	if err := doc.Save(filepath.Join(root, "saved.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := doc.ChangeHistory(); err != nil {
		t.Fatal(err)
	}
	if got, err := consumer.LineMask(0, 3, 3); err != nil || got == 0 || got == mask {
		t.Fatalf("successful Save did not transition modified editions: %d -> %d, %v", mask, got, err)
	}
}

func TestDocumentChangeHistoryGapFailsClosedWithoutBlockingEdits(t *testing.T) {
	doc := New("gap")
	for range 300 {
		doc.Editor.SetSavePoint()
	}
	if consumer, err := doc.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("expired history appeared available: %v, %v", consumer, err)
	}
	doc.Editor.Insert("still editable")
	if consumer, err := doc.ChangeHistory(); err == nil || consumer != nil || doc.Editor.String() != "still editable" {
		t.Fatalf("gap was re-bootstrapped or blocked editing: %v, %v", consumer, err)
	}
}

func TestDocumentRecoveryEditKeepsHistoryUnsaved(t *testing.T) {
	doc := New("recovered")
	doc.Editor.Insert("unsaved backup")
	doc.MarkRecovered()
	consumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Dirty() {
		t.Fatal("recovered edit appeared saved")
	}
	if mask, err := consumer.LineMask(0, 0, len(doc.Editor.String())); err != nil || mask == 0 {
		t.Fatalf("recovery edit has no history marker: %d, %v", mask, err)
	}
	if err := doc.SaveCopy(filepath.Join(t.TempDir(), "backup.txt")); err != nil {
		t.Fatal(err)
	}
	if !doc.Dirty() {
		t.Fatal("recovery Save Copy cleared dirty state")
	}
}
