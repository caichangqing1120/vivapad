package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

var utfConversionMenuFixtures = []struct {
	label  string
	format document.Encoding
	data   string
}{
	{"UTF-8", document.UTF8, "41cea9f09f98800d0a420d0a"},
	{"UTF-8 BOM", document.UTF8BOM, "efbbbf41cea9f09f98800d0a420d0a"},
	{"UTF-16 LE", document.UTF16LE, "fffe4100a9033dd800de0d000a0042000d000a00"},
	{"UTF-16 BE", document.UTF16BE, "feff004103a9d83dde00000d000a0042000d000a"},
}

func utfConversionMenuBytes(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func startUTFConversionDriver(t *testing.T, source []byte, width, height int) (*model, *app.Driver, string) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "unicode.txt")
	if err := os.WriteFile(file, source, 0o600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{file}, filepath.Join(root, "settings", "session.json"))
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		driver.Close()
		if state.recoveryWriter != nil {
			if err := state.recoveryWriter.Wait(); err != nil {
				t.Error(err)
			}
		}
	})
	if err := driver.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	return state, driver, file
}

func clickUTFConversion(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	index := slices.Index(menuCommands[4], label)
	if index < 0 {
		t.Fatalf("missing encoding command %q", label)
	}
	clickID(t, driver, menuScope.At(4))
	clickID(t, driver, menuItemScope.At(index))
}

func TestUTFConversionMenuLiteralSave(t *testing.T) {
	for _, target := range utfConversionMenuFixtures {
		for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
			t.Run(fmt.Sprintf("%s/%dx%d", target.label, size.width, size.height), func(t *testing.T) {
				source := utfConversionMenuFixtures[0]
				if target.format == document.UTF8 {
					source = utfConversionMenuFixtures[1]
				}
				original := utfConversionMenuBytes(t, source.data)
				state, driver, file := startUTFConversionDriver(t, original, size.width, size.height)
				doc := state.space.Current()
				editor, value := doc.Editor, doc.Editor.String()
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret := editor.Selection(), editor.Caret()
				clickUTFConversion(t, driver, target.label)
				if doc.Editor != editor || editor.String() != value || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || editor.CanRedo() || !editor.AtSavePoint() {
					t.Fatalf("encoding menu changed editor state: text=%q selection=%+v caret=%d", editor.String(), editor.Selection(), editor.Caret())
				}
				if doc.Encoding() != target.format || doc.SavedEncoding() != source.format || !doc.Dirty() || doc.LineEnding() != document.CRLF {
					t.Fatalf("encoding command metadata: encoding=%s saved=%v dirty=%t ending=%v", doc.EncodingLabel(), doc.SavedEncoding(), doc.Dirty(), doc.LineEnding())
				}
				if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
					t.Fatalf("menu conversion wrote disk before Save: %x, %v", disk, err)
				}
				clickID(t, driver, menuScope.At(4))
				for _, candidate := range utfConversionMenuFixtures {
					found := false
					index := slices.Index(menuCommands[4], candidate.label)
					for _, hit := range driver.Snapshot().Hits {
						if hit.ID == menuItemScope.At(index) {
							found = true
							if hit.Role != gfx.RoleMenuItem || hit.Checked != (candidate.format == target.format) {
								t.Fatalf("encoding menu state for %s: role=%v checked=%t", candidate.label, hit.Role, hit.Checked)
							}
						}
					}
					if !found {
						t.Fatalf("encoding menu did not expose %s", candidate.label)
					}
				}
				clickID(t, driver, menuScope.At(4))
				clickID(t, driver, saveID)
				if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, utfConversionMenuBytes(t, target.data)) || doc.Dirty() || doc.SavedEncoding() != target.format {
					t.Fatalf("menu Save produced %x, %v, dirty=%t", disk, err, doc.Dirty())
				}
			})
		}
	}
}

func TestUTFConversionMenuPreservesUndoRedo(t *testing.T) {
	for _, target := range utfConversionMenuFixtures {
		t.Run(target.label, func(t *testing.T) {
			source := utfConversionMenuFixtures[0]
			if target.format == document.UTF8 {
				source = utfConversionMenuFixtures[1]
			}
			state, driver, _ := startUTFConversionDriver(t, utfConversionMenuBytes(t, source.data), 640, 400)
			doc := state.space.Current()
			editor, value := doc.Editor, doc.Editor.String()
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("!")
			editor.DeleteBackward()
			if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
				t.Fatal("fixture did not establish Undo and Redo")
			}
			editor.SetSelection(text.Range{Start: 3, End: 1})
			selection, caret := editor.Selection(), editor.Caret()
			clickUTFConversion(t, driver, target.label)
			if editor.String() != value+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || doc.Encoding() != target.format {
				t.Fatal("menu conversion changed pending text, selection or history")
			}
			if !editor.Undo() || editor.String() != value || !editor.AtSavePoint() || !doc.Dirty() || !editor.Redo() || !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
				t.Fatal("menu conversion lost Undo/Redo ordering or pending-format dirty state")
			}
			if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
				t.Fatal("fixture did not retain Undo and Redo before Save")
			}
			selection, caret = editor.Selection(), editor.Caret()
			clickID(t, driver, saveID)
			if doc.Editor != editor || editor.String() != value+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || !editor.AtSavePoint() || doc.Dirty() || doc.SavedEncoding() != target.format {
				t.Fatal("menu Save lost existing history or failed to mark the current boundary")
			}
			if !editor.Undo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() || !editor.Redo() || editor.String() != value+"!" || !editor.AtSavePoint() || doc.Dirty() || !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
				t.Fatal("menu Save changed history ordering or the new saved boundary")
			}
		})
	}
}

func TestUTFConversionSaveDialogCancelAndFailure(t *testing.T) {
	for _, target := range utfConversionMenuFixtures {
		for _, mode := range []string{"cancel", "invalid-target"} {
			t.Run(target.label+"/"+mode, func(t *testing.T) {
				source := utfConversionMenuFixtures[0]
				if target.format == document.UTF8 {
					source = utfConversionMenuFixtures[1]
				}
				original := utfConversionMenuBytes(t, source.data)
				state, driver, file := startUTFConversionDriver(t, original, 640, 400)
				doc := state.space.Current()
				editor, value := doc.Editor, doc.Editor.String()
				editor.MoveTo(editor.RuneCount(), false)
				editor.Insert("!")
				editor.DeleteBackward()
				if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("fixture did not establish Undo and Redo")
				}
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret, savedPath := editor.Selection(), editor.Caret(), doc.Path
				clickUTFConversion(t, driver, target.label)
				state.notice = "before Save As"
				if mode == "cancel" {
					if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
						t.Fatal(err)
					}
				} else if err := driver.QueueSaveFile(filepath.Join(filepath.Dir(file), "missing", "target.txt"), nil); err != nil {
					t.Fatal(err)
				}
				clickID(t, driver, menuScope.At(0))
				clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[0], "Save As...")))
				if request, ok := driver.LastSaveFileRequest(); !ok || request.Title != "Save As" || request.SuggestedName != doc.Name || request.DefaultExtension != "txt" {
					t.Fatalf("Save As picker was not invoked with the document options: %+v, %t", request, ok)
				}
				if state.menuOpen != -1 || doc.Path != savedPath || doc.Encoding() != target.format || doc.SavedEncoding() != source.format || !doc.Dirty() || doc.Editor != editor || editor.String() != value+"!" || editor.AtSavePoint() || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("unsuccessful Save As changed source identity, pending format or editor state")
				}
				if mode == "invalid-target" && (state.notice == "" || state.notice == "before Save As") {
					t.Fatal("failed Save As was not reported")
				}
				if mode == "cancel" && state.notice != "before Save As" {
					t.Fatalf("cancelled Save As changed the notice: %q", state.notice)
				}
				if !editor.Undo() || editor.String() != value || !editor.AtSavePoint() || !doc.Dirty() || !editor.Redo() || !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
					t.Fatal("unsuccessful Save As changed the old saved boundary or history ordering")
				}
				if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
					t.Fatalf("unsuccessful Save As overwrote source: %x, %v", disk, err)
				}
			})
		}
	}
}

func TestUTFConversionMenuNoOpPreservesDirtyHistory(t *testing.T) {
	for _, source := range utfConversionMenuFixtures {
		t.Run(source.label, func(t *testing.T) {
			state, driver, _ := startUTFConversionDriver(t, utfConversionMenuBytes(t, source.data), 640, 400)
			doc := state.space.Current()
			editor, value := doc.Editor, doc.Editor.String()
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("!")
			editor.DeleteBackward()
			editor.SetSelection(text.Range{Start: 3, End: 1})
			selection, caret := editor.Selection(), editor.Caret()
			clickUTFConversion(t, driver, source.label)
			if editor.String() != value || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || editor.CanRedo() || editor.AtSavePoint() || !doc.Dirty() || doc.Encoding() != source.format {
				t.Fatal("no-op menu conversion hid equal-text dirty history or changed editor state")
			}
		})
	}
}
