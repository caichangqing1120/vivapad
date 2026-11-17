package document

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestReplaceAllReportsOriginalByteEditsWithRegexCaptures(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫1\n猫2")
	count, edits, err := doc.ReplaceAllWithEdits(`(猫)(\d)`, "$2-$1\n", SearchOptions{Regex: true})
	if err != nil || count != 2 || len(edits) != 2 ||
		edits[0] != (ReplacementEdit{Start: 0, End: 4, Text: "1-猫\n"}) ||
		edits[1] != (ReplacementEdit{Start: 5, End: 9, Text: "2-猫\n"}) ||
		doc.Editor.String() != "1-猫\n\n2-猫\n" {
		t.Fatalf("regex edits = %+v, count=%d, text=%q, err=%v", edits, count, doc.Editor.String(), err)
	}
}

func TestFindOptionsRespectCaseAndWholeWords(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("cat scatter CAT cat_ cat")
	doc.Editor.MoveTo(0, false)
	options := SearchOptions{MatchCase: true, WholeWord: true}
	if found, err := doc.FindWith("cat", options, false); err != nil || !found || doc.Editor.Selection().Start != 0 {
		t.Fatalf("first match: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
	if found, err := doc.FindWith("cat", options, false); err != nil || !found || doc.Editor.Selection().Start != 21 {
		t.Fatalf("next whole word: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
	doc.Editor.MoveTo(0, false)
	if found, err := doc.FindWith("cat", SearchOptions{WholeWord: true}, true); err != nil || !found || doc.Editor.Selection().Start != 21 {
		t.Fatalf("wrapped previous: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
}

func TestUnicodeWholeWordDoesNotMatchInsideIdentifier(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫 猫咪 猫")
	doc.Editor.MoveTo(0, false)
	options := SearchOptions{WholeWord: true}
	if found, err := doc.FindWith("猫", options, false); err != nil || !found {
		t.Fatalf("first match: found=%v err=%v", found, err)
	}
	if found, err := doc.FindWith("猫", options, false); err != nil || !found || doc.Editor.Selection().Start != 5 {
		t.Fatalf("last match: found=%v err=%v selection=%+v", found, err, doc.Editor.Selection())
	}
}

func TestRegexReplaceAllExpandsCapturesAsOneUndoUnit(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("item-12 item-34")
	count, err := doc.ReplaceAllWith(`(item)-(\d+)`, `${2}:$1`, SearchOptions{Regex: true})
	if err != nil || count != 2 || doc.Editor.String() != "12:item 34:item" {
		t.Fatalf("replace count=%d err=%v text=%q", count, err, doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != "item-12 item-34" {
		t.Fatalf("undo = %q", doc.Editor.String())
	}
}

func TestInvalidRegexLeavesTextAndSelectionUntouched(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("one two")
	before := doc.Editor.Selection()
	if _, err := doc.FindWith("(", SearchOptions{Regex: true}, false); err == nil {
		t.Fatal("invalid search regex succeeded")
	}
	if _, err := doc.ReplaceAllWith("(", "x", SearchOptions{Regex: true}); err == nil {
		t.Fatal("invalid replace regex succeeded")
	}
	if doc.Editor.String() != "one two" || doc.Editor.Selection() != before {
		t.Fatal("invalid regex modified the document")
	}
}

func TestZeroWidthRegexDoesNotTrapFindOrReplace(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("abc")
	options := SearchOptions{Regex: true}
	if found, err := doc.FindWith("^", options, false); err != nil || found {
		t.Fatalf("zero-width find: found=%v err=%v", found, err)
	}
	if count, err := doc.ReplaceAllWith("^", "x", options); err != nil || count != 0 || doc.Editor.String() != "abc" {
		t.Fatalf("zero-width replace: count=%d err=%v text=%q", count, err, doc.Editor.String())
	}
}

func TestMatchesWithReturnsByteSpansWithoutChangingSelection(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert("猫 cat 猫")
	before := doc.Editor.Selection()
	matches, err := doc.MatchesWith("猫", SearchOptions{})
	if err != nil || len(matches) != 2 || matches[0].Start != 0 || matches[0].End != 3 || matches[1].Start != 8 || matches[1].End != 11 {
		t.Fatalf("matches = %+v, err=%v", matches, err)
	}
	if doc.Editor.Selection() != before {
		t.Fatal("highlight query moved the selection")
	}
}

func TestMatchesWithLimitKeepsMatchSemantics(t *testing.T) {
	tests := []struct {
		name, value, query string
		options            SearchOptions
	}{
		{"unicode whole word", "猫 猫咪 猫", "猫", SearchOptions{WholeWord: true}},
		{"unicode case fold", "k K K", "k", SearchOptions{WholeWord: true}},
		{"regex multiline anchors", "a\na\na", `(?m)^a`, SearchOptions{Regex: true}},
		{"regex zero width", "cat dog", `\b|cat`, SearchOptions{Regex: true}},
		{"regex captures", "a1 a2 a3", `a(\d)`, SearchOptions{Regex: true}},
		{"regex whole word", "cat_cat cat cats cat", `cat`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word capture", "cat_dog cat dog DOG", `(cat|dog)`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word anchored", "cat\ncat\ncat", `(?m)^cat`, SearchOptions{Regex: true, WholeWord: true}},
		{"regex whole word boundary", "cat_cat cat", `\bcat`, SearchOptions{Regex: true, WholeWord: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := New("new 1")
			doc.Editor.Insert(test.value)
			all, err := doc.MatchesWith(test.query, test.options)
			if err != nil {
				t.Fatal(err)
			}
			for limit := 0; limit <= len(all)+1; limit++ {
				spans, truncated, err := doc.MatchesWithLimit(test.query, test.options, limit)
				if err != nil || truncated != (len(all) > limit) || !reflect.DeepEqual(spans, all[:min(limit, len(all))]) {
					t.Fatalf("limit %d: spans=%+v truncated=%v err=%v; all=%+v", limit, spans, truncated, err, all)
				}
			}
		})
	}
}

func TestRegexWidthProofKeepsZeroWidthBranchesOnFullSearch(t *testing.T) {
	tests := []struct {
		pattern    string
		consumes   bool
		suffixSafe bool
	}{
		{`(?i)a+?`, true, true},
		{`(?m)^a`, true, false},
		{`(?:a|b)+`, true, true},
		{`\bcat`, true, false},
		{`a|\b`, false, false},
		{`a?`, false, false},
		{`(?m)^`, false, false},
		{`cat$`, true, false},
	}
	for _, test := range tests {
		expression := regexp.MustCompile(test.pattern)
		if got := regexAlwaysConsumes(expression); got != test.consumes {
			t.Errorf("width proof for %q = %v, want %v", test.pattern, got, test.consumes)
		}
		if got := regexCanResumeOnSuffix(expression); got != test.suffixSafe {
			t.Errorf("suffix proof for %q = %v, want %v", test.pattern, got, test.suffixSafe)
		}
	}
}

func TestRegexWholeWordBatchBoundariesKeepAcceptedOrder(t *testing.T) {
	doc := New("new 1")
	doc.Editor.Insert(strings.Repeat("cat_", 255) + " cat " + strings.Repeat("cat_", 256) + " cat " + strings.Repeat("cat_", 257) + " cat")
	options := SearchOptions{Regex: true, WholeWord: true}
	all, err := doc.MatchesWith("cat", options)
	if err != nil || len(all) != 3 {
		t.Fatalf("reference matches=%d err=%v", len(all), err)
	}
	for limit := 0; limit <= len(all); limit++ {
		spans, truncated, err := doc.MatchesWithLimit("cat", options, limit)
		if err != nil || truncated != (limit < len(all)) || !reflect.DeepEqual(spans, all[:limit]) {
			t.Fatalf("limit %d: spans=%+v truncated=%v err=%v, want %+v", limit, spans, truncated, err, all[:limit])
		}
	}
}
