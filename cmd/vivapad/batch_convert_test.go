package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/batchconvert"
	"vivapad/internal/document"
)

func openBatchConvertMenu(t *testing.T, state *model, driver *app.Driver, label string) {
	t.Helper()
	clickID(t, driver, menuScope.At(4))
	id := menuItemScope.At(slices.Index(menuCommands[4], batchConvertCommand))
	if hit := batchConvertHit(t, driver, id); hit.Label != label {
		t.Fatalf("menu label: %q, want %q", hit.Label, label)
	}
	clickID(t, driver, id)
	if state.batchConvert == nil || !batchConvertHit(t, driver, batchApplyID).Disabled {
		t.Fatal("batch dialog must open with Apply disabled")
	}
}

func batchConvertHit(t *testing.T, driver *app.Driver, id gfx.ID) gfx.Hit {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	t.Fatalf("missing hit %v", id)
	return gfx.Hit{}
}

func waitBatchConvert(t *testing.T, state *model, driver *app.Driver) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for state.batchConvert.job != nil {
		select {
		case <-driver.Wakes():
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("batch job did not finish")
		}
	}
}

func TestBatchConvertOpenTabsDialogAndUndo(t *testing.T) {
	state, driver := startDriver(t)
	originalThreshold := document.LargeFileThreshold
	document.LargeFileThreshold = 32
	t.Cleanup(func() { document.LargeFileThreshold = originalThreshold })
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	if err := os.WriteFile(path, []byte("A\nB\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	first := state.space.Current()
	first.Editor.Insert("first\n")
	state.openDocumentPaths([]string{path})
	second := state.space.Current()
	largePath := filepath.Join(root, "large.txt")
	if err := os.WriteFile(largePath, []byte(strings.Repeat("large file\n", 32)), 0644); err != nil {
		t.Fatal(err)
	}
	if !state.openDocumentPaths([]string{largePath}) || state.space.Current().LargeFile() == nil {
		t.Fatalf("could not open large viewer: %s", state.notice)
	}
	large := state.space.Current()
	readonly := state.space.NewDocument()
	state.readOnly[readonly] = true
	state.space.Select(state.documentIndex(second))
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	openBatchConvertMenu(t, state, driver, "Batch Convert...")
	clickID(t, driver, batchEncodingChoices.At(2))
	clickID(t, driver, batchEOLChoices.At(int(batchconvert.CR)))
	clickID(t, driver, batchPreviewID)
	if second.Dirty() || second.Encoding() != document.UTF8 || len(state.batchConvert.entries) != 4 || batchConvertHit(t, driver, batchApplyID).Disabled {
		t.Fatal("preview changed a tab or did not enable Apply")
	}
	clickID(t, driver, batchApplyID)
	result := state.batchConvert.result
	if result == nil || result.Converted != 2 || result.Skipped != 2 || first.LineEnding() != document.CR || second.Encoding() != document.UTF8BOM || !second.Dirty() || large.Dirty() {
		t.Fatalf("unexpected conversion result: %+v", result)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "A\nB\r\n" {
		t.Fatalf("Open Tabs wrote disk: %q, %v", data, err)
	}
	clickID(t, driver, batchCloseID)
	if _, err := driver.ShortcutChord(primary(), 'z'); err != nil {
		t.Fatal(err)
	}
	if second.Dirty() || second.EOLSequence() != "LW" || second.Encoding() != document.UTF8 || second.Editor.String() != "A\nB\n" {
		t.Fatalf("keyboard undo did not restore format: %+v", second.FileFormat())
	}
	state.command("Redo")
	if second.LineEnding() != document.CR || !second.Dirty() {
		t.Fatal("menu redo failed")
	}
}

func TestBatchConvertRejectsStaleTabPreview(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	openBatchConvertMenu(t, state, driver, "Batch Convert...")
	clickID(t, driver, batchEncodingChoices.At(2))
	clickID(t, driver, batchPreviewID)
	doc.Editor.Insert("updated externally")
	clickID(t, driver, batchApplyID)
	result := state.batchConvert.result
	if result == nil || result.Failed != 1 || len(result.Errors) != 1 || doc.Encoding() != document.UTF8 || doc.Editor.String() != "updated externally" {
		t.Fatalf("stale preview: %+v", result)
	}
}

func TestBatchConvertDirectoryDialogRequiresFreshPreview(t *testing.T) {
	state, driver := startDriver(t)
	state.chineseUI = true
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0644); err != nil {
		t.Fatal(err)
	}
	openBatchConvertMenu(t, state, driver, "批量转换…")
	clickID(t, driver, batchScopeChoices.At(1))
	if err := driver.QueueOpenDirectory(root, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, batchBrowseID)
	clickID(t, driver, batchEOLChoices.At(int(batchconvert.CRLF)))
	clickID(t, driver, batchPreviewID)
	waitBatchConvert(t, state, driver)
	if data, err := os.ReadFile(path); err != nil || string(data) != "first\nsecond\n" || batchConvertHit(t, driver, batchApplyID).Disabled {
		t.Fatalf("preview wrote disk or did not enable Apply: %q, %v", data, err)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact directory dialog layout: %+v", problems)
	}
	clickID(t, driver, batchGlobID)
	if err := driver.Type("*.txt"); err != nil {
		t.Fatal(err)
	}
	if !batchConvertHit(t, driver, batchApplyID).Disabled {
		t.Fatal("changing a field left an earlier preview applicable")
	}
	clickID(t, driver, batchPreviewID)
	waitBatchConvert(t, state, driver)
	clickID(t, driver, batchApplyID)
	waitBatchConvert(t, state, driver)
	if result := state.batchConvert.result; result == nil || result.Converted != 1 || !batchConvertHit(t, driver, batchApplyID).Disabled {
		t.Fatalf("apply: %+v", result)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "first\r\nsecond\r\n" {
		t.Fatalf("apply output: %q, %v", data, err)
	}
}
