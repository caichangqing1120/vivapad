package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestChangeHistoryReloadUsesNewSourceWithoutLosingDirtyDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.Open(path); err != nil {
		t.Fatal(err)
	}
	old := space.Current()
	oldConsumer, err := old.ChangeHistory()
	if err != nil {
		t.Fatal(err)
	}
	old.Editor.Insert("local")
	if _, err := space.Reload(space.Active); !errors.Is(err, ErrDirty) || space.Current() != old {
		t.Fatalf("dirty reload replaced the document: %v", err)
	}
	if current, err := old.ChangeHistory(); err != nil || current != oldConsumer {
		t.Fatalf("refused reload lost the original source: %v", err)
	}
	if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	if previous, err := space.ReloadDiscard(space.Active); err != nil || previous != old {
		t.Fatalf("discard reload did not return the prior document: %v", err)
	}
	updated := space.Current()
	if updated == old || updated.Editor.String() != "external" {
		t.Fatalf("reload reused the old document or bytes: %q", updated.Editor.String())
	}
	fresh, err := updated.ChangeHistory()
	if err != nil || fresh == oldConsumer {
		t.Fatalf("reload reused an old history source: %v", err)
	}
	if current, err := old.ChangeHistory(); err != nil || current != oldConsumer {
		t.Fatalf("discarded document's source was mutated: %v", err)
	}
	updated.Editor.Insert("new")
	if current, err := updated.ChangeHistory(); err != nil || current != fresh {
		t.Fatalf("reload did not track its first edit: %v", err)
	}
}
