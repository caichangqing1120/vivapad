package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestGB18030PUAMenuSavePaths(t *testing.T) {
	for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
		for _, action := range []string{"Save", "Save As...", "Save a Copy As..."} {
			t.Run(fmt.Sprintf("%dx%d/%s", size.width, size.height, action), func(t *testing.T) {
				root := t.TempDir()
				source, target := filepath.Join(root, "source.txt"), filepath.Join(root, "target.txt")
				original := []byte("A\uE000\uE5E5\r\n")
				if err := os.WriteFile(source, original, 0600); err != nil {
					t.Fatal(err)
				}
				state := newModelWithFiles([]string{source})
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
				doc.Editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
				clickID(t, driver, menuScope.At(4))
				clickID(t, driver, menuItemScope.At(6))
				if doc.Encoding() != document.GB18030 {
					t.Fatal("Encoding menu did not select GB18030")
				}
				if action == "Save" {
					target = source
					clickID(t, driver, saveID)
				} else {
					if err := driver.QueueSaveFile(target, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					index := 3
					if action == "Save a Copy As..." {
						index = 4
					}
					clickID(t, driver, menuItemScope.At(index))
				}
				want := []byte{'A', 0xaa, 0xa1, 0xa3, 0xa0, '\r', '\n', '!'}
				if disk, err := os.ReadFile(target); err != nil || !bytes.Equal(disk, want) {
					t.Fatalf("%s output = %x, %v; notice=%q", action, disk, err, state.notice)
				}
				if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.String() != "A\uE000\uE5E5\n!" || !doc.Editor.CanUndo() {
					t.Fatal("menu save changed source selection or history")
				}
				if action == "Save a Copy As..." {
					if !doc.Dirty() || doc.SavedEncoding() != document.UTF8 {
						t.Fatal("Save Copy accepted the source save boundary")
					}
				} else if doc.Dirty() || doc.SavedEncoding() != document.GB18030 {
					t.Fatal("Save did not establish the GB18030 save boundary")
				}
				if action != "Save" {
					if disk, err := os.ReadFile(source); err != nil || !bytes.Equal(disk, original) {
						t.Fatalf("alternate save changed source = %x, %v", disk, err)
					}
				}
				loaded, err := document.OpenWithEncoding(target, document.GB18030)
				if err != nil || loaded.Editor.String() != "A\uE000\uE5E5\n!" || loaded.Dirty() {
					t.Fatalf("saved output did not reopen: %+v, %v", loaded, err)
				}
			})
		}
	}
}
