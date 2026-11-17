package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestLegacyMenuSaveRejectsUnrepresentableUnicode(t *testing.T) {
	for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
		for _, action := range []string{"Save", "Save As...", "Save a Copy As..."} {
			t.Run(action, func(t *testing.T) {
				root := t.TempDir()
				path, target := filepath.Join(root, "source.txt"), filepath.Join(root, "target.txt")
				original := []byte("A\uE000\r\n")
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
				state := newModelWithFiles([]string{path})
				driver, err := app.NewDriver(newConfig(state), nil)
				if err != nil {
					t.Fatal(err)
				}
				defer driver.Close()
				if err := driver.Resize(size.width, size.height); err != nil {
					t.Fatal(err)
				}
				doc := state.space.Current()
				doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
				doc.Editor.Insert("!")
				doc.Editor.SetSelection(text.Range{Start: 2, End: 0})
				selection := doc.Editor.Selection()
				clickID(t, driver, menuScope.At(4))
				clickID(t, driver, menuItemScope.At(7))
				if doc.Encoding() != document.Windows1252 {
					t.Fatal("Encoding menu did not select Windows-1252")
				}
				if action != "Save" {
					if err := driver.QueueSaveFile(target, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					index := 3
					if action == "Save a Copy As..." {
						index = 4
					}
					clickID(t, driver, menuItemScope.At(index))
				} else {
					clickID(t, driver, saveID)
				}
				if !strings.Contains(state.notice, "Windows-1252") || doc.Path == target || !doc.Dirty() || doc.Editor.Selection() != selection || doc.Editor.String() != "A\uE000\n!" {
					t.Fatalf("lossy %s changed editor or hid error: notice=%q", action, state.notice)
				}
				if disk, err := os.ReadFile(path); err != nil || !bytes.Equal(disk, original) {
					t.Fatalf("source bytes changed: %x, %v", disk, err)
				}
				if disk, err := os.ReadFile(target); err != nil || string(disk) != "existing" {
					t.Fatalf("destination bytes changed: %x, %v", disk, err)
				}
			})
		}
	}
}
