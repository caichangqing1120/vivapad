package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/recovery"
)

func TestMixedEOLMenuSaveUndoPreservesPhysicalBytes(t *testing.T) {
	for _, operation := range []string{"Save", "Save As...", "Save a Copy As..."} {
		for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
			t.Run(fmt.Sprintf("%s/%dx%d", operation, size.width, size.height), func(t *testing.T) {
				original := []byte("a\r\nb\nc\rd")
				state, driver, file := startUTFConversionDriver(t, original, size.width, size.height)
				doc := state.space.Current()
				doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
				doc.Editor.SetSelection(text.Range{Start: 2, End: 0})
				selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
				destination := filepath.Join(filepath.Dir(file), "saved.txt")
				if operation == "Save" {
					destination = file
					clickID(t, driver, saveID)
				} else {
					if err := driver.QueueSaveFile(destination, nil); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], operation)))
					if _, ok := driver.LastSaveFileRequest(); !ok {
						t.Fatal("save command did not invoke its picker")
					}
				}
				if data, err := os.ReadFile(destination); err != nil || string(data) != "a\r\nbc\rd" {
					t.Fatalf("menu Save changed edited bytes: %x, %v", data, err)
				}
				copyOnly := operation == "Save a Copy As..."
				if doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Dirty() != copyOnly || !doc.Editor.CanUndo() {
					t.Fatal("menu Save changed selection or the saved boundary")
				}
				clickID(t, driver, undoID)
				if doc.Editor.String() != "a\nb\nc\nd" || doc.Dirty() == copyOnly || !doc.Editor.CanRedo() {
					t.Fatal("menu Undo did not restore text across the correct saved boundary")
				}
				roundTrip := filepath.Join(filepath.Dir(file), "roundtrip.txt")
				if err := driver.QueueSaveFile(roundTrip, nil); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, menuScope.At(0))
				clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], "Save a Copy As...")))
				if data, err := os.ReadFile(roundTrip); err != nil || !bytes.Equal(data, original) {
					t.Fatalf("menu Save/Undo lost original physical separators: %x, %v", data, err)
				}
				clickID(t, driver, redoID)
				if doc.Editor.String() != "a\nbc\nd" || doc.Dirty() != copyOnly || !doc.Editor.CanUndo() {
					t.Fatal("menu Redo did not recover the edited state and saved boundary")
				}
			})
		}
	}
}

func TestMixedEOLRecoveryUndoKeepsOriginalDiskSeparators(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(fmt.Sprintf("equal-%t", equal), func(t *testing.T) {
			root := t.TempDir()
			file, privatePath := filepath.Join(root, "mixed.txt"), filepath.Join(root, "config", "session.json")
			original := []byte("a\r\nb\nc\rd")
			if err := os.WriteFile(file, original, 0o600); err != nil {
				t.Fatal(err)
			}
			state := newStartupModel([]string{file}, privatePath)
			doc := state.space.Current()
			if equal {
				doc.MarkRecovered()
			} else {
				doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "X")
			}
			stored := state.currentRecovery()
			if len(stored.Tabs) != 1 || stored.Tabs[0].LineEndings != "WLM" {
				t.Fatal("recovery fixture did not retain its separator sequence")
			}
			if err := recovery.Save(state.recoveryPath, stored); err != nil {
				t.Fatal(err)
			}
			restored := newStartupModel(nil, privatePath).space.Current()
			if restored.Editor.String() != stored.Tabs[0].Content || !restored.Editor.Undo() {
				t.Fatal("actual startup recovery lost its Undo")
			}
			copyPath := filepath.Join(root, "undone.txt")
			if err := restored.SaveCopy(copyPath); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(data, original) {
				t.Fatalf("recovery Undo changed original disk separators: %x, %v", data, err)
			}
			if !restored.Editor.Redo() || restored.EOLSequence() != "WLM" {
				t.Fatal("recovery Redo lost its own LF identities")
			}
		})
	}
}
