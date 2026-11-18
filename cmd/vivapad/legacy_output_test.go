package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestSaveAllReturnsFocusAfterEncodingRefusalAndRetry(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(fmt.Sprintf("split=%t", split), func(t *testing.T) {
			root := t.TempDir()
			firstPath, peerPath := filepath.Join(root, "first.txt"), filepath.Join(root, "peer.txt")
			firstBytes, peerBytes := []byte("A\r\n\u4e2d\nB\r"), []byte("peer\r\n")
			for path, value := range map[string][]byte{firstPath: firstBytes, peerPath: peerBytes} {
				if err := os.WriteFile(path, value, 0600); err != nil {
					t.Fatal(err)
				}
			}
			state := newModelWithFiles([]string{firstPath, peerPath})
			first, peer := state.space.Documents[0], state.space.Documents[1]
			focus := editorID
			if split {
				state.toggleSplit()
				state.paneDocs = [2]*document.Document{first, peer}
				state.paneOwner[first], state.paneOwner[peer] = 0, 1
				state.activePane = 1
				focus = rightEditorID
			}
			state.space.Select(1)
			first.Editor.MoveTo(first.Editor.RuneCount(), false)
			first.Editor.Insert("x")
			first.SetEncoding(document.Windows1252)
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, focus)
			peer.Editor.MoveTo(peer.Editor.RuneCount(), false)
			if err := driver.Type("y"); err != nil {
				t.Fatal(err)
			}
			peer.Editor.SetSelection(text.Range{Start: 6, End: 5})
			before, caret := peer.Editor.Selection(), peer.Editor.Caret()
			saveAll := func() {
				clickID(t, driver, menuScope.At(0))
				clickID(t, driver, menuItemScope.At(5))
			}
			checkDisk := func(path string, want []byte) {
				t.Helper()
				if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
					t.Fatalf("%s = %q, %v; want %q", filepath.Base(path), got, err, want)
				}
			}
			saveAll()
			checkDisk(firstPath, firstBytes)
			checkDisk(peerPath, peerBytes)
			if !strings.Contains(state.notice, "Windows-1252") || !first.Dirty() || !peer.Dirty() ||
				state.space.Current() != peer || peer.Editor.Selection() != before || peer.Editor.Caret() != caret {
				t.Fatal("encoding refusal changed the active tab, selection or drafts")
			}
			requireEditorCommandFocus(t, driver, focus)
			if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || peer.Editor.String() != "peer\n" || first.Editor.String() != "A\n\u4e2d\nB\nx" {
				t.Fatalf("Undo after refused Save All reached the wrong target: %q/%q, %v", first.Editor.String(), peer.Editor.String(), err)
			}
			if !peer.Editor.Redo() {
				t.Fatal("refusal lost peer Redo history")
			}
			first.SetEncoding(document.UTF8)
			saveAll()
			checkDisk(firstPath, append(firstBytes, 'x'))
			checkDisk(peerPath, append(peerBytes, 'y'))
			if first.Dirty() || peer.Dirty() || state.space.Current() != peer {
				t.Fatal("successful Save All did not preserve the active tab and mark both saved")
			}
			requireEditorCommandFocus(t, driver, focus)
		})
	}
}

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
