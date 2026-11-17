package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestGB18030PUAMenuSave(t *testing.T) {
	for _, operation := range []string{"Save", "Save As...", "Save a Copy As..."} {
		for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
			t.Run(fmt.Sprintf("%s/%dx%d", operation, size.width, size.height), func(t *testing.T) {
				original := []byte("A\r\n\ue000\ue001\ue4c5\ue5e5\ue7c7\nB\r")
				want := []byte{'A', '\r', '\n', 0xaa, 0xa1, 0xaa, 0xa2, 0xfe, 0xfe, 0xa3, 0xa0, 0xa8, 0xbc, '\n', 'B', '\r', '!'}
				state, driver, file := startUTFConversionDriver(t, original, size.width, size.height)
				doc := state.space.Current()
				savedPath := doc.Path
				editor, before, endings := doc.Editor, doc.Editor.String(), doc.EOLSequence()
				editor.MoveTo(editor.RuneCount(), false)
				editor.Insert("!")
				editor.DeleteBackward()
				if !editor.Undo() {
					t.Fatal("missing history fixture")
				}
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret := editor.Selection(), editor.Caret()
				clickUTFConversion(t, driver, "GB18030")
				target := filepath.Join(filepath.Dir(savedPath), "destination.txt")
				if operation == "Save" {
					target = savedPath
					clickID(t, driver, saveID)
				} else {
					if err := driver.QueueSaveFile(target, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], operation)))
					if _, ok := driver.LastSaveFileRequest(); !ok {
						t.Fatal("save menu did not invoke picker")
					}
				}
				if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, want) {
					t.Fatalf("save=%x, %v; notice=%q", data, err, state.notice)
				}
				if doc.Editor != editor || editor.String() != before+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || doc.EOLSequence() != endings {
					t.Fatal("menu save changed source, selection, history or mixed EOL")
				}
				if operation == "Save a Copy As..." {
					if !doc.Dirty() || doc.Path != savedPath || doc.SavedEncoding() != document.UTF8 {
						t.Fatalf("copy metadata: dirty=%t path=%q want=%q saved=%v", doc.Dirty(), doc.Path, savedPath, doc.SavedEncoding())
					}
				} else if doc.Dirty() || doc.Path != target || doc.SavedEncoding() != document.GB18030 {
					t.Fatalf("save metadata: dirty=%t path=%q want=%q saved=%v", doc.Dirty(), doc.Path, target, doc.SavedEncoding())
				}
				if operation != "Save" {
					if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
						t.Fatalf("alternate save changed original=%x, %v", data, err)
					}
				}
				loaded, err := document.OpenWithEncoding(target, document.GB18030)
				if err != nil || loaded.Editor.String() != before+"!" || loaded.Dirty() || loaded.EOLSequence() != endings {
					t.Fatalf("saved output did not reopen: %+v, %v", loaded, err)
				}
			})
		}
	}
}
