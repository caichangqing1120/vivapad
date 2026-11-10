package document

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestFindIncrementalFirstNextAndWrapBothDirections(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("cat cat cat")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(0, false)
	for index, expected := range []text.Range{{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11}, {Start: 0, End: 3}} {
		found, wrapped := doc.FindIncremental("cat", false, false, index != 0)
		if !found || wrapped != (index == 3) || doc.Editor.Selection() != expected {
			t.Fatalf("incremental step %d = found:%v wrapped:%v selection:%+v", index, found, wrapped, doc.Editor.Selection())
		}
	}
	if found, wrapped := doc.FindIncremental("cat", false, true, true); !found || !wrapped || doc.Editor.Selection() != (text.Range{Start: 8, End: 11}) {
		t.Fatalf("incremental previous wrap = %v %v %+v", found, wrapped, doc.Editor.Selection())
	}
	if doc.Editor.String() != "cat cat cat" || doc.Editor.CanUndo() {
		t.Fatal("incremental navigation edited text or undo history")
	}
}

func TestFindIncrementalSupportsOverlapsUnicodeAndInclusiveRefinement(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("banana")
	doc.Editor.MoveTo(0, false)
	if found, _ := doc.FindIncremental("ana", false, false, false); !found || doc.Editor.Selection() != (text.Range{Start: 1, End: 4}) {
		t.Fatalf("first overlap = %v %+v", found, doc.Editor.Selection())
	}
	if found, _ := doc.FindIncremental("ana", false, false, true); !found || doc.Editor.Selection() != (text.Range{Start: 3, End: 6}) {
		t.Fatalf("next overlap = %v %+v", found, doc.Editor.Selection())
	}
	doc.Editor.SetSelection(text.Range{Start: 1, End: 4})
	if found, _ := doc.FindIncremental("anana", false, false, false); !found || doc.Editor.Selection() != (text.Range{Start: 1, End: 6}) {
		t.Fatalf("inclusive refinement = %v %+v", found, doc.Editor.Selection())
	}
	unicode := New("unicode")
	unicode.Editor.Insert("猫猫猫")
	unicode.Editor.MoveTo(0, false)
	unicode.FindIncremental("猫猫", true, false, false)
	unicode.FindIncremental("猫猫", true, false, true)
	if unicode.Editor.Selection() != (text.Range{Start: 1, End: 3}) {
		t.Fatalf("Unicode overlap = %+v", unicode.Editor.Selection())
	}
}

func TestFindIncrementalCaseAndNoMatchPreserveSelection(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("Cat cat")
	doc.Editor.MoveTo(0, false)
	if found, _ := doc.FindIncremental("cat", false, false, false); !found || doc.Editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("insensitive first = %v %+v", found, doc.Editor.Selection())
	}
	if found, _ := doc.FindIncremental("cat", true, false, false); !found || doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("case-sensitive update = %v %+v", found, doc.Editor.Selection())
	}
	before := doc.Editor.Selection()
	if found, _ := doc.FindIncremental("missing", false, false, false); found || doc.Editor.Selection() != before {
		t.Fatal("missing query changed selection")
	}
	if found, _ := doc.FindIncremental("", false, false, false); found || doc.Editor.Selection() != before {
		t.Fatal("empty query changed selection")
	}
}

func TestIncrementalMatchesCountOverlapsAtRuneBoundaries(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("banana")
	if got := doc.IncrementalMatches("ana", true); !reflect.DeepEqual(got, []text.Span{{Start: 1, End: 4}, {Start: 3, End: 6}}) {
		t.Fatalf("overlapping matches = %+v", got)
	}
	unicode := New("unicode")
	unicode.Editor.Insert("猫猫猫")
	if got := unicode.IncrementalMatches("猫猫", true); !reflect.DeepEqual(got, []text.Span{{Start: 0, End: 6}, {Start: 3, End: 9}}) {
		t.Fatalf("Unicode overlapping byte spans = %+v", got)
	}
}
