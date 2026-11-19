package document

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestUTF8OutputRefusesLeadingBOMCharacter(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "copy.txt")
	const original, copyBytes = "\xef\xbb\xbf\xef\xbb\xbfA\r\nB", "keep copy\r\n"
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(copyBytes), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(source)
	if err != nil || doc.Editor.String() != "\ufeffA\nB" || doc.Encoding() != UTF8BOM {
		t.Fatalf("fixture must decode a leading content character: %+v, %v", doc, err)
	}
	doc.Editor.SetSelection(text.Range{Start: 2, End: 1})
	selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
	doc.SetEncoding(UTF8)
	for _, op := range []struct {
		name string
		run  func() error
	}{{"preflight", doc.ValidateEncoding}, {"Save Copy", func() error { return doc.SaveCopy(destination) }}, {"Save", func() error { return doc.Save("") }}} {
		if err := op.run(); err == nil || !strings.Contains(err.Error(), "U+FEFF") {
			t.Errorf("%s must refuse a leading U+FEFF in plain UTF-8, got %v", op.name, err)
		}
	}
	if disk, err := os.ReadFile(source); err != nil || string(disk) != original {
		t.Errorf("refusal changed the source: %x, %v", disk, err)
	}
	if disk, err := os.ReadFile(destination); err != nil || string(disk) != copyBytes {
		t.Errorf("refusal changed the copy destination: %x, %v", disk, err)
	}
	if doc.Editor.String() != "\ufeffA\nB" || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret ||
		doc.Encoding() != UTF8 || doc.SavedEncoding() != UTF8BOM || !doc.Dirty() || !doc.Editor.AtSavePoint() {
		t.Error("refusal changed editor state, pending encoding or saved boundary")
	}
	doc.SetEncoding(UTF8BOM)
	if err := doc.SaveCopy(destination); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(destination); err != nil || string(disk) != original {
		t.Fatalf("UTF-8 with BOM did not retain both byte sequences: %x, %v", disk, err)
	}
	if copy, err := Open(destination); err != nil || copy.Editor.String() != doc.Editor.String() || copy.Encoding() != UTF8BOM {
		t.Fatalf("UTF-8 BOM copy did not reopen with the leading text character: %+v, %v", copy, err)
	}
	const interior = "A\ufeffB"
	if encoded, err := encodeText(interior, UTF8); err != nil || string(encoded) != interior {
		t.Fatalf("an interior U+FEFF is valid plain UTF-8: %x, %v", encoded, err)
	}
}

var utfConversionFixtures = []struct {
	format         Encoding
	label          string
	lf, crlf       string
	lfEnd, crlfEnd string
	bom            string
}{
	{UTF8, "UTF-8", "41cea9f09f98800a42", "41cea9f09f98800d0a42", "0a", "0d0a", ""},
	{UTF8BOM, "UTF-8 BOM", "efbbbf41cea9f09f98800a42", "efbbbf41cea9f09f98800d0a42", "0a", "0d0a", "efbbbf"},
	{UTF16LE, "UTF-16 LE", "fffe4100a9033dd800de0a004200", "fffe4100a9033dd800de0d000a004200", "0a00", "0d000a00", "fffe"},
	{UTF16BE, "UTF-16 BE", "feff004103a9d83dde00000a0042", "feff004103a9d83dde00000d000a0042", "000a", "000d000a", "feff"},
}

func utfConversionLiteral(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestUTFConversionLiteralBytesAndEditorState(t *testing.T) {
	for _, source := range utfConversionFixtures {
		for _, target := range utfConversionFixtures {
			for _, ending := range []LineEnding{LF, CRLF} {
				endingName := "LF"
				if ending == CRLF {
					endingName = "CRLF"
				}
				for _, final := range []bool{false, true} {
					t.Run(source.label+"/to-"+target.label+"/eol-"+endingName+"/final-"+strconv.FormatBool(final), func(t *testing.T) {
						sourceHex, targetHex := source.lf, target.lf
						if ending == CRLF {
							sourceHex, targetHex = source.crlf, target.crlf
						}
						value := "A\u03a9\U0001f600\nB"
						if final {
							value += "\n"
							if ending == CRLF {
								sourceHex, targetHex = sourceHex+source.crlfEnd, targetHex+target.crlfEnd
							} else {
								sourceHex, targetHex = sourceHex+source.lfEnd, targetHex+target.lfEnd
							}
						}
						original := utfConversionLiteral(t, sourceHex)
						file := filepath.Join(t.TempDir(), "unicode.txt")
						if err := os.WriteFile(file, original, 0o600); err != nil {
							t.Fatal(err)
						}
						doc, err := Open(file)
						if err != nil {
							t.Fatal(err)
						}
						editor := doc.Editor
						editor.MoveTo(3, false)
						editor.MoveTo(1, true)
						selection, caret := editor.Selection(), editor.Caret()
						doc.SetEncoding(target.format)
						if doc.Editor != editor || editor.String() != value || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || editor.CanRedo() || !editor.AtSavePoint() {
							t.Fatalf("conversion changed editor state: text=%q selection=%+v caret=%d undo=%t redo=%t saved=%t", editor.String(), editor.Selection(), editor.Caret(), editor.CanUndo(), editor.CanRedo(), editor.AtSavePoint())
						}
						if doc.Encoding() != target.format || doc.EncodingLabel() != target.label || doc.SavedEncoding() != source.format || doc.LineEnding() != ending || doc.Dirty() != (source.format != target.format) {
							t.Fatalf("conversion metadata: encoding=%s saved=%v ending=%v dirty=%t", doc.EncodingLabel(), doc.SavedEncoding(), doc.LineEnding(), doc.Dirty())
						}
						if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
							t.Fatalf("conversion wrote disk before Save: %x, %v", disk, err)
						}
						if err := doc.Save(""); err != nil {
							t.Fatal(err)
						}
						if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, utfConversionLiteral(t, targetHex)) || doc.Dirty() || doc.SavedEncoding() != target.format {
							t.Fatalf("saved conversion: bytes=%x error=%v dirty=%t", disk, err, doc.Dirty())
						}
						loaded, err := Open(file)
						if err != nil || loaded.Editor.String() != value || loaded.Encoding() != target.format || loaded.LineEnding() != ending || loaded.Dirty() {
							t.Fatalf("converted file did not reopen unchanged: document=%+v, %v", loaded, err)
						}
					})
				}
			}
		}
	}
}

func TestUTFConversionPreservesUndoRedo(t *testing.T) {
	for _, target := range utfConversionFixtures {
		t.Run(target.label, func(t *testing.T) {
			source := utfConversionFixtures[0]
			if target.format == UTF8 {
				source = utfConversionFixtures[1]
			}
			file := filepath.Join(t.TempDir(), "history.txt")
			original := utfConversionLiteral(t, source.lf)
			if err := os.WriteFile(file, original, 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := Open(file)
			if err != nil {
				t.Fatal(err)
			}
			editor, value := doc.Editor, doc.Editor.String()
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("!")
			editor.DeleteBackward()
			if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() || editor.String() != value+"!" {
				t.Fatal("fixture did not establish both Undo and Redo")
			}
			editor.SetSelection(text.Range{Start: 1, End: 3})
			selection, caret := editor.Selection(), editor.Caret()
			doc.SetEncoding(target.format)
			if doc.Editor != editor || editor.String() != value+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || !doc.Dirty() {
				t.Fatal("conversion changed the current edit or history")
			}
			if !editor.Undo() || editor.String() != value || !editor.AtSavePoint() || !doc.Dirty() || editor.CanUndo() {
				t.Fatal("format conversion added an Undo action or lost the saved boundary")
			}
			if !editor.Redo() || editor.String() != value+"!" || !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() || editor.CanRedo() {
				t.Fatal("conversion lost an existing Redo action or equal-text dirty state")
			}
			if !editor.Undo() || !editor.Undo() || editor.String() != value || !editor.AtSavePoint() {
				t.Fatal("conversion changed history ordering")
			}
			doc.SetEncoding(source.format)
			if doc.Dirty() || !editor.CanRedo() || doc.SavedEncoding() != source.format {
				t.Fatal("reverting only the encoding did not restore clean metadata")
			}
			if !editor.Redo() || !editor.Redo() || editor.String() != value || !doc.Dirty() {
				t.Fatal("fixture did not restore an equal-text dirty history position")
			}
			doc.SetEncoding(source.format)
			if !doc.Dirty() || editor.AtSavePoint() || !editor.CanUndo() {
				t.Fatal("no-op conversion hid an unsaved equal-text history position")
			}
			if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
				t.Fatalf("unsaved history conversion wrote disk: %x, %v", disk, err)
			}
		})
	}
}

func TestUTFConversionFailedSaveKeepsPendingFormat(t *testing.T) {
	for _, target := range utfConversionFixtures {
		for _, conflict := range []bool{false, true} {
			t.Run(target.label+"/external-conflict-"+strconv.FormatBool(conflict), func(t *testing.T) {
				source := utfConversionFixtures[0]
				if target.format == UTF8 {
					source = utfConversionFixtures[1]
				}
				root := t.TempDir()
				file := filepath.Join(root, "source.txt")
				original := utfConversionLiteral(t, source.crlf)
				if err := os.WriteFile(file, original, 0o600); err != nil {
					t.Fatal(err)
				}
				doc, err := Open(file)
				if err != nil {
					t.Fatal(err)
				}
				editor, value := doc.Editor, doc.Editor.String()
				editor.MoveTo(editor.RuneCount(), false)
				editor.Insert("!")
				editor.DeleteBackward()
				if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("fixture did not establish Undo and Redo")
				}
				editor.SetSelection(text.Range{Start: 3, End: 1})
				selection, caret := editor.Selection(), editor.Caret()
				savedPath := doc.Path
				doc.SetEncoding(target.format)
				destination := filepath.Join(root, "missing", "target.txt")
				if conflict {
					original, destination = []byte("external change"), ""
					if err := os.WriteFile(file, original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := doc.Save(destination); err == nil {
					t.Fatal("invalid target or changed source accepted")
				}
				if doc.Path != savedPath || doc.Encoding() != target.format || doc.SavedEncoding() != source.format || !doc.Dirty() || doc.Editor != editor || editor.String() != value+"!" || editor.AtSavePoint() || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() {
					t.Fatal("failed Save changed pending format, source identity or editor state")
				}
				if !editor.Undo() || editor.String() != value || !editor.AtSavePoint() || !doc.Dirty() || !editor.Redo() || !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
					t.Fatal("failed Save changed the old saved boundary or history ordering")
				}
				if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, original) {
					t.Fatalf("failed Save overwrote source: %x, %v", disk, err)
				}
			})
		}
	}
}

func TestUTFConversionSavePreservesUndoRedo(t *testing.T) {
	for _, target := range utfConversionFixtures {
		t.Run(target.label, func(t *testing.T) {
			source := utfConversionFixtures[0]
			if target.format == UTF8 {
				source = utfConversionFixtures[1]
			}
			file := filepath.Join(t.TempDir(), "history.txt")
			if err := os.WriteFile(file, utfConversionLiteral(t, source.lf), 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := Open(file)
			if err != nil {
				t.Fatal(err)
			}
			editor, value := doc.Editor, doc.Editor.String()
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("!")
			editor.DeleteBackward()
			if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
				t.Fatal("fixture did not establish Undo and Redo")
			}
			editor.SetSelection(text.Range{Start: 3, End: 1})
			selection, caret := editor.Selection(), editor.Caret()
			doc.SetEncoding(target.format)
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if doc.Editor != editor || editor.String() != value+"!" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || !editor.CanRedo() || !editor.AtSavePoint() || doc.Dirty() || doc.SavedEncoding() != target.format {
				t.Fatal("successful conversion Save lost history or failed to mark the current boundary")
			}
			bang := "21"
			if target.format == UTF16LE {
				bang = "2100"
			} else if target.format == UTF16BE {
				bang = "0021"
			}
			if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, utfConversionLiteral(t, target.lf+bang)) {
				t.Fatalf("conversion Save with history produced %x, %v", disk, err)
			}
			if !editor.Undo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
				t.Fatal("Undo did not cross the new saved boundary")
			}
			if !editor.Redo() || editor.String() != value+"!" || !editor.AtSavePoint() || doc.Dirty() {
				t.Fatal("Redo did not return to the new saved boundary")
			}
			if !editor.Redo() || editor.String() != value || editor.AtSavePoint() || !doc.Dirty() {
				t.Fatal("conversion Save discarded the existing future edit")
			}
		})
	}
}

func TestUTFConversionEmptyFileBOM(t *testing.T) {
	for _, target := range utfConversionFixtures {
		t.Run(target.label, func(t *testing.T) {
			doc := New("empty.txt")
			doc.SetEncoding(target.format)
			file := filepath.Join(t.TempDir(), "empty.txt")
			if err := doc.Save(file); err != nil {
				t.Fatal(err)
			}
			if disk, err := os.ReadFile(file); err != nil || !bytes.Equal(disk, utfConversionLiteral(t, target.bom)) || doc.Dirty() || doc.Editor.String() != "" || doc.Editor.CanUndo() {
				t.Fatalf("empty conversion: bytes=%x error=%v dirty=%t", disk, err, doc.Dirty())
			}
		})
	}
}
