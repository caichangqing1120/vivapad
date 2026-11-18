package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
)

func TestCompactDirectoryReplaceShowsUnmappableEncodingCause(t *testing.T) {
	temp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(temp, strings.Repeat("long-directory-", 10))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "legacy.txt")
	original := "cat caf\xe9"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
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
	state.filesRoot = text.NewEditor(root)
	clickID(t, driver, queryID)
	if err := driver.Type("cat"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replacementID)
	if err := driver.Type("\ue000"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, filesWindowsID)
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
	const cause = "cannot save text as Windows-1252"
	if state.filesSearching || !state.filesDialogOpen || state.replaceConfirmOpen || state.replacePlan != nil ||
		!strings.Contains(state.filesMessage, cause) || !strings.Contains(state.filesMessage, path) {
		t.Fatalf("unexpected preview state: searching=%v confirm=%v error=%q", state.filesSearching, state.replaceConfirmOpen, state.filesMessage)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("refused preview changed the file: %x, %v", data, err)
	}
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, cause) {
			return
		}
	}
	t.Fatalf("compact preview hid the encoding refusal: %q", state.filesMessage)
}
