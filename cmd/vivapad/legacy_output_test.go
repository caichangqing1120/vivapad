package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestLegacyUnrepresentableMenuSavePreservesSource(t *testing.T) {
	for _, operation := range []string{"Save", "Save As...", "Save a Copy As..."} {
		for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
			t.Run(fmt.Sprintf("%s/%dx%d", operation, size.width, size.height), func(t *testing.T) {
				original := []byte("A\r\n\u4e2d\nB\r")
				state, driver, file := startUTFConversionDriver(t, original, size.width, size.height)
				doc := state.space.Current()
				editor, before, savedPath := doc.Editor, doc.Editor.String(), doc.Path
				editor.MoveTo(editor.RuneCount(), false)
				editor.Insert("!")
				editor.DeleteBackward()
				if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("fixture did not establish Undo and Redo")
				}
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret := editor.Selection(), editor.Caret()
				clickUTFConversion(t, driver, "Windows-1252")
				target := filepath.Join(filepath.Dir(file), "destination.txt")
				if err := os.WriteFile(target, []byte("existing destination"), 0o600); err != nil {
					t.Fatal(err)
				}
				state.notice = ""
				if operation == "Save" {
					clickID(t, driver, saveID)
				} else {
					if err := driver.QueueSaveFile(target, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], operation)))
					if request, ok := driver.LastSaveFileRequest(); !ok || request.SuggestedName != doc.Name {
						t.Fatalf("save command did not invoke its picker: %+v, %t", request, ok)
					}
				}
				if state.notice == "" || !strings.Contains(state.notice, "Windows-1252") {
					t.Errorf("lossy output was not reported: %q", state.notice)
				}
				if doc.Editor != editor || editor.String() != before+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || editor.AtSavePoint() || !doc.Dirty() || doc.Path != savedPath || doc.Encoding() != document.Windows1252 || doc.SavedEncoding() != document.UTF8 {
					t.Error("lossy menu Save changed source, pending encoding, selection or history")
				}
				if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
					t.Errorf("lossy menu Save changed source disk bytes: %x, %v", data, err)
				}
				if data, err := os.ReadFile(target); err != nil || string(data) != "existing destination" {
					t.Errorf("lossy menu Save changed destination bytes: %x, %v", data, err)
				}
				clickUTFConversion(t, driver, "UTF-8")
				clickID(t, driver, saveID)
				if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, append(original, '!')) || doc.Dirty() || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatalf("UTF-8 retry did not preserve the original Unicode text and history: %x, %v", data, err)
				}
			})
		}
	}
}
