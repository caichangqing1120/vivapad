package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestIntegerAwareSortHandlesSignedAndLargeRunsInSelection(t *testing.T) {
	doc := New("numbers.txt")
	before := "item10\nitem2\nitem-3\nitem123456789012345678901234567890\noutside\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: len([]rune("item10\nitem2\nitem-3\nitem123456789012345678901234567890\n"))})
	changed, err := doc.SortLinesMode(SortIntegerAware, true)
	if err != nil || !changed || doc.Editor.String() != "item-3\nitem2\nitem10\nitem123456789012345678901234567890\noutside\n" {
		t.Fatalf("integer-aware ascending = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("integer-aware sort should undo as one edit")
	}
	changed, err = doc.SortLinesMode(SortIntegerAware, false)
	if err != nil || !changed || doc.Editor.String() != "item123456789012345678901234567890\nitem10\nitem2\nitem-3\noutside\n" {
		t.Fatalf("integer-aware descending = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
}

func TestLengthSortCountsUTF16CodeUnitsAndKeepsEqualKeysStable(t *testing.T) {
	doc := New("length.txt")
	doc.Editor.Insert("😀\n猫\nab\n")
	doc.Editor.ClearHistory()
	changed, err := doc.SortLinesMode(SortLength, true)
	if err != nil || !changed || doc.Editor.String() != "\n猫\n😀\nab" {
		t.Fatalf("UTF-16 length ascending = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
	changed, err = doc.SortLinesMode(SortLength, false)
	if err != nil || !changed || doc.Editor.String() != "😀\nab\n猫\n" {
		t.Fatalf("UTF-16 length descending = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
}

func TestDecimalSortSeparatorsEmptyPlacementAndStableTies(t *testing.T) {
	for _, test := range []struct {
		mode                          SortMode
		before, ascending, descending string
	}{
		{SortDecimalComma, "2,5 trailing\n\n1,5\n1,50\n", "\n\n1,5\n1,50\n2,5 trailing", "2,5 trailing\n1,5\n1,50\n\n"},
		{SortDecimalDot, "2.5 trailing\n\n1.5\n1.50\n", "\n\n1.5\n1.50\n2.5 trailing", "2.5 trailing\n1.5\n1.50\n\n"},
	} {
		doc := New("decimal.txt")
		doc.Editor.Insert(test.before)
		doc.Editor.ClearHistory()
		changed, err := doc.SortLinesMode(test.mode, true)
		if err != nil || !changed || doc.Editor.String() != test.ascending {
			t.Fatalf("mode %d ascending = %q changed=%v err=%v", test.mode, doc.Editor.String(), changed, err)
		}
		if !doc.Editor.Undo() || doc.Editor.String() != test.before || doc.Editor.CanUndo() {
			t.Fatalf("mode %d ascending did not undo", test.mode)
		}
		changed, err = doc.SortLinesMode(test.mode, false)
		if err != nil || !changed || doc.Editor.String() != test.descending {
			t.Fatalf("mode %d descending = %q changed=%v err=%v", test.mode, doc.Editor.String(), changed, err)
		}
	}
}

func TestDecimalSortInvalidNumericPrefixIsAtomicAndReportsLine(t *testing.T) {
	doc := New("decimal.txt")
	doc.Editor.Insert("5.2\n- not a number\n1.0\n")
	doc.Editor.ClearHistory()
	changed, err := doc.SortLinesMode(SortDecimalDot, true)
	if changed || err == nil || !strings.Contains(err.Error(), "line 2") || doc.Editor.String() != "5.2\n- not a number\n1.0\n" || doc.Editor.CanUndo() {
		t.Fatalf("invalid numeric line = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
	if changed, err := doc.SortLinesMode(SortMode(99), true); changed || err == nil || doc.Editor.CanUndo() {
		t.Fatalf("invalid sort mode changed document: changed=%v err=%v", changed, err)
	}
}

func TestDecimalSortPreservesCRLFOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "numbers.txt")
	if err := os.WriteFile(path, []byte("2,5\r\n1,2\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := doc.SortLinesMode(SortDecimalComma, true); err != nil || !changed {
		t.Fatalf("CRLF sort changed=%v err=%v", changed, err)
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "\r\n1,2\r\n2,5" {
		t.Fatalf("saved sorted CRLF = %q err=%v", data, err)
	}
}

func TestIntegerAwareSortOrdersLeadingZerosAndNegativeRuns(t *testing.T) {
	doc := New("natural.txt")
	doc.Editor.Insert("id2\nid02\nid-2\nid-10\n")
	doc.Editor.ClearHistory()
	if changed, err := doc.SortLinesMode(SortIntegerAware, true); err != nil || !changed || doc.Editor.String() != "\nid-10\nid-2\nid02\nid2" {
		t.Fatalf("leading zero natural order = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
}

func TestDecimalSortReportsAbsoluteSelectedLineAndTreatsNonNumericAsEmpty(t *testing.T) {
	doc := New("selected.txt")
	doc.Editor.Insert("header\n2.0\n-\n1.0\ntail")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: len([]rune("header\n")), End: len([]rune("header\n2.0\n-\n1.0"))})
	if changed, err := doc.SortLinesMode(SortDecimalDot, true); changed || err == nil || !strings.Contains(err.Error(), "line 3") || doc.Editor.CanUndo() {
		t.Fatalf("selected invalid number changed=%v err=%v", changed, err)
	}
	doc.Editor.ReplaceRange(text.Range{Start: len([]rune("header\n2.0\n")), End: len([]rune("header\n2.0\n-"))}, "not numeric")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: len([]rune("header\n")), End: len([]rune("header\n2.0\nnot numeric\n1.0"))})
	if changed, err := doc.SortLinesMode(SortDecimalDot, true); err != nil || !changed || doc.Editor.String() != "header\nnot numeric\n1.0\n2.0\ntail" {
		t.Fatalf("prepared-empty line placement = %q changed=%v err=%v", doc.Editor.String(), changed, err)
	}
}

func TestDecimalSortRejectsOverflowWithoutEditing(t *testing.T) {
	doc := New("overflow.txt")
	doc.Editor.Insert(strings.Repeat("9", 400) + "\n1.0")
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	if changed, err := doc.SortLinesMode(SortDecimalDot, true); changed || err == nil || !strings.Contains(err.Error(), "line 1") || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("overflow changed=%v err=%v", changed, err)
	}
}
