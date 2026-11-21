package document

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func mixedHistoryBytes(t *testing.T, value string, format Encoding) []byte {
	t.Helper()
	for _, character := range value {
		if character > 0x7f {
			t.Fatal("mixed history fixture must be ASCII")
		}
	}
	switch format {
	case UTF8, Windows1252:
		return []byte(value)
	case UTF8BOM:
		return append([]byte{0xef, 0xbb, 0xbf}, value...)
	case UTF16LE, UTF16BE:
		data := []byte{0xff, 0xfe}
		if format == UTF16BE {
			data = []byte{0xfe, 0xff}
		}
		for _, character := range []byte(value) {
			if format == UTF16LE {
				data = append(data, character, 0)
			} else {
				data = append(data, 0, character)
			}
		}
		return data
	default:
		t.Fatalf("unsupported fixture format %d", format)
		return nil
	}
}

func openMixedHistoryDocument(t *testing.T, value string, format Encoding) *Document {
	t.Helper()
	file := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(file, mixedHistoryBytes(t, value, format), 0o600); err != nil {
		t.Fatal(err)
	}
	var doc *Document
	var err error
	if format == Windows1252 {
		doc, err = OpenWithEncoding(file, format)
	} else {
		doc, err = Open(file)
	}
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func assertMixedHistoryCopy(t *testing.T, doc *Document, want []byte) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "copy.txt")
	dirty, saved := doc.Dirty(), doc.Editor.AtSavePoint()
	if err := doc.SaveCopy(file); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, want) {
		t.Fatalf("Save Copy changed physical separators: got %x, want %x, error=%v", data, want, err)
	}
	if doc.Dirty() != dirty || doc.Editor.AtSavePoint() != saved {
		t.Fatal("Save Copy changed the source saved boundary")
	}
}

func TestMixedEOLSavePreservesUndoSeparators(t *testing.T) {
	const original, normalized = "a\r\nb\nc\rd\r\n", "a\nb\nc\nd\n"
	for _, format := range []Encoding{UTF8, UTF8BOM, UTF16LE, UTF16BE, Windows1252} {
		for _, saveAs := range []bool{false, true} {
			for _, scenario := range []struct {
				index          int
				physical, text string
			}{
				{1, "ab\nc\rd\r\n", "ab\nc\nd\n"},
				{3, "a\r\nbc\rd\r\n", "a\nbc\nd\n"},
				{5, "a\r\nb\ncd\r\n", "a\nb\ncd\n"},
				{7, "a\r\nb\nc\rd", "a\nb\nc\nd"},
			} {
				t.Run(fmt.Sprintf("format-%d/save-as-%t/cut-%d", format, saveAs, scenario.index), func(t *testing.T) {
					doc := openMixedHistoryDocument(t, original, format)
					editor := doc.Editor
					editor.ReplaceRange(text.Range{Start: scenario.index, End: scenario.index + 1}, "")
					editor.SetSelection(text.Range{Start: 2, End: 0})
					selection, caret := editor.Selection(), editor.Caret()
					destination := ""
					if saveAs {
						destination = filepath.Join(t.TempDir(), "saved-as.txt")
					}
					if err := doc.Save(destination); err != nil {
						t.Fatal(err)
					}
					if data, err := os.ReadFile(doc.Path); err != nil || !bytes.Equal(data, mixedHistoryBytes(t, scenario.physical, format)) {
						t.Fatalf("edited Save bytes = %x, %v", data, err)
					}
					if doc.Editor != editor || editor.String() != scenario.text || editor.Selection() != selection || editor.Caret() != caret || !editor.CanUndo() || editor.CanRedo() || !editor.AtSavePoint() || doc.Dirty() {
						t.Fatal("Save changed editor state or failed to mark the saved boundary")
					}
					if !editor.Undo() || editor.String() != normalized || !doc.Dirty() || !editor.CanRedo() {
						t.Fatal("Undo did not restore original text across Save")
					}
					assertMixedHistoryCopy(t, doc, mixedHistoryBytes(t, original, format))
					if !editor.Redo() || editor.String() != scenario.text || doc.Dirty() || !editor.AtSavePoint() {
						t.Fatal("Redo did not return to the edited saved boundary")
					}
					assertMixedHistoryCopy(t, doc, mixedHistoryBytes(t, scenario.physical, format))
					if !editor.Undo() {
						t.Fatal("second Undo failed")
					}
					if err := doc.Save(""); err != nil {
						t.Fatal(err)
					}
					if data, err := os.ReadFile(doc.Path); err != nil || !bytes.Equal(data, mixedHistoryBytes(t, original, format)) || doc.Dirty() || !editor.CanRedo() {
						t.Fatalf("Save after Undo lost original separators or future history: %x, %v", data, err)
					}
					if !editor.Redo() || !doc.Dirty() {
						t.Fatal("Redo after saving original text lost its future edit")
					}
					assertMixedHistoryCopy(t, doc, mixedHistoryBytes(t, scenario.physical, format))
				})
			}
		}
	}
}

func TestMixedEOLSavedUniformFormatSelectionIsNoOp(t *testing.T) {
	for _, noBreaks := range []bool{false, true} {
		t.Run(fmt.Sprintf("no-breaks-%t", noBreaks), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
			doc.Editor.ReplaceRange(text.Range{Start: 4, End: 5}, "")
			if noBreaks {
				doc.Editor.ReplaceRange(text.Range{Start: 1, End: 2}, "")
			}
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			doc.SetLineEnding(CRLF)
			if doc.Dirty() || doc.MixedLineEndings() || !doc.Editor.AtSavePoint() {
				t.Fatal("choosing an already saved uniform format changed the document")
			}
			for doc.Editor.CanUndo() {
				if !doc.Editor.Undo() {
					t.Fatal("Undo failed")
				}
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
		})
	}
}

func TestMixedEOLExplicitConversionStillUniformAfterSaveAndUndo(t *testing.T) {
	for _, scenario := range []struct {
		format LineEnding
		ending string
	}{{LF, "\n"}, {CRLF, "\r\n"}, {CR, "\r"}} {
		t.Run(fmt.Sprintf("ending-%d", scenario.format), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if !doc.Editor.Undo() {
				t.Fatal("Undo failed")
			}
			doc.SetLineEnding(scenario.format)
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			assertMixedHistoryCopy(t, doc, []byte(strings.Join([]string{"a", "b", "c", "d"}, scenario.ending)))
			if doc.MixedLineEndings() || !doc.Editor.CanRedo() || !doc.Editor.Redo() {
				t.Fatal("explicit conversion changed history availability or remained mixed")
			}
			assertMixedHistoryCopy(t, doc, []byte(strings.Join([]string{"a", "bc", "d"}, scenario.ending)))
			if !doc.Editor.Undo() {
				t.Fatal("Undo after explicit conversion failed")
			}
			assertMixedHistoryCopy(t, doc, []byte(strings.Join([]string{"a", "b", "c", "d"}, scenario.ending)))
		})
	}
}

func TestMixedEOLHistorySurvivesSeparatedEditsAndRepeatedSave(t *testing.T) {
	doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
	doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "new\n")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.Undo() {
		t.Fatal("Undo inserted line failed")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nbc\rd"))
	if !doc.Editor.Undo() {
		t.Fatal("Undo deleted separator failed")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
	if !doc.Editor.Redo() || !doc.Editor.Redo() || doc.Dirty() {
		t.Fatal("Redo did not recover the latest saved boundary")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nnew\r\nbc\rd"))
}

func TestMixedEOLFailedSaveKeepsHistoricalSeparators(t *testing.T) {
	for _, externalConflict := range []bool{false, true} {
		t.Run(fmt.Sprintf("external-conflict-%t", externalConflict), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
			destination := filepath.Join(filepath.Dir(doc.Path), "missing", "target.txt")
			if externalConflict {
				destination = ""
				if err := os.WriteFile(doc.Path, []byte("external data"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := doc.Save(destination); err == nil || !doc.Dirty() || doc.Editor.AtSavePoint() {
				t.Fatal("failed Save changed the old saved boundary")
			}
			if !doc.Editor.Undo() || doc.Dirty() || !doc.Editor.AtSavePoint() {
				t.Fatal("Undo after failed Save lost the original saved boundary")
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
		})
	}
}

func TestMixedEOLRecoveredSequenceSurvivesSaveAndUndo(t *testing.T) {
	doc := openMixedHistoryDocument(t, "a\nb\nc\nd", UTF8)
	doc.SetLineEnding(CRLF)
	if err := doc.RestoreEOLSequence("WLM"); err != nil {
		t.Fatal(err)
	}
	doc.MarkRecovered()
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.Undo() {
		t.Fatal("Undo failed")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
}

func TestLargeMixedEOLSavePreservesUndoSeparators(t *testing.T) {
	var original strings.Builder
	for index := range 300 {
		fmt.Fprintf(&original, "line%03d", index)
		if index < 299 {
			original.WriteString([]string{"\r\n", "\n", "\r"}[index%3])
		}
	}
	doc := openMixedHistoryDocument(t, original.String(), UTF8)
	position := strings.Index(doc.Editor.String(), "line100\n") + len("line100")
	doc.Editor.ReplaceRange(text.Range{Start: position, End: position + 1}, "")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if !doc.Editor.Undo() {
		t.Fatal("Undo failed")
	}
	assertMixedHistoryCopy(t, doc, []byte(original.String()))
}

func TestMixedEOLReplacedSourceRefusesPhysicalSave(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprintf("tracked-%t", tracked), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			doc.Editor = text.NewEditor(doc.Editor.String())
			if tracked && !doc.Editor.TrackLFLabels([]byte{'M', 'M', 'M'}, 'M', nil) {
				t.Fatal("replacement tracker setup failed")
			}
			copyPath := filepath.Join(t.TempDir(), "copy.txt")
			if err := doc.SaveCopy(copyPath); err == nil {
				t.Fatal("Save Copy silently reconstructed labels for a replaced source")
			}
			if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
				t.Fatal("refused Save Copy created output")
			}
			if err := doc.Save(""); err == nil {
				t.Fatal("Save accepted a replaced label source")
			}
			if data, err := os.ReadFile(doc.Path); err != nil || string(data) != "a\r\nb\nc\rd" {
				t.Fatal("refused Save changed original bytes")
			}
		})
	}
}

func TestMixedEOLSaveSurvivesProducerGapTrimAndClear(t *testing.T) {
	for _, scenario := range []string{"gap", "trim", "clear"} {
		t.Run(scenario, func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			var suffix string
			switch scenario {
			case "gap", "trim":
				count := 300
				if scenario == "trim" {
					count = 1200
				}
				doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
				for range count {
					doc.Editor.Paste("x")
				}
				suffix = strings.Repeat("x", count)
				if _, err := doc.ChangeHistory(); err == nil {
					t.Fatal("fixture did not expire the bounded producer")
				}
			case "clear":
				doc.Editor.ClearHistory()
			}
			doc.Editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nbc\rd"+suffix))
			if !doc.Editor.Undo() {
				t.Fatal("Undo after producer expiry failed")
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"+suffix))
			if !doc.Editor.Redo() || doc.Dirty() {
				t.Fatal("Redo did not recover the actual saved boundary")
			}
		})
	}
}

func TestMixedEOLLegacyCompositionSaveAndUndo(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-%t", cancel), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
			doc.Editor.SetMarkedText("x\n", text.Range{})
			if _, err := doc.ChangeHistory(); err == nil {
				t.Fatal("legacy composition did not invalidate the producer")
			}
			if cancel {
				if !doc.Editor.CancelComposition() {
					t.Fatal("legacy Cancel failed")
				}
			} else {
				doc.Editor.Unmark()
			}
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if cancel {
				assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
				return
			}
			assertMixedHistoryCopy(t, doc, []byte("ax\r\nb\nc\rd"))
			if !doc.Editor.Undo() {
				t.Fatal("legacy Undo failed")
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
			if !doc.Editor.Redo() || doc.Dirty() {
				t.Fatal("legacy Redo lost its saved boundary")
			}
		})
	}
}

func TestMixedEOLTentativeSavePreservesPrestartDeletion(t *testing.T) {
	for _, phase := range []text.CompositionPhase{text.CompositionCancel, text.CompositionResult, text.CompositionCommit} {
		t.Run(fmt.Sprintf("phase-%d", phase), func(t *testing.T) {
			doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
			if !doc.Editor.EnableTentativeComposition() {
				t.Fatal("tentative setup failed")
			}
			doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
			if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "x\n"}) ||
				!doc.Editor.ApplyComposition(text.CompositionEvent{Phase: phase, Text: "x\n"}) {
				t.Fatal("tentative event refused")
			}
			if err := doc.Save(""); err != nil {
				t.Fatal(err)
			}
			if phase != text.CompositionCancel {
				assertMixedHistoryCopy(t, doc, []byte("ax\r\nb\nc\rd"))
				if !doc.Editor.Undo() {
					t.Fatal("tentative result Undo failed")
				}
			}
			assertMixedHistoryCopy(t, doc, []byte("ab\nc\rd"))
			if !doc.Editor.Undo() {
				t.Fatal("prestart deletion lost its ordinary Undo")
			}
			assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
		})
	}
}

func TestMixedEOLZeroNetTransactionKeepsDistinctSeparatorIdentity(t *testing.T) {
	doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
	if err := doc.Editor.Transact(func(draft *text.Editor) error {
		draft.ReplaceRange(text.Range{Start: 3, End: 4}, "")
		draft.ReplaceRange(text.Range{Start: 3, End: 3}, "\n")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nb\r\nc\rd"))
	if !doc.Editor.Undo() {
		t.Fatal("zero-net Undo failed")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nb\nc\rd"))
	if !doc.Editor.Redo() || doc.Dirty() {
		t.Fatal("zero-net Redo lost its saved boundary")
	}
	assertMixedHistoryCopy(t, doc, []byte("a\r\nb\r\nc\rd"))
}

func TestMixedEOLDraftEncodingValidationHasNoHistoryProducer(t *testing.T) {
	doc := NewDraft("preview", text.NewEditor("a\nb"))
	doc.Editor.SetSavePoint()
	doc.SetLineEnding(CRLF)
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 3}, "preview\n")
	if err := doc.ValidateEncoding(); err != nil {
		t.Fatal(err)
	}
	if doc.LineEnding() != CRLF {
		t.Fatal("draft EOL configuration refused")
	}
	if consumer, err := doc.ChangeHistory(); err == nil || consumer != nil || doc.historyBound {
		t.Fatal("preview draft became a change-history source")
	}
}

func TestMixedEOLCacheFollowsMetadataReplayAndTentativeCancellation(t *testing.T) {
	doc := openMixedHistoryDocument(t, "a\r\nb\nc\rd", UTF8)
	if doc.LineEndingAt(1) != "\n" {
		t.Fatal("initial separator cache is wrong")
	}
	doc.Editor.ReplaceRange(text.Range{End: doc.Editor.RuneCount()}, doc.Editor.String())
	if !doc.Editor.SetLFLabels([]byte("MWL"), 'W') || doc.EOLSequence() != "MWL" || doc.LineEndingAt(1) != "\r\n" {
		t.Fatal("same-text metadata update did not invalidate the cache")
	}
	if !doc.Editor.Undo() || doc.EOLSequence() != "WLM" || doc.LineEndingAt(1) != "\n" {
		t.Fatal("same-text Undo did not invalidate the cache")
	}
	if !doc.Editor.Redo() || doc.EOLSequence() != "MWL" {
		t.Fatal("same-text Redo did not invalidate the cache")
	}
	if !doc.Editor.EnableTentativeComposition() {
		t.Fatal("tentative setup failed")
	}
	doc.Editor.SetSelection(text.Range{Start: 1, End: 2})
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "x\n"}) || doc.EOLSequence() != "WWL" {
		t.Fatal("tentative update cache is wrong")
	}
	if !doc.Editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel}) || doc.EOLSequence() != "WL" {
		t.Fatal("tentative cancellation did not invalidate the cache")
	}
	if allocations := testing.AllocsPerRun(100, func() { doc.LineEndingAt(1) }); allocations != 0 {
		t.Fatalf("warm separator lookup allocates %g objects", allocations)
	}
}
