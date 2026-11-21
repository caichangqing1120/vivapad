package workspace

// H03 workspace lifecycle tests: a reload or adoption replaces the document (and
// its editor) with a new bound change-history source; the retired source's
// history never transfers to the replacement.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func lineMask(t *testing.T, doc *document.Document, line int) uint8 {
	t.Helper()
	consumer, err := doc.ChangeHistory()
	if err != nil || consumer == nil {
		t.Fatalf("%s ChangeHistory = %p, %v", doc.Name, consumer, err)
	}
	value := doc.Editor.String()
	start := 0
	for index, content := range strings.SplitAfter(value, "\n") {
		next := start + len(content)
		end := next
		if strings.HasSuffix(content, "\n") {
			end--
		}
		if index == line {
			mask, err := consumer.LineMask(start, end, next)
			if err != nil {
				t.Fatalf("%s LineMask line %d: %v", doc.Name, line, err)
			}
			return mask
		}
		start = next
	}
	t.Fatalf("%s line %d is out of range for %q", doc.Name, line, value)
	return 0
}

// TestReloadBindsNewChangeHistorySource pins Workspace.Reload, the external
// reload path used by the Reload command, directory replace and file monitors:
// the tab gets a distinct bound consumer, the old history does not transfer and
// the retired document keeps its own source.
func TestReloadBindsNewChangeHistorySource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.Open(path); err != nil {
		t.Fatal(err)
	}
	old := space.Current()
	oldConsumer, err := old.ChangeHistory()
	if err != nil || oldConsumer == nil {
		t.Fatalf("opened document ChangeHistory = %p, %v", oldConsumer, err)
	}
	old.Editor.MoveTo(old.Editor.RuneCount(), false)
	old.Editor.Insert(" local")
	if mask := lineMask(t, old, 0); mask != 4 {
		t.Fatalf("edit before reload mask = %d, want 4 (modified)", mask)
	}

	// A declined reload of a dirty tab keeps the existing source untouched.
	if _, err := space.Reload(0); !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty reload = %v, want ErrDirty", err)
	}
	if mask := lineMask(t, old, 0); mask != 4 {
		t.Fatalf("declined reload changed the source mask to %d, want 4", mask)
	}

	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	retired, err := space.ReloadDiscard(0)
	if err != nil || retired != old {
		t.Fatalf("reload = retained %p, %v", retired, err)
	}
	fresh := space.Current()
	if fresh == old {
		t.Fatal("reload did not replace the document")
	}
	freshConsumer, err := fresh.ChangeHistory()
	if err != nil || freshConsumer == nil {
		t.Fatalf("reloaded document ChangeHistory = %p, %v", freshConsumer, err)
	}
	if freshConsumer == oldConsumer {
		t.Fatal("reload transferred the retired change-history source")
	}
	if mask := lineMask(t, fresh, 0); mask != 0 {
		t.Fatalf("retired history appeared on the reloaded document: mask = %d, want 0", mask)
	}
	if fresh.Editor.CanUndo() {
		t.Fatal("reloaded document started with the retired Undo history")
	}
	if mask := lineMask(t, old, 0); mask != 4 {
		t.Fatalf("retired document lost its own history: mask = %d, want 4", mask)
	}
}

// TestAdoptedDocumentKeepsItsBoundSource pins AddOpened: a prevalidated
// document keeps the single consumer bound when it was opened, and adopting it
// does not create a second source.
func TestAdoptedDocumentKeepsItsBoundSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adopted.txt")
	if err := os.WriteFile(path, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.AddOpened(prepared); err != nil {
		t.Fatal(err)
	}
	if space.Current() != prepared {
		t.Fatal("AddOpened did not adopt the prepared document")
	}
	first, err := prepared.ChangeHistory()
	if err != nil || first == nil {
		t.Fatalf("adopted document ChangeHistory = %p, %v", first, err)
	}
	second, err := space.Current().ChangeHistory()
	if err != nil || second != first {
		t.Fatalf("adoption created a second consumer: %p, %v", second, err)
	}
	if mask := lineMask(t, prepared, 0); mask != 0 {
		t.Fatalf("freshly adopted document mask = %d, want 0", mask)
	}
}
