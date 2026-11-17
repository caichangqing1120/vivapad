package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
)

func TestMixedEOLDriverSaveUndoKeepsPhysicalSeparators(t *testing.T) {
	for _, action := range []string{"Save", "Save As...", "Save a Copy As..."} {
		for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
			t.Run(fmt.Sprintf("%s/%dx%d", action, size.width, size.height), func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "mixed.txt")
				original := []byte("a\r\nb\nc\rd")
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				state := newModelWithFiles([]string{path})
				driver, err := app.NewDriver(newConfig(state), nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(driver.Close)
				if err := driver.Resize(size.width, size.height); err != nil {
					t.Fatal(err)
				}
				doc := state.space.Current()
				doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
				destination := path
				if action == "Save" {
					clickID(t, driver, saveID)
				} else {
					destination = filepath.Join(root, "saved.txt")
					if err := driver.QueueSaveFile(destination, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0][:], action)))
				}
				if disk, err := os.ReadFile(destination); err != nil || string(disk) != "a\r\nbc\rd" {
					t.Fatalf("%s bytes = %q, %v", action, disk, err)
				}
				if !doc.Editor.Undo() {
					t.Fatal("Undo failed after Driver save")
				}
				copyPath := filepath.Join(root, "undone.txt")
				if err := doc.SaveCopy(copyPath); err != nil {
					t.Fatal(err)
				}
				if disk, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(disk, original) {
					t.Fatalf("Driver %s/Undo lost physical separators: %q, %v", action, disk, err)
				}
			})
		}
	}
}
