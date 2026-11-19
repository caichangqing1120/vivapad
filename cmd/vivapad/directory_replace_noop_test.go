package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestCompactDirectoryReplaceDoesNotConfirmByteIdenticalOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.txt")
	const original = "\xef\xbb\xbfcat\r\ncat\ncat\r"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.MoveTo(4, false)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(2))
	revealSearchMenuItem(t, driver, searchCommandIndex(t, "Replace in Files..."))
	clickID(t, driver, searchCommandID(t, "Replace in Files..."))
	for _, field := range []gfx.ID{queryID, replacementID} {
		clickID(t, driver, field)
		if err := driver.Type("cat"); err != nil {
			t.Fatal(err)
		}
	}
	clickID(t, driver, replacePreviewID)
	for deadline := time.Now().Add(5 * time.Second); state.filesSearching && time.Now().Before(deadline); {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	const message = "No changes to preview"
	if state.filesSearching || !state.filesDialogOpen || state.replaceConfirmOpen || state.replacePlan != nil || state.filesMessage != message {
		t.Fatalf("unexpected no-op preview: searching=%t confirm=%t message=%q plan=%+v",
			state.filesSearching, state.replaceConfirmOpen, state.filesMessage, state.replacePlan)
	}
	if state.space.Current() != doc || doc.Dirty() || doc.Editor.CanUndo() || doc.Editor.Caret() != 4 {
		t.Fatal("no-op preview changed the open document or its history")
	}
	if disk, err := os.ReadFile(path); err != nil || string(disk) != original {
		t.Fatalf("no-op preview changed disk bytes: %x, %v", disk, err)
	}
	for _, label := range driver.Snapshot().Texts {
		if label.Value == message {
			return
		}
	}
	t.Fatal("compact dialog did not display the full no-op message")
}
