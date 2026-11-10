package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/hashops"
)

func TestHashToolsMenuCoversFourAlgorithmsAndSelectionClipboard(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("abc"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.ClearHistory()
	state.command("Read-Only on Current Document")
	wasDirty := doc.Dirty()
	for index, test := range []struct {
		algorithm hashops.Algorithm
		want      string
	}{
		{hashops.MD5, "900150983cd24fb0d6963f7d28e17f72"},
		{hashops.SHA1, "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{hashops.SHA256, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{hashops.SHA512, "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
			"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	} {
		doc.Editor.SetSelection(text.Range{Start: 0, End: 3})
		clickID(t, driver, menuScope.At(7))
		clickID(t, driver, menuItemScope.At(index))
		if state.hashAlgorithm != test.algorithm || !state.hashMenuOpen {
			t.Fatalf("Tools algorithm %d did not open", index)
		}
		parentY, childY := float32(-1), float32(-1)
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				parentY = hit.Bounds.Y
			}
			if hit.ID == hashScope.At(0) {
				childY = hit.Bounds.Y
			}
		}
		if parentY < 0 || childY < 0 || childY-parentY > 10 || parentY-childY > 10 {
			t.Fatalf("hash submenu is detached from parent row %d: parent=%g child=%g", index, parentY, childY)
		}
		for subIndex := range hashCommands {
			found := false
			for _, hit := range driver.Snapshot().Hits {
				if hit.ID == hashScope.At(subIndex) {
					found = true
					if hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
						t.Fatalf("hash submenu row outside compact viewport: %+v", hit.Bounds)
					}
				}
			}
			if !found {
				t.Fatalf("hash submenu row %d missing", subIndex)
			}
		}
		clickID(t, driver, hashScope.At(2))
		if got := driver.Clipboard(); got != test.want || doc.Editor.String() != "abc" || doc.Dirty() != wasDirty || doc.Editor.CanUndo() {
			t.Fatalf("algorithm %d clipboard=%q text=%q dirty=%v", index, got, doc.Editor.String(), doc.Dirty())
		}
	}
	doc.Editor.MoveTo(0, false)
	driver.SetClipboard("unchanged")
	state.command("Generate from selection into clipboard")
	if driver.Clipboard() != "unchanged" {
		t.Fatal("empty selection overwrote clipboard")
	}
}

func TestHashTextDialogUpdatesPerLineAndCopiesWithoutEditing(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(7))
	clickID(t, driver, menuItemScope.At(0))
	clickID(t, driver, hashScope.At(0))
	if !state.hashDialogOpen || state.hashMode != hashFromText {
		t.Fatal("Generate text dialog did not open")
	}
	clickID(t, driver, hashInputID)
	if err := driver.Type("a\n\nb"); err != nil {
		t.Fatal(err)
	}
	if state.hashResult != "f3085cccd508e131c268385174aee259" {
		t.Fatalf("unexpected text digest = %q", state.hashResult)
	}
	clickID(t, driver, hashEachLineID)
	if got := state.hashResult; !strings.Contains(got, "\r\n\r\n") || strings.Count(got, "\r\n") != 3 {
		t.Fatalf("per-line hash result = %q", got)
	}
	clickID(t, driver, hashCopyID)
	if driver.Clipboard() != state.hashResult || state.space.Current().Editor.String() != "" {
		t.Fatalf("copy hash result = %q editor=%q", driver.Clipboard(), state.space.Current().Editor.String())
	}
	if err := driver.Key(ui.KeyEscape, 0); err != nil || state.hashDialogOpen {
		t.Fatalf("Escape did not close text hash dialog: %v", err)
	}
}

func TestHashFilesDialogCancelAndBinaryReport(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "binary.dat")
	if err := os.WriteFile(path, []byte{0, 0xff, 0x0a}, 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	clickID(t, driver, menuScope.At(7))
	clickID(t, driver, menuItemScope.At(2))
	clickID(t, driver, hashScope.At(1))
	if !state.hashDialogOpen || state.hashMode != hashFromFiles {
		t.Fatal("file hash dialog did not open")
	}
	if err := driver.QueueOpenFiles(nil, app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, hashBrowseID)
	if state.hashResult != "" || !state.hashDialogOpen {
		t.Fatal("canceled picker changed the hash report")
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, hashBrowseID)
	if !strings.HasSuffix(state.hashResult, "  binary.dat\r\n") || len(state.hashResult) != 64+2+len("binary.dat")+2 {
		t.Fatalf("file hash report = %q", state.hashResult)
	}
	clickID(t, driver, hashCopyID)
	if driver.Clipboard() != state.hashResult {
		t.Fatal("file hash copy missed the report")
	}
}

func TestSelectionHashUsesActiveSplitPaneOnly(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("abc")
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 0, End: 3})
	state.hashAlgorithm = hashops.SHA256
	state.command("Generate from selection into clipboard")
	if state.activePane != 1 || driver.Clipboard() != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" ||
		left.Editor.String() != "left" || right.Editor.String() != "abc" {
		t.Fatalf("split hash pane=%d clipboard=%q left=%q right=%q", state.activePane, driver.Clipboard(), left.Editor.String(), right.Editor.String())
	}
}

func TestHashFileResultsScrollWithoutMovingDocument(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 12)
	for index := range paths {
		paths[index] = filepath.Join(root, fmt.Sprintf("file%02d.txt", index))
		if err := os.WriteFile(paths[index], []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(7))
	clickID(t, driver, menuItemScope.At(2))
	clickID(t, driver, hashScope.At(1))
	if err := driver.QueueOpenFiles(paths, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, hashBrowseID)
	visible := func() string {
		var result strings.Builder
		for _, label := range driver.Snapshot().Labels {
			result.WriteString(label.Value)
		}
		return result.String()
	}
	before := visible()
	if err := driver.Scroll(ui.ScrollEvent{X: 320, Y: 190, DeltaY: 240}); err != nil {
		t.Fatal(err)
	}
	if err := driver.Scroll(ui.ScrollEvent{X: 320, Y: 190, DeltaY: 240}); err != nil {
		t.Fatal(err)
	}
	after := visible()
	if before == after || !strings.Contains(after, "file11.txt") || state.space.Current().Editor.String() != "" {
		t.Fatalf("result pane did not scroll: before=%q after=%q", before, after)
	}
}
