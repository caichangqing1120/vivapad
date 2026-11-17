package document

// H03 document-lifecycle tests for the change-history consumer binding.
//
// Contract: docs/specs/001-vivapad/h03-lifecycle-contract.md.
//
// The line-mask values are the four low edition bits produced by
// changehistory.Tracker.LineMask: mask 2 is the saved edition (edition 2) and
// mask 4 the modified edition (edition 3), as derived from the source
// predicates and pinned by the H02 real-producer oracle.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

const (
	changeHistorySavedMask    uint8 = 2 // edition 2: saved
	changeHistoryModifiedMask uint8 = 4 // edition 3: modified
)

// changeLineMask returns the change-history line mask for one normalized-LF
// line. It consumes the document's single bound history first; an unavailable
// history is a test failure, never an empty mask.
func changeLineMask(t *testing.T, doc *Document, line int) uint8 {
	t.Helper()
	consumer, err := doc.ChangeHistory()
	if err != nil {
		t.Fatalf("ChangeHistory: %v", err)
	}
	if consumer == nil {
		t.Fatal("ChangeHistory returned no consumer and no error")
	}
	start := 0
	for index, content := range strings.SplitAfter(doc.Editor.String(), "\n") {
		next := start + len(content)
		end := next
		if strings.HasSuffix(content, "\n") {
			end--
		}
		if index == line {
			mask, err := consumer.LineMask(start, end, next)
			if err != nil {
				t.Fatalf("LineMask line %d: %v", line, err)
			}
			return mask
		}
		start = next
	}
	t.Fatalf("line %d is out of range for %q", line, doc.Editor.String())
	return 0
}

// TestChangeHistorySaveFailureDoesNotSaveEditions pins the save boundary: only
// a successful Save/Save As emits the editor save point as a Save frame.
// A failed Save, a failed Save As and Save Copy must leave the editions
// modified (never saved) and must not touch the disk or Dirty.
func TestChangeHistorySaveFailureDoesNotSaveEditions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "save-failure.txt")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	openedPath := doc.Path
	if mask := changeLineMask(t, doc, 0); mask != 0 {
		t.Fatalf("freshly opened document mask = %d, want 0", mask)
	}

	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" plus")
	if mask := changeLineMask(t, doc, 0); mask != changeHistoryModifiedMask {
		t.Fatalf("unsaved edit mask = %d, want %d (modified)", mask, changeHistoryModifiedMask)
	}
	if !doc.Dirty() {
		t.Fatal("edited document is not dirty")
	}

	// A concurrent external write makes Save refuse before it can write.
	if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(""); err == nil {
		t.Fatal("Save succeeded against a file changed on disk")
	} else if !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("failed Save error = %v, want the on-disk change refusal", err)
	}
	if mask := changeLineMask(t, doc, 0); mask != changeHistoryModifiedMask {
		t.Fatalf("failed Save emitted the editor save point: mask = %d, want %d (modified)", mask, changeHistoryModifiedMask)
	}
	if !doc.Dirty() {
		t.Fatal("failed Save cleaned the dirty state")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "external" {
		t.Fatalf("failed Save touched the disk: %q, %v", data, err)
	}

	// A failed Save As (destination is a directory) emits nothing either.
	if err := doc.Save(root); err == nil {
		t.Fatal("Save As accepted a directory destination")
	}
	if mask := changeLineMask(t, doc, 0); mask != changeHistoryModifiedMask {
		t.Fatalf("failed Save As emitted the editor save point: mask = %d, want %d (modified)", mask, changeHistoryModifiedMask)
	}

	// Save Copy writes a copy but is not a save of this document.
	copyPath := filepath.Join(root, "copy.txt")
	if err := doc.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if mask := changeLineMask(t, doc, 0); mask != changeHistoryModifiedMask {
		t.Fatalf("Save Copy emitted the editor save point: mask = %d, want %d (modified)", mask, changeHistoryModifiedMask)
	}
	if !doc.Dirty() || doc.Path != openedPath {
		t.Fatalf("Save Copy changed the document save state: dirty=%v path=%q", doc.Dirty(), doc.Path)
	}
	if data, err := os.ReadFile(copyPath); err != nil || string(data) != "base plus" {
		t.Fatalf("Save Copy content = %q, %v", data, err)
	}

	// A successful Save As does publish the save point.
	saveAs := filepath.Join(root, "saved-as.txt")
	if err := doc.Save(saveAs); err != nil {
		t.Fatal(err)
	}
	if mask := changeLineMask(t, doc, 0); mask != changeHistorySavedMask {
		t.Fatalf("successful Save As did not publish the save point: mask = %d, want %d (saved)", mask, changeHistorySavedMask)
	}
	if doc.Dirty() {
		t.Fatal("successful Save As left the document dirty")
	}
	if mask := changeLineMask(t, doc, 0); mask != changeHistorySavedMask {
		t.Fatalf("repeated consumption changed the saved mask: %d", mask)
	}
}

// TestChangeHistoryCloneSharesSource pins split-view clones: both panes
// reference the same document and therefore the same consumer. A second view
// never bootstraps a second source, and a distinct document never shares one.
func TestChangeHistoryCloneSharesSource(t *testing.T) {
	doc := New("new 1")
	left, right := doc, doc // Clone to Other View shares one *Document
	first, err := left.ChangeHistory()
	if err != nil || first == nil {
		t.Fatalf("left pane ChangeHistory = %p, %v", first, err)
	}
	second, err := right.ChangeHistory()
	if err != nil || second == nil {
		t.Fatalf("right pane ChangeHistory = %p, %v", second, err)
	}
	if first != second {
		t.Fatal("split clone created a second change-history consumer")
	}
	left.Editor.Insert("shared")
	if mask := changeLineMask(t, right, 0); mask != changeHistoryModifiedMask {
		t.Fatalf("edit through the shared clone mask = %d, want %d", mask, changeHistoryModifiedMask)
	}
	third, err := right.ChangeHistory()
	if err != nil || third != first {
		t.Fatalf("second pane re-bootstrapped the source: %p, %v", third, err)
	}

	other := New("new 2")
	otherConsumer, err := other.ChangeHistory()
	if err != nil || otherConsumer == nil {
		t.Fatalf("second document ChangeHistory = %p, %v", otherConsumer, err)
	}
	if otherConsumer == first {
		t.Fatal("distinct documents share one change-history consumer")
	}
	if mask := changeLineMask(t, other, 0); mask != 0 {
		t.Fatalf("history leaked between documents: mask = %d, want 0", mask)
	}
}

// TestChangeHistoryRecoveryAndReload pins recovery and reload lifecycle:
// recovered text is formed by real restore edits (modified, never an implicit
// save), and an external reload binds a new source without transferring the old
// history.
func TestChangeHistoryRecoveryAndReload(t *testing.T) {
	t.Run("recovery uses real restore edits", func(t *testing.T) {
		doc := New("Recovered notes.txt")
		// This is exactly what cmd/vivapad/recovery.go does: replace the whole
		// buffer through the editor, then mark the document recovered. No
		// SetSavePoint may be involved.
		doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
		doc.Editor.Insert("recovered unsaved text")
		doc.MarkRecovered()
		if mask := changeLineMask(t, doc, 0); mask != changeHistoryModifiedMask {
			t.Fatalf("recovery marked restored text as saved: mask = %d, want %d (modified)", mask, changeHistoryModifiedMask)
		}
		if !doc.Dirty() {
			t.Fatal("recovered document is not dirty")
		}
		path := filepath.Join(t.TempDir(), "recovered.txt")
		if err := doc.Save(path); err != nil {
			t.Fatal(err)
		}
		if mask := changeLineMask(t, doc, 0); mask != changeHistorySavedMask {
			t.Fatalf("saving a recovered document did not publish the save point: mask = %d, want %d (saved)", mask, changeHistorySavedMask)
		}
		if doc.Dirty() {
			t.Fatal("saved recovered document is still dirty")
		}
	})

	t.Run("reload binds a new source", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reload.txt")
		if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
			t.Fatal(err)
		}
		original, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		originalConsumer, err := original.ChangeHistory()
		if err != nil || originalConsumer == nil {
			t.Fatalf("original ChangeHistory = %p, %v", originalConsumer, err)
		}
		original.Editor.MoveTo(original.Editor.RuneCount(), false)
		original.Editor.Insert(" local")
		if mask := changeLineMask(t, original, 0); mask != changeHistoryModifiedMask {
			t.Fatalf("edit before reload mask = %d, want %d", mask, changeHistoryModifiedMask)
		}

		if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
			t.Fatal(err)
		}
		reloaded, err := Open(path) // what workspace.Reload and directory replace use
		if err != nil {
			t.Fatal(err)
		}
		reloadedConsumer, err := reloaded.ChangeHistory()
		if err != nil || reloadedConsumer == nil {
			t.Fatalf("reloaded ChangeHistory = %p, %v", reloadedConsumer, err)
		}
		if reloadedConsumer == originalConsumer {
			t.Fatal("reload reused the retired source")
		}
		if mask := changeLineMask(t, reloaded, 0); mask != 0 {
			t.Fatalf("old source's history transferred to the reloaded document: mask = %d, want 0", mask)
		}
		if reloaded.Editor.CanUndo() {
			t.Fatal("reloaded document started with the old source's Undo history")
		}
		if mask := changeLineMask(t, original, 0); mask != changeHistoryModifiedMask {
			t.Fatalf("reload disturbed the retired source: mask = %d, want %d", mask, changeHistoryModifiedMask)
		}
	})
}

// TestChangeHistoryReplacedEditorNeverBecomesSource covers an editor swap on an
// existing document: the old consumer is retired, and not even an equal-length
// replacement is accepted as a silent continuation of the old source.
func TestChangeHistoryReplacedEditorNeverBecomesSource(t *testing.T) {
	doc := New("swap")
	original, err := doc.ChangeHistory()
	if err != nil || original == nil {
		t.Fatalf("initial ChangeHistory = %p, %v", original, err)
	}
	// Empty-to-empty keeps the byte length identical, so only source identity can
	// detect that this is not the bound editor.
	doc.Editor = text.NewEditor("")
	consumer, err := doc.ChangeHistory()
	if err == nil {
		t.Fatalf("replaced editor leaked the old source: consumer %p", consumer)
	}
	if consumer != nil {
		t.Fatalf("replaced editor returned consumer %p with error %v", consumer, err)
	}
	if !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("unavailable error does not describe the replaced source: %v", err)
	}
	doc.Editor.Insert("new source text")
	if consumer, err := doc.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("edited replacement became an available history: %p, %v", consumer, err)
	}
}

// TestChangeHistoryUnavailableConstructionIsPersistent pins the explicit
// unavailable state: a producer that cannot be enabled at construction (a
// restricted/private transaction editor) is a persistent error, never an empty
// available history, and an explicit draft document has no source at all.
func TestChangeHistoryUnavailableConstructionIsPersistent(t *testing.T) {
	source := text.NewEditor("draft")
	var draft *Document
	if err := source.Transact(func(editor *text.Editor) error {
		// The transaction draft editor has a private journal, so the history
		// producer must refuse it.
		draft = newDocument("draft", editor)
		draft.bindChangeHistory()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	consumer, err := draft.ChangeHistory()
	if err == nil || consumer != nil {
		t.Fatalf("unavailable construction returned consumer %p and error %v", consumer, err)
	}
	first := err.Error()
	if !strings.Contains(first, "change history unavailable") {
		t.Fatalf("unavailable construction error = %q, want an explicit change-history cause", first)
	}
	consumer, err = draft.ChangeHistory()
	if err == nil || consumer != nil || err.Error() != first {
		t.Fatalf("unavailable construction is not persistent: %p, %v (first %q)", consumer, err, first)
	}

	blank := &Document{}
	if consumer, err := blank.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("zero document reported an available history: %p, %v", consumer, err)
	}
	explicitDraft := NewDraft("macro", text.NewEditor("x"))
	if consumer, err := explicitDraft.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("explicit draft reported an available history: %p, %v", consumer, err)
	}
}

// TestChangeHistoryLateBindingIsRefused pins that the producer cannot be turned
// on after undoable edits: the consumer must be bound at creation, not when the
// first query appears.
func TestChangeHistoryLateBindingIsRefused(t *testing.T) {
	editor := text.NewEditor("already")
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert(" edited")
	late := newDocument("late", editor)
	late.bindChangeHistory()
	if consumer, err := late.ChangeHistory(); err == nil || consumer != nil {
		t.Fatalf("producer was enabled after undo history existed: %p, %v", consumer, err)
	}
}

// TestChangeHistoryGapIsExplicitUnavailable pins the 256-frame producer ring:
// an unconsumed gap must surface as an explicit persistent unavailable history,
// never as a silently shorter or empty mask.
func TestChangeHistoryGapIsExplicitUnavailable(t *testing.T) {
	doc := New("gap")
	for range 300 {
		doc.Editor.Insert("x") // each keystroke is one producer frame
	}
	if got := doc.Editor.String(); len(got) != 300 {
		t.Fatalf("document text length = %d, want 300", len(got))
	}
	consumer, err := doc.ChangeHistory()
	if err == nil || consumer != nil {
		t.Fatalf("256-frame producer gap was served as a complete history: %p, %v", consumer, err)
	}
	reason := err.Error()
	if !strings.Contains(reason, "unavailable") {
		t.Fatalf("gap error = %q, want an explicit unavailable history", reason)
	}
	consumer, err = doc.ChangeHistory()
	if err == nil || consumer != nil || err.Error() != reason {
		t.Fatalf("gap is not a persistent explicit unavailable history: %p, %v", consumer, err)
	}
	// The text editor is unaffected: only the optional history is unavailable.
	doc.Editor.Insert("y")
	if got := doc.Editor.String(); len(got) != 301 {
		t.Fatalf("document did not remain editable after the history gap: %d bytes", len(got))
	}
}
